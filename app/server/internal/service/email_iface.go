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
}
