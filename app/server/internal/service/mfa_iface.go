package service

import "context"

type MFASetupResult struct {
	Secret     string `json:"secret"`
	OTPAuthURL string `json:"otpauth_url"`
}

type MFALoginChallenge struct {
	MFARequired bool   `json:"mfa_required"`
	MFAToken    string `json:"mfa_token,omitempty"`
}

type MFAService interface {
	SetupTOTP(ctx context.Context, userID, username string) (*MFASetupResult, error)
	EnableTOTP(ctx context.Context, userID, code string) ([]string, error) // returns plaintext backup codes once
	DisableTOTP(ctx context.Context, userID, code string) error
	RegenerateBackupCodes(ctx context.Context, userID, code string) ([]string, error)
	IsEnabled(ctx context.Context, userID string) (bool, error)
	// BeginLoginChallenge stores a short-lived mfa token for user after password ok.
	BeginLoginChallenge(ctx context.Context, userID string) (mfaToken string, err error)
	// VerifyLoginChallenge validates mfa_token + code and returns userID.
	VerifyLoginChallenge(ctx context.Context, mfaToken, code string) (userID string, err error)
}
