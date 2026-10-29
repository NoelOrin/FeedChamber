package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"orinforum/internal/db"
	"orinforum/internal/metrics"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/pkg/eventbus"
	"orinforum/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

var (
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

// validateDisplayName enforces display name rules: non-empty, at most 32
// runes, and no whitespace anywhere. Chinese characters and emoji are allowed.
func validateDisplayName(name string) *pkg.AppError {
	if name == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "显示名称不能为空")
	}
	if utf8.RuneCountInString(name) > 32 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "显示名称不能超过 32 个字符")
	}
	for _, r := range name {
		if unicode.IsSpace(r) {
			return pkg.NewAppError(pkg.INVALID_PARAMS, "显示名称不能包含空格")
		}
	}
	return nil
}

// displayNameOf returns the user's display name, falling back to username
// when display name is empty (e.g. legacy rows not yet backfilled).
func displayNameOf(u *model.User) string {
	if u != nil && u.DisplayName != "" {
		return u.DisplayName
	}
	if u != nil {
		return u.Username
	}
	return ""
}

type authService struct {
	userRepo   repository.UserRepository
	credRepo   repository.CredentialRepository
	metaRepo   repository.MetaRepository
	settingSvc SettingService
	banSvc     BannedService
	inviteSvc  InviteService
	userSvc    UserService
	txMgr      db.TransactionManager
	sessions   SessionStore
	secMgr     SecManager
	emailSvc   EmailService
	mfaSvc     MFAService
	bus        *eventbus.EventBus
	groupSync  systemGroupSyncer
}

// AuthServiceDeps 是 NewAuthService 的依赖清单。
//
// 13 个位置参数时，调用点与构造函数的顺序必须靠人眼对齐；改成具名结构体后
// 漏填或错序都会编译报错。
type AuthServiceDeps struct {
	Users    repository.UserRepository
	Creds    repository.CredentialRepository
	Meta     repository.MetaRepository
	Settings SettingService

	Ban    BannedService
	Invite InviteService
	User   UserService
	MFA    MFAService
	Email  EmailService

	Tx       db.TransactionManager
	Sessions SessionStore
	Security SecManager
	Bus      *eventbus.EventBus
}

func NewAuthService(d AuthServiceDeps) AuthService {
	return &authService{
		userRepo:   d.Users,
		credRepo:   d.Creds,
		metaRepo:   d.Meta,
		settingSvc: d.Settings,
		banSvc:     d.Ban,
		inviteSvc:  d.Invite,
		userSvc:    d.User,
		mfaSvc:     d.MFA,
		emailSvc:   d.Email,
		txMgr:      d.Tx,
		sessions:   d.Sessions,
		secMgr:     d.Security,
		bus:        d.Bus,
	}
}

// SetGroupSync injects system-group membership sync (best effort).
func (s *authService) SetGroupSync(syncer systemGroupSyncer) {
	s.groupSync = syncer
}

// bcryptCost returns the bcrypt cost from settings, defaulting to DefaultCost (10).
func (s *authService) bcryptCost(ctx context.Context) int {
	n := pkg.GetSettingInt(ctx, s.settingSvc, "bcrypt_cost", "BCRYPT_COST", bcrypt.DefaultCost)
	// Cost < bcrypt.MinCost (4) is invalid; cost > bcrypt.DefaultCost (10) is
	// allowed but will slow down auth significantly. Clamp to [min, max].
	if n < bcrypt.DefaultCost {
		return bcrypt.DefaultCost
	}
	if n > 31 {
		return 31
	}
	return n
}

// passwordMinLen returns the minimum password length from settings, defaulting to 8.
func (s *authService) passwordMinLen(ctx context.Context) int {
	n := pkg.GetSettingInt(ctx, s.settingSvc, "password_min_length", "PASSWORD_MIN_LENGTH", 8)
	if n >= 1 {
		return n
	}
	return 8
}

func (s *authService) Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error) {
	// 注册开关检查
	enabled := pkg.GetSettingOrDefault(ctx, s.settingSvc, "register_enabled", "REGISTER_ENABLED", "true")
	if enabled == "false" {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "注册功能已关闭")
	}

	// register_mode: "all" | "email" | "username" | "invite" | "approve"
	// - all: 开放注册（默认）
	// - email: 必须填写邮箱
	// - username: 必须填写用户名（兼容旧配置）
	// - invite: 必须提供有效 invite_code
	// - approve: 注册成功后 role=pending_user，需管理员审批
	mode := pkg.GetSettingOrDefault(ctx, s.settingSvc, "register_mode", "REGISTER_MODE", "all")
	switch mode {
	case "email":
		if req.Email == "" {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "当前仅支持邮箱注册，请填写邮箱")
		}
		if req.Username == "" {
			req.Username = strings.Split(req.Email, "@")[0]
		}
	case "username":
		if req.Username == "" {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "当前仅支持用户名注册，请填写用户名")
		}
	case "invite":
		if req.Username == "" && req.Email == "" {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "用户名或邮箱至少填写一项")
		}
		if strings.TrimSpace(req.InviteCode) == "" {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "当前为邀请注册，请填写邀请码")
		}
	case "approve":
		if req.Username == "" && req.Email == "" {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "用户名或邮箱至少填写一项")
		}
	default: // "all"
		if req.Username == "" && req.Email == "" {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "用户名或邮箱至少填写一项")
		}
	}

	// Input validation
	if req.Username != "" {
		if !usernameRegex.MatchString(req.Username) {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "用户名须为 3-32 位字母、数字或下划线")
		}
	}
	if req.Email != "" {
		if len(req.Email) > 254 || !emailRegex.MatchString(req.Email) {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "邮箱格式不正确")
		}
	}
	if len(req.Password) < s.passwordMinLen(ctx) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "密码长度不能少于 8 位")
	}
	if len(req.Password) > 72 {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "密码长度不能超过 72 位")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.bcryptCost(ctx))
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	// invite 模式：先校验邀请码（事务外读 + 事务内占用，避免无效码进入写路径）
	if mode == "invite" {
		if s.inviteSvc == nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "邀请服务未配置")
		}
	}

	user := model.NewUser(req.Username, req.Email)
	if mode == "approve" {
		user.Role = "pending_user"
	}

	if err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		// Uniqueness checks inside transaction — prevents concurrent registration race
		existing, err := s.userRepo.FindByUsername(txCtx, req.Username)
		if err == nil && existing != nil {
			return pkg.NewAppError(pkg.USERNAME_EXISTS)
		}
		if req.Email != "" {
			existing, err = s.userRepo.FindByEmail(txCtx, req.Email)
			if err == nil && existing != nil {
				return pkg.NewAppError(pkg.EMAIL_EXISTS)
			}
		}
		if mode == "invite" {
			if err := s.inviteSvc.ConsumeValid(txCtx, req.InviteCode); err != nil {
				return err
			}
		}
		if err := s.userRepo.Create(txCtx, user); err != nil {
			return err
		}
		cred := &model.Credential{
			UserID:       user.ID,
			PasswordHash: string(hash),
		}
		if err := s.credRepo.Upsert(txCtx, cred); err != nil {
			return err
		}
		return nil
	}); err != nil {
		// DB unique constraint backstop — convert to friendly error
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "duplicate") {
			// Try to determine which field conflicted
			if strings.Contains(err.Error(), req.Username) || strings.Contains(err.Error(), "username") {
				return nil, pkg.NewAppError(pkg.USERNAME_EXISTS)
			}
			if strings.Contains(err.Error(), req.Email) || strings.Contains(err.Error(), "email") {
				return nil, pkg.NewAppError(pkg.EMAIL_EXISTS)
			}
			return nil, pkg.NewAppError(pkg.USERNAME_EXISTS, "用户名或邮箱已存在")
		}
		var appErr *pkg.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	// best-effort: 同步系统组
	if s.groupSync != nil {
		_ = s.groupSync.SyncUserSystemGroup(ctx, user.ID, user.Role)
	}
	// 凭证 = 服务端会话（不再是 JWT）；注册即登录，签发 opaque session id。
	sid, err := s.issueSession(ctx, user, req.IP, req.UserAgent)
	if err != nil {
		return nil, err
	}

	if s.bus != nil {
		s.bus.Publish(eventbus.Event{Type: eventbus.EventUserCreated, Ctx: ctx, Payload: map[string]any{
			"user_id": user.ID, "username": user.Username, "email": user.Email,
		}})
		s.bus.Publish(eventbus.Event{Type: eventbus.EventUserRegistered, Ctx: ctx, Payload: map[string]any{
			"user_id": user.ID, "username": user.Username, "email": user.Email,
		}})
	}

	return &AuthResponse{Token: sid, User: user.ToResponse()}, nil
}

func (s *authService) ApproveRegistration(ctx context.Context, userID string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "用户不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if user.Role != "pending_user" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "该用户无需审批或已审批")
	}
	// 优先走 UserService.UpdateRole，自动吊销旧 token
	if s.userSvc != nil {
		return s.userSvc.UpdateRole(ctx, userID, "user")
	}
	user.Role = "user"
	if err := s.userRepo.Update(ctx, user); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if s.sessions != nil {
		_ = s.sessions.RevokeAllForUser(ctx, user.ID)
	}
	return nil
}

func (s *authService) Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error) {
	login := strings.TrimSpace(req.Login)

	user, err := s.userRepo.FindByIdentifier(ctx, login)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			metrics.LoginFailuresTotal.WithLabelValues(metrics.LoginReasonUnknownUser).Inc()
			if s.secMgr != nil {
				s.secMgr.RecordFailedLogin(ctx, req.IP, login)
			}
			return nil, pkg.NewAppError(pkg.USER_NOT_FOUND)
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	cred, err := s.credRepo.FindByUserID(ctx, user.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			metrics.LoginFailuresTotal.WithLabelValues(metrics.LoginReasonNoCredential).Inc()
			if s.secMgr != nil {
				s.secMgr.RecordFailedLogin(ctx, req.IP, login)
			}
			return nil, pkg.NewAppError(pkg.INVALID_PASSWORD)
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if err := bcrypt.CompareHashAndPassword([]byte(cred.PasswordHash), []byte(req.Password)); err != nil {
		metrics.LoginFailuresTotal.WithLabelValues(metrics.LoginReasonBadPassword).Inc()
		if s.secMgr != nil {
			s.secMgr.RecordFailedLogin(ctx, req.IP, login)
		}
		return nil, pkg.NewAppError(pkg.INVALID_PASSWORD)
	}

	// 封号检查（账号层面）：role = "ban" 直接拒绝登录，不颁发 token
	if model.HasBanRole(user.Role) {
		return nil, pkg.NewAppError(pkg.USER_BANNED)
	}

	// 禁言检查（操作层面）：用户可登录，但后续请求会被 BanCheck 中间件拦截
	if s.banSvc != nil && s.banSvc.IsBanned(ctx, user.ID) {
		return nil, pkg.NewAppError(pkg.USER_BANNED)
	}

	// MFA challenge
	if s.mfaSvc != nil {
		if enabled, err := s.mfaSvc.IsEnabled(ctx, user.ID); err == nil && enabled {
			mfaToken, err := s.mfaSvc.BeginLoginChallenge(ctx, user.ID)
			if err != nil {
				return nil, err
			}
			if s.secMgr != nil {
				s.secMgr.ClearFailedLogin(ctx, req.IP, login)
			}
			return &AuthResponse{MFARequired: true, MFAToken: mfaToken}, nil
		}
	}

	sid, err := s.issueSession(ctx, user, req.IP, req.UserAgent)
	if err != nil {
		return nil, err
	}

	// 登录成功，清除暴力破解计数
	if s.secMgr != nil {
		s.secMgr.ClearFailedLogin(ctx, req.IP, login)
	}

	// R-05: a present must-change-password flag forces the client to the
	// change-password flow before the account can be used normally.
	_, mustChange, _ := s.metaRepo.Get(ctx, model.MustChangePasswordKey(user.ID))

	return &AuthResponse{Token: sid, User: user.ToResponse(), MustChangePassword: mustChange}, nil
}

// issueSession mints the opaque session credential for a freshly verified
// login/registration. The session row is the credential's authority: revoke
// the row and the presented id stops authenticating immediately.
func (s *authService) issueSession(ctx context.Context, user *model.User, ip, userAgent string) (string, error) {
	if s.sessions == nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, "session store unavailable")
	}
	sess, err := s.sessions.Issue(ctx, user, ip, userAgent)
	if err != nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return sess.SessionID, nil
}

func (s *authService) ChangePassword(ctx context.Context, username, oldPassword, newPassword string) error {
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.USER_NOT_FOUND)
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if len(newPassword) < s.passwordMinLen(ctx) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "密码长度不能少于 8 位")
	}
	if len(newPassword) > 72 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "密码长度不能超过 72 位")
	}

	cred, err := s.credRepo.FindByUserID(ctx, user.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.INVALID_PASSWORD, "用户尚未设置密码")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if err := bcrypt.CompareHashAndPassword([]byte(cred.PasswordHash), []byte(oldPassword)); err != nil {
		return pkg.NewAppError(pkg.INVALID_PASSWORD)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.bcryptCost(ctx))
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		cred.PasswordHash = string(hash)
		if err := s.credRepo.Upsert(txCtx, cred); err != nil {
			return err
		}
		// R-05: password changed — clear the force-change flag if present.
		_ = s.metaRepo.Delete(txCtx, model.MustChangePasswordKey(user.ID))
		return nil
	}); err != nil {
		return err
	}

	// 密码已变更，吊销该用户所有 token
	if s.sessions != nil {
		_ = s.sessions.RevokeAllForUser(ctx, user.ID)
	}
	return nil
}

func (s *authService) FirstChangePassword(ctx context.Context, username, newPassword string, name *string) error {
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return pkg.NewAppError(pkg.USER_NOT_FOUND)
	}
	if user.Role != "admin" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "仅管理员支持首次修改密码")
	}

	if len(newPassword) < s.passwordMinLen(ctx) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "密码长度不能少于 8 位")
	}
	if len(newPassword) > 72 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "密码长度不能超过 72 位")
	}

	// Username uniqueness check before entering the transaction (read-only).
	if name != nil && *name != "" && *name != user.Username {
		existing, _ := s.userRepo.FindByUsername(ctx, *name)
		if existing != nil {
			return pkg.NewAppError(pkg.USERNAME_EXISTS)
		}
		user.Username = *name
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.bcryptCost(ctx))
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	// Upsert credential and clear force-change flag atomically.
	cred := &model.Credential{
		UserID:       user.ID,
		PasswordHash: string(hash),
	}
	if err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		if name != nil && *name != "" {
			if err := s.userRepo.Update(txCtx, user); err != nil {
				return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
		}
		if err := s.credRepo.Upsert(txCtx, cred); err != nil {
			return err
		}
		// R-05: password changed — clear the force-change flag if present.
		_ = s.metaRepo.Delete(txCtx, model.MustChangePasswordKey(user.ID))
		return nil
	}); err != nil {
		return err
	}

	// 密码已变更，吊销该用户所有 token
	if s.sessions != nil {
		_ = s.sessions.RevokeAllForUser(ctx, user.ID)
	}
	return nil
}

func (s *authService) ResetPassword(ctx context.Context, username, newPassword string) error {
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return pkg.NewAppError(pkg.USER_NOT_FOUND)
	}

	if len(newPassword) < s.passwordMinLen(ctx) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "密码长度不能少于 8 位")
	}
	if len(newPassword) > 72 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "密码长度不能超过 72 位")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.bcryptCost(ctx))
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	cred := &model.Credential{
		UserID:       user.ID,
		PasswordHash: string(hash),
	}
	if err := s.credRepo.Upsert(ctx, cred); err != nil {
		return err
	}

	// 密码已重置，吊销该用户所有 token
	if s.sessions != nil {
		_ = s.sessions.RevokeAllForUser(ctx, user.ID)
	}
	return nil
}

func (s *authService) ResetPasswordByEmail(ctx context.Context, email, newPassword string) error {
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return pkg.NewAppError(pkg.USER_NOT_FOUND)
	}

	if len(newPassword) < s.passwordMinLen(ctx) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "密码长度不能少于 8 位")
	}
	if len(newPassword) > 72 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "密码长度不能超过 72 位")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.bcryptCost(ctx))
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	cred := &model.Credential{
		UserID:       user.ID,
		PasswordHash: string(hash),
	}
	if err := s.credRepo.Upsert(ctx, cred); err != nil {
		return err
	}

	// 密码已重置，吊销该用户所有 token
	if s.sessions != nil {
		_ = s.sessions.RevokeAllForUser(ctx, user.ID)
	}
	return nil
}

func (s *authService) SendVerifyEmail(ctx context.Context, userID string) error {
	if s.emailSvc == nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "邮件服务不可用")
	}
	u, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "用户不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if u.EmailVerified {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "邮箱已验证")
	}
	if u.Email == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "用户未绑定邮箱")
	}
	return s.emailSvc.SendCode(ctx, PurposeVerifyEmail, u.Email)
}

func (s *authService) VerifyEmail(ctx context.Context, userID, code string) error {
	if s.emailSvc == nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "邮件服务不可用")
	}
	u, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "用户不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if u.EmailVerified {
		return nil
	}
	if err := s.emailSvc.VerifyCode(ctx, PurposeVerifyEmail, u.Email, code); err != nil {
		return err
	}
	u.EmailVerified = true
	u.UpdatedAt = time.Now()
	return s.userRepo.Update(ctx, u)
}
