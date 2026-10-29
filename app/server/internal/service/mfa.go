package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/pkg/kvstore"
	"orinforum/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

type mfaService struct {
	repo       repository.UserMFARepository
	settingSvc SettingService
	kv         kvstore.Store
}

func NewMFAService(repo repository.UserMFARepository, settingSvc SettingService, kv kvstore.Store) MFAService {
	return &mfaService{repo: repo, settingSvc: settingSvc, kv: kv}
}

func (s *mfaService) totpEnabled(ctx context.Context) bool {
	return pkg.GetSettingOrDefault(ctx, s.settingSvc, model.SettingMFATOTPEnabled, "MFA_TOTP_ENABLED", "false") == "true"
}

func (s *mfaService) SetupTOTP(ctx context.Context, userID, username string) (*MFASetupResult, error) {
	if !s.totpEnabled(ctx) {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "未启用 TOTP 双因素认证")
	}
	secret, err := GenerateTOTPSecret()
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	m := &model.UserMFA{
		UserID:     userID,
		TOTPSecret: secret,
		Enabled:    false,
		UpdatedAt:  time.Now(),
	}
	// preserve backup codes if any
	if existing, err := s.repo.Get(ctx, userID); err == nil && existing != nil {
		m.BackupCodes = existing.BackupCodes
		if existing.Enabled {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "已启用 MFA，请先关闭再重新绑定")
		}
	}
	if err := s.repo.Upsert(ctx, m); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	issuer := pkg.GetSettingOrDefault(ctx, s.settingSvc, "site_name", "SITE_NAME", "OrinForum")
	return &MFASetupResult{
		Secret:     secret,
		OTPAuthURL: BuildOTPAuthURL(issuer, username, secret),
	}, nil
}

func (s *mfaService) EnableTOTP(ctx context.Context, userID, code string) ([]string, error) {
	if !s.totpEnabled(ctx) {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "未启用 TOTP 双因素认证")
	}
	m, err := s.repo.Get(ctx, userID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "请先完成 TOTP 初始化")
	}
	if m.Enabled {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "MFA 已启用")
	}
	if !VerifyTOTP(m.TOTPSecret, strings.TrimSpace(code), time.Now()) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "验证码无效")
	}
	plain, hashed, err := generateBackupCodes(10)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	raw, _ := json.Marshal(hashed)
	m.Enabled = true
	m.BackupCodes = string(raw)
	m.UpdatedAt = time.Now()
	if err := s.repo.Upsert(ctx, m); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return plain, nil
}

func (s *mfaService) DisableTOTP(ctx context.Context, userID, code string) error {
	m, err := s.repo.Get(ctx, userID)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "未配置 MFA")
	}
	if !m.Enabled {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "MFA 未启用")
	}
	if !s.validateCode(m, code) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "验证码无效")
	}
	if err := s.repo.Delete(ctx, userID); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *mfaService) RegenerateBackupCodes(ctx context.Context, userID, code string) ([]string, error) {
	m, err := s.repo.Get(ctx, userID)
	if err != nil || !m.Enabled {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "MFA 未启用")
	}
	if !s.validateCode(m, code) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "验证码无效")
	}
	plain, hashed, err := generateBackupCodes(10)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	raw, _ := json.Marshal(hashed)
	m.BackupCodes = string(raw)
	m.UpdatedAt = time.Now()
	if err := s.repo.Upsert(ctx, m); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return plain, nil
}

func (s *mfaService) IsEnabled(ctx context.Context, userID string) (bool, error) {
	m, err := s.repo.Get(ctx, userID)
	if err != nil {
		if err == repository.ErrNotFound {
			return false, nil
		}
		return false, err
	}
	return m.Enabled, nil
}

func (s *mfaService) BeginLoginChallenge(ctx context.Context, userID string) (string, error) {
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	token := hex.EncodeToString(tokenBytes)
	if s.kv != nil {
		if err := s.kv.Set(ctx, "mfa:login:"+token, userID, 5*time.Minute); err != nil {
			return "", pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	return token, nil
}

func (s *mfaService) VerifyLoginChallenge(ctx context.Context, mfaToken, code string) (string, error) {
	if s.kv == nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, "KV unavailable")
	}
	userID, err := s.kv.Get(ctx, "mfa:login:"+mfaToken)
	if err != nil || userID == "" {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "mfa_token 无效或已过期")
	}
	m, err := s.repo.Get(ctx, userID)
	if err != nil || !m.Enabled {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "MFA 未启用")
	}
	if !s.validateCode(m, code) {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "验证码无效")
	}
	_ = s.kv.Delete(ctx, "mfa:login:"+mfaToken)
	// if backup code used, persist remaining hashes
	if m.BackupCodes != "" {
		_ = s.repo.Upsert(ctx, m)
	}
	return userID, nil
}

func (s *mfaService) validateCode(m *model.UserMFA, code string) bool {
	code = strings.TrimSpace(code)
	if VerifyTOTP(m.TOTPSecret, code, time.Now()) {
		return true
	}
	// backup codes
	if m.BackupCodes == "" {
		return false
	}
	var hashes []string
	if err := json.Unmarshal([]byte(m.BackupCodes), &hashes); err != nil {
		return false
	}
	for i, h := range hashes {
		if bcrypt.CompareHashAndPassword([]byte(h), []byte(code)) == nil {
			// consume
			hashes = append(hashes[:i], hashes[i+1:]...)
			raw, _ := json.Marshal(hashes)
			m.BackupCodes = string(raw)
			m.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

func generateBackupCodes(n int) (plain []string, hashed []string, err error) {
	for i := 0; i < n; i++ {
		b := make([]byte, 5)
		if _, err = rand.Read(b); err != nil {
			return nil, nil, err
		}
		code := strings.ToUpper(hex.EncodeToString(b)) // 10 hex chars
		plain = append(plain, code)
		h, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, nil, err
		}
		hashed = append(hashed, string(h))
	}
	return plain, hashed, nil
}
