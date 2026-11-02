package model

import "testing"

func TestDefaultSettings_IncludesOAuthOverrideKeys(t *testing.T) {
	providers := []string{"github", "google", "qq", "wechat", "weibo"}
	fields := []string{"auth_url", "token_url", "user_api_url", "scopes"}

	for _, p := range providers {
		for _, f := range fields {
			key := "oauth_" + p + "_" + f
			val, ok := DefaultSettings[key]
			if !ok {
				t.Errorf("DefaultSettings missing override key: %s", key)
			} else if val != "" {
				t.Errorf("DefaultSettings[%s] should be empty string, got %q", key, val)
			}
		}
	}
}

// TestDefaultSettings_SeedsEveryDeclaredConstant is the regression guard for the
// `rate_limit_enabled` bug: the gateway read that key on every request, but it
// was absent from DefaultSettings, so every read missed the settings table and
// logged GORM's "record not found". Any Setting* constant that the code reads by
// name must also be seeded, otherwise it silently falls back to a default and
// costs a DB round-trip per request.
//
// The list is explicit rather than reflected: constants whose values are built
// at runtime (the oauth_<provider>_* family) are covered by the test above.
func TestDefaultSettings_SeedsEveryDeclaredConstant(t *testing.T) {
	keys := []string{
		SettingSSOOIDCEnabled, SettingSSOOIDCIssuer, SettingSSOOIDCClientID, SettingSSOOIDCClientSecret,
		SettingAPIKeyEnabled, SettingAPIKeyDefaultRateLimit,
		SettingRegisterMode, SettingRegisterEnabled,
		SettingMFATOTPEnabled,
		SettingRateLimitEnabled,
		SettingSecurityBruteForceMaxAttempts,
		SettingSecurityBruteForceWindow, SettingSecurityUnusualLocationEnabled,
		SettingBackupRetentionDays,
		SettingSEODescription, SettingSEOKeywords, SettingSEOTitle, SettingOGImage,
		SettingGoogleSiteVerification, SettingBaiduSiteVerification,
		SettingUserProfileIndexable, SettingSitemapPageSize,
	}
	for _, key := range keys {
		if _, ok := DefaultSettings[key]; !ok {
			t.Errorf("DefaultSettings is missing %q, which the code reads by name", key)
		}
	}
}

func TestIsSecretSettingKey(t *testing.T) {
	secrets := []string{
		"email_password",
		"email_api_key",
		"oauth_github_client_secret",
		"sso_oidc_client_secret",
		"s3_access_key_id",
		"s3_access_key_secret",
		"EMAIL_PASSWORD",
	}
	for _, key := range secrets {
		if !IsSecretSettingKey(key) {
			t.Errorf("IsSecretSettingKey(%q) = false, want true", key)
		}
	}

	nonSecrets := []string{
		"site_name",
		"email_enabled",
		"email_smtp_host",
		"oauth_github_client_id",
		"storage_provider",
	}
	for _, key := range nonSecrets {
		if IsSecretSettingKey(key) {
			t.Errorf("IsSecretSettingKey(%q) = true, want false", key)
		}
	}
}

func TestRedactSettingAuditDetail(t *testing.T) {
	if got := RedactSettingAuditDetail("email_password", "super-secret"); got != "[REDACTED]" {
		t.Errorf("secret detail = %q, want [REDACTED]", got)
	}
	if got := RedactSettingAuditDetail("site_name", "Feed Chamber"); got != "Feed Chamber" {
		t.Errorf("non-secret detail = %q, want Feed Chamber", got)
	}
}
