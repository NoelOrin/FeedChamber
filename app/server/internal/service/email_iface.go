package service

import "context"

// Purpose identifies why a verification code was issued.
type Purpose string

const (
	PurposeResetPassword Purpose = "reset_password"
	PurposeVerifyEmail   Purpose = "verify_email"
)

// EmailService sends and verifies email verification codes.
type EmailService interface {
	// SendCode generates a numeric code and emails it to the given address.
	// Returns RATE_LIMITED when the cooldown/daily-limit is exceeded,
	// or FORBIDDEN when email is disabled in settings.
	SendCode(ctx context.Context, purpose Purpose, email string) error

	// VerifyCode checks the code and consumes it (single-use).
	// Returns INVALID_PARAMS on mismatch or expiry.
	VerifyCode(ctx context.Context, purpose Purpose, email, code string) error

	// SendNotification sends a notification email. Returns nil silently when
	// email notifications are disabled.
	SendNotification(ctx context.Context, to, subject, htmlBody, textBody string) error

	// SendTest delivers one test message using the *currently saved* mailer
	// configuration and reports what actually happened. Config and transport
	// failures come back inside the report (err == nil) so the admin UI can
	// show the reason instead of a bare 500; only a bad argument returns an
	// AppError. The report never contains credentials.
	SendTest(ctx context.Context, to string) (*MailTestReport, error)
}

// MailTestReport 是「发送测试邮件」的诊断结果。字段全部可安全回给前端：
// 密码与 API Key 不属于这个结构，因此不存在「顺手序列化出去」这条路。
type MailTestReport struct {
	Ok           bool   `json:"ok"`
	Provider     string `json:"provider"`
	From         string `json:"from,omitempty"`
	Host         string `json:"host,omitempty"`
	Port         int    `json:"port,omitempty"`
	TLSMode      string `json:"tls_mode,omitempty"` // implicit | starttls，非 SMTP 为空
	EmailEnabled bool   `json:"email_enabled"`
	ElapsedMs    int64  `json:"elapsed_ms"`
	Error        string `json:"error,omitempty"`
	Notice       string `json:"notice,omitempty"`
}
