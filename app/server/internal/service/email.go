package service

import (
	"context"
	"fmt"
	"time"

	"feedchamber/internal/mailer"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/kvstore"
)

type emailService struct {
	settingSvc SettingService
	kv         kvstore.Store
}

func NewEmailService(settingSvc SettingService, kv kvstore.Store) EmailService {
	return &emailService{settingSvc: settingSvc, kv: kv}
}

func (s *emailService) SendCode(ctx context.Context, purpose Purpose, email string) error {
	bg := context.Background()

	enabled := pkg.GetSettingOrDefault(ctx, s.settingSvc, model.SettingEmailEnabled, "EMAIL_ENABLED", "false")
	if enabled != "true" {
		return pkg.NewAppError(pkg.FORBIDDEN, "邮件功能未启用，请联系管理员")
	}

	// ── cooldown ────────────────────────────────────────────────────
	cooldownSec := pkg.GetSettingInt(ctx, s.settingSvc, model.SettingEmailResendCooldown, "EMAIL_RESEND_COOLDOWN", 60)
	cooldownKey := fmt.Sprintf("emailcooldown:%s:%s", purpose, email)
	ok, err := s.kv.SetNX(bg, cooldownKey, "1", time.Duration(cooldownSec)*time.Second)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if !ok {
		return pkg.NewAppError(pkg.RATE_LIMITED, fmt.Sprintf("请 %d 秒后再重新获取验证码", cooldownSec))
	}

	// ── daily limit ─────────────────────────────────────────────────
	dailyLimit := pkg.GetSettingInt(ctx, s.settingSvc, model.SettingEmailDailyLimit, "EMAIL_DAILY_LIMIT", 10)
	dateStr := time.Now().Format("2006-01-02")
	dailyKey := fmt.Sprintf("emaildaily:%s:%s:%s", purpose, email, dateStr)
	created, err := s.kv.SetNX(bg, dailyKey, "1", 25*time.Hour)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	var dailyCount int64 = 1
	if !created {
		dailyCount, err = s.kv.Incr(bg, dailyKey)
		if err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	if dailyCount > int64(dailyLimit) {
		return pkg.NewAppError(pkg.RATE_LIMITED, "今日发送次数已达上限，请明天再试")
	}

	// ── generate code ────────────────────────────────────────────────
	codeLen := pkg.GetSettingInt(ctx, s.settingSvc, model.SettingEmailCodeLength, "EMAIL_CODE_LENGTH", 6)
	code := pkg.GenerateNumericCode(codeLen)

	ttlSec := pkg.GetSettingInt(ctx, s.settingSvc, model.SettingEmailCodeTTL, "EMAIL_CODE_TTL", 600)
	codeKey := codeKVKey(purpose, email)
	if err := s.kv.Set(bg, codeKey, code, time.Duration(ttlSec)*time.Second); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	// reset failure counter so a fresh code always starts clean
	_ = s.kv.Delete(bg, failKVKey(purpose, email))

	// ── build mailer and send ────────────────────────────────────────
	mailerCfg := pkg.BuildMailerConfig(ctx, s.settingSvc)
	sender, err := mailer.New(mailerCfg)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "邮件服务配置错误: "+err.Error())
	}

	msg := buildCodeMessage(mailerCfg.From, email, purpose, code, ttlSec)
	if err := sender.Send(ctx, msg); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "邮件发送失败: "+err.Error())
	}

	return nil
}

func (s *emailService) VerifyCode(ctx context.Context, purpose Purpose, email, code string) error {
	bg := context.Background()
	codeKey := codeKVKey(purpose, email)
	failKey := failKVKey(purpose, email)

	stored, err := s.kv.Get(bg, codeKey)
	if err != nil {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "验证码已过期或不存在，请重新获取")
	}

	if stored != code {
		failCount, _ := s.kv.Incr(bg, failKey)
		if failCount >= 5 {
			_ = s.kv.Delete(bg, codeKey)
			_ = s.kv.Delete(bg, failKey)
			return pkg.NewAppError(pkg.INVALID_PARAMS, "验证码错误次数过多，已作废，请重新获取")
		}
		return pkg.NewAppError(pkg.INVALID_PARAMS, fmt.Sprintf("验证码错误，还可尝试 %d 次", 5-failCount))
	}

	_ = s.kv.Delete(bg, codeKey)
	_ = s.kv.Delete(bg, failKey)
	return nil
}

func (s *emailService) SendNotification(ctx context.Context, to, subject, htmlBody, textBody string) error {
	notifyEnabled := pkg.GetSettingOrDefault(ctx, s.settingSvc, model.SettingEmailNotifyEnabled, "", "false")
	if notifyEnabled != "true" {
		return nil
	}

	mailerCfg := pkg.BuildMailerConfig(ctx, s.settingSvc)
	sender, err := mailer.New(mailerCfg)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "邮件服务配置错误: "+err.Error())
	}

	msg := mailer.Message{
		To:       []string{to},
		Subject:  subject,
		HTMLBody: htmlBody,
		TextBody: textBody,
	}
	if err := sender.Send(ctx, msg); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "邮件发送失败: "+err.Error())
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func codeKVKey(purpose Purpose, email string) string {
	return fmt.Sprintf("emailcode:%s:%s", purpose, email)
}

func failKVKey(purpose Purpose, email string) string {
	return fmt.Sprintf("emailfail:%s:%s", purpose, email)
}

func buildCodeMessage(from, to string, purpose Purpose, code string, ttlSec int) mailer.Message {
	ttlMin := ttlSec / 60
	var subject, action string
	switch purpose {
	case PurposeResetPassword:
		subject = "重置密码验证码"
		action = "重置密码"
	default:
		subject = "邮箱验证码"
		action = "验证邮箱"
	}

	text := fmt.Sprintf(
		"您的%s验证码为：%s\n有效期 %d 分钟，请勿泄露给他人。",
		action, code, ttlMin,
	)
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;max-width:480px;margin:0 auto;padding:24px">
  <h2 style="color:#333">%s</h2>
  <p>您的验证码为：</p>
  <div style="font-size:32px;font-weight:bold;letter-spacing:8px;color:#4f46e5;padding:16px 0">%s</div>
  <p style="color:#666;font-size:14px">有效期 %d 分钟，请勿泄露给他人。如非本人操作，请忽略此邮件。</p>
</body>
</html>`, subject, code, ttlMin)

	return mailer.Message{
		To:       []string{to},
		Subject:  subject,
		HTMLBody: html,
		TextBody: text,
	}
}
