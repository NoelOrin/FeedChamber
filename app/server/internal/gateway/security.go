package gateway

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/kvstore"

	"github.com/kataras/iris/v12"
	"github.com/sirupsen/logrus"
)

// SecurityManager provides risk-control checks: brute-force detection, unusual
// location alerts, security event recording, and login history tracking.
type SecurityManager struct {
	deps Deps
}

// NewSecurityManager creates a new SecurityManager.
func NewSecurityManager(deps Deps) *SecurityManager {
	return &SecurityManager{deps: deps}
}

func (s *SecurityManager) getKV() kvstore.Store { return s.deps.KV }

// ── Brute-force helpers ─────────────────────────────────────────────────────

// isIPBlocked checks whether an IP is temporarily blocked.
func (s *SecurityManager) isIPBlocked(ip string) bool {
	kv := s.getKV()
	if kv == nil {
		return false
	}
	exists, err := kv.Exists(context.Background(), "blocked_ip:"+ip)
	return err == nil && exists
}

// isUserBlocked checks whether a username is temporarily locked.
func (s *SecurityManager) isUserBlocked(username string) bool {
	kv := s.getKV()
	if kv == nil {
		return false
	}
	exists, err := kv.Exists(context.Background(), "blocked_user:"+username)
	return err == nil && exists
}

// getBruteForceMaxAttempts reads the setting or returns default 5.
func (s *SecurityManager) getBruteForceMaxAttempts(ctx context.Context) int {
	if s.deps.SettingService == nil {
		return 5
	}
	val, err := s.deps.SettingService.Get(ctx, model.SettingSecurityBruteForceMaxAttempts)
	if err != nil {
		return 5
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return 5
	}
	return n
}

// getBruteForceWindow reads the setting or returns default 300s.
func (s *SecurityManager) getBruteForceWindow(ctx context.Context) time.Duration {
	if s.deps.SettingService == nil {
		return 5 * time.Minute
	}
	val, err := s.deps.SettingService.Get(ctx, model.SettingSecurityBruteForceWindow)
	if err != nil {
		return 5 * time.Minute
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(n) * time.Second
}

// RecordFailedLogin increments the brute-force counter for IP and username.
// When the threshold is exceeded the IP and username are temporarily blocked
// and a security event is recorded.
func (s *SecurityManager) RecordFailedLogin(ctx context.Context, ip, username string) {
	kv := s.getKV()
	if kv == nil {
		return
	}

	// Increment per-IP counter
	ipKey := fmt.Sprintf("brute_force:ip:%s", ip)
	window := s.getBruteForceWindow(ctx)
	_, _ = kv.SetNX(ctx, ipKey, "0", window)
	ipCount, _ := kv.Incr(ctx, ipKey)

	// Increment per-username counter
	userKey := fmt.Sprintf("brute_force:user:%s", username)
	_, _ = kv.SetNX(ctx, userKey, "0", window)
	userCount, _ := kv.Incr(ctx, userKey)

	maxAttempts := s.getBruteForceMaxAttempts(ctx)
	blockDuration := 30 * time.Minute

	if int(ipCount) >= maxAttempts {
		if err := kv.Set(ctx, "blocked_ip:"+ip, "1", blockDuration); err != nil {
			logrus.Errorf("security: failed to block IP %s: %v", ip, err)
		}
		s.RecordEvent(ctx, "", ip, "brute_force", "high",
			fmt.Sprintf("IP %s exceeded %d failed login attempts", ip, maxAttempts), "blocked")
	}

	if int(userCount) >= maxAttempts {
		if err := kv.Set(ctx, "blocked_user:"+username, "1", blockDuration); err != nil {
			logrus.Errorf("security: failed to block user %s: %v", username, err)
		}
		s.RecordEvent(ctx, "", ip, "brute_force", "high",
			fmt.Sprintf("User %s exceeded %d failed login attempts from IP %s", username, maxAttempts, ip), "blocked")
	}
}

// ClearFailedLogin removes the brute-force counters for a successful login.
func (s *SecurityManager) ClearFailedLogin(ctx context.Context, ip, username string) {
	kv := s.getKV()
	if kv == nil {
		return
	}
	_ = kv.Delete(ctx, fmt.Sprintf("brute_force:ip:%s", ip))
	_ = kv.Delete(ctx, fmt.Sprintf("brute_force:user:%s", username))
}

// ── Unusual location detection ──────────────────────────────────────────────

// isUnusualLocation checks whether this is the first time a user logs in from
// the given IP (based on login history).
func (s *SecurityManager) isUnusualLocation(ctx context.Context, userUUID, ip string) bool {
	if s.deps.SettingService != nil {
		val, err := s.deps.SettingService.Get(ctx, model.SettingSecurityUnusualLocationEnabled)
		if err != nil || val != "true" {
			return false
		}
	}
	db := s.deps.DB
	if db == nil || userUUID == "" {
		return false
	}
	var count int64
	if err := db.WithContext(ctx).Model(&model.LoginHistory{}).
		Where("user_uuid = ? AND ip = ?", userUUID, ip).
		Count(&count).Error; err != nil {
		logrus.Warnf("security: failed to query login history for unusual location check: %v", err)
		// On DB error, conservatively return false (no alert) rather than
		// treating as "first time from this IP" which would generate false alerts.
		return false
	}
	return count == 0
}

// ── Record helpers ──────────────────────────────────────────────────────────

// RecordEvent persists a security event to the database.
func (s *SecurityManager) RecordEvent(ctx context.Context, userUUID, ip, eventType, severity, detail, action string) {
	db := s.deps.DB
	if db == nil {
		return
	}
	if err := db.WithContext(ctx).Create(&model.SecurityEvent{
		UserUUID:  userUUID,
		IP:        ip,
		EventType: eventType,
		Severity:  severity,
		Detail:    detail,
		Action:    action,
		Handled:   false,
		CreatedAt: time.Now().Unix(),
	}).Error; err != nil {
		logrus.Errorf("security: failed to record security event (%s %s): %v", eventType, action, err)
	}
}

// RecordLoginHistory persists a login attempt record.
func (s *SecurityManager) RecordLoginHistory(ctx context.Context, userUUID, ip, location, userAgent string, success bool) {
	db := s.deps.DB
	if db == nil {
		return
	}
	if err := db.WithContext(ctx).Create(&model.LoginHistory{
		UserUUID:  userUUID,
		IP:        ip,
		Location:  location,
		UserAgent: userAgent,
		Success:   success,
		CreatedAt: time.Now(),
	}).Error; err != nil {
		logrus.Errorf("security: failed to record login history for user %s: %v", userUUID, err)
	}
}

// ── Middleware ──────────────────────────────────────────────────────────────

// SecurityCheck is the middleware that runs as part of the Gateway pipeline.
//  1. Checks IP against temporary blocklist (brute-force prevention).
//  2. Checks username against temporary lockout.
//  3. Detects unusual-location logins for authenticated users.
func (s *SecurityManager) SecurityCheck(ctx iris.Context) {
	ip := ctx.RemoteAddr()
	username := ctx.Values().GetString(pkg.CtxUsername)
	reqCtx := ctx.Request().Context()

	// 1. IP block check (brute force)
	if s.isIPBlocked(ip) {
		pkg.Fail(ctx, pkg.FORBIDDEN, "该 IP 已被临时封禁")
		return
	}

	// 2. Username lockout check
	if username != "" && s.isUserBlocked(username) {
		pkg.Fail(ctx, pkg.FORBIDDEN, "账号已被临时锁定，请稍后再试")
		return
	}

	// 3. Unusual location check (authenticated users only)
	userUUID := ctx.Values().GetString(pkg.CtxUserID)
	if userUUID != "" && s.isUnusualLocation(reqCtx, userUUID, ip) {
		s.RecordEvent(reqCtx, userUUID, ip, "unusual_location", "medium",
			fmt.Sprintf("User %s logged in from new IP %s", userUUID, ip), "alert_only")
	}

	ctx.Next()
}

// ── API handlers ────────────────────────────────────────────────────────────

// 安全事件列表
// @Summary      安全事件列表
// @Description  列出与当前用户相关的安全事件。
// @Tags         安全
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /security/events [get]
// ListSecurityEvents returns security events (admin only).
// GET /api/v1/security/events
func (s *SecurityManager) ListSecurityEvents(ctx iris.Context) {
	role := ctx.Values().GetString(pkg.CtxRole)
	if role != "admin" && role != "super_admin" {
		pkg.Fail(ctx, pkg.FORBIDDEN)
		return
	}

	db := s.deps.DB
	if db == nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "数据库不可用")
		return
	}

	var events []model.SecurityEvent
	if err := db.WithContext(ctx.Request().Context()).
		Order("created_at desc").
		Limit(100).
		Find(&events).Error; err != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "安全事件读取失败")
		return
	}

	pkg.Success(ctx, events)
}

// 登录历史
// @Summary      登录历史
// @Description  列出当前用户的登录记录。
// @Tags         安全
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /security/login-history [get]
// ListLoginHistory returns the current user's login history.
// GET /api/v1/security/login-history
func (s *SecurityManager) ListLoginHistory(ctx iris.Context) {
	userUUID := ctx.Values().GetString(pkg.CtxUserID)
	if userUUID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}

	db := s.deps.DB
	if db == nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "数据库不可用")
		return
	}

	var histories []model.LoginHistory
	if err := db.WithContext(ctx.Request().Context()).
		Where("user_uuid = ?", userUUID).
		Order("created_at desc").
		Limit(50).
		Find(&histories).Error; err != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "登录历史读取失败")
		return
	}

	pkg.Success(ctx, histories)
}

// 上报可疑活动
// @Summary      上报可疑活动
// @Description  用户主动上报可疑登录或异常行为。
// @Tags         安全
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{event_type=string,detail=string} true "上报内容"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /security/report [post]
// ReportSuspicious records a user-reported suspicious activity.
// POST /api/v1/security/report
func (s *SecurityManager) ReportSuspicious(ctx iris.Context) {
	userUUID := ctx.Values().GetString(pkg.CtxUserID)
	if userUUID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}

	var body struct {
		EventType string `json:"event_type"`
		Detail    string `json:"detail"`
		IP        string `json:"ip"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	if body.EventType == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "缺少 event_type")
		return
	}

	ip := body.IP
	if ip == "" {
		ip = ctx.RemoteAddr()
	}

	s.RecordEvent(ctx.Request().Context(), userUUID, ip, body.EventType, "low", body.Detail, "alert_only")
	pkg.Success(ctx, map[string]any{"message": "Report recorded"})
}
