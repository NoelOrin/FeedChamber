package model

import (
	"strings"
	"time"
)

const (
	SettingPostAuthRequired         = "post_auth_required"
	SettingCommentAuthRequired      = "comment_auth_required"
	SettingSpaceViewAuthRequired    = "space_view_auth_required"
	SettingTagListAuthRequired      = "tag_list_auth_required"
	SettingAnnouncementAuthRequired = "announcement_auth_required"
	SettingOAuthGitHub              = "oauth_github_enabled"
	SettingOAuthGoogle              = "oauth_google_enabled"
	SettingOAuthQQ                  = "oauth_qq_enabled"
	SettingOAuthWeChat              = "oauth_wechat_enabled"
	SettingOAuthWeibo               = "oauth_weibo_enabled"
	SettingOAuthDiscord             = "oauth_discord_enabled"
	// Email
	SettingEmailEnabled  = "email_enabled"
	SettingEmailProvider = "email_provider"
	SettingEmailSMTPHost = "email_smtp_host"
	SettingEmailSMTPPort = "email_smtp_port"
	SettingEmailUsername = "email_username"
	SettingEmailPassword = "email_password"
	SettingEmailFrom     = "email_from"
	SettingEmailFromName = "email_from_name"
	SettingEmailUseTLS   = "email_use_tls"
	SettingEmailAPIKey   = "email_api_key"
	// Email code settings
	SettingEmailCodeTTL        = "email_code_ttl"
	SettingEmailCodeLength     = "email_code_length"
	SettingEmailResendCooldown = "email_resend_cooldown"
	SettingEmailDailyLimit     = "email_daily_limit"
	SettingEmailNotifyEnabled  = "email_notify_enabled"
	SettingEmailDigestEnabled  = "email_digest_enabled"
	// SSO/OIDC
	SettingSSOOIDCEnabled      = "sso_oidc_enabled"
	SettingSSOOIDCIssuer       = "sso_oidc_issuer"
	SettingSSOOIDCClientID     = "sso_oidc_client_id"
	SettingSSOOIDCClientSecret = "sso_oidc_client_secret"
	// API Key
	SettingAPIKeyEnabled          = "api_key_enabled"
	SettingAPIKeyDefaultRateLimit = "api_key_default_rate_limit"
	// Passkey
	// Register
	SettingRegisterMode    = "register_mode"
	SettingRegisterEnabled = "register_enabled"
	// MFA
	SettingMFATOTPEnabled = "mfa_totp_enabled"
	// Security/Risk Control
	// SettingRateLimitEnabled toggles the gateway's global per-IP ceiling.
	SettingRateLimitEnabled               = "rate_limit_enabled"
	SettingSecurityBruteForceMaxAttempts  = "security_brute_force_max_attempts"
	SettingSecurityBruteForceWindow       = "security_brute_force_window"
	SettingSecurityUnusualLocationEnabled = "security_unusual_location_enabled"
	// Backup
	SettingBackupRetentionDays = "backup_retention_days"
	// SEO
	SettingSEODescription         = "seo_description"
	SettingSEOKeywords            = "seo_keywords"
	SettingSEOTitle               = "seo_title"
	SettingOGImage                = "og_image"
	SettingGoogleSiteVerification = "google_site_verification"
	SettingBaiduSiteVerification  = "baidu_site_verification"
	SettingUserProfileIndexable   = "user_profile_indexable"
	SettingSitemapPageSize        = "sitemap_page_size"
	// Sitemap protocol hard limit is 50000 URLs per file.
	SitemapMaxPageSize = 50000
	// Moderation
	// SettingModerationMode 是文章审核模式（off / async / external / manual），
	// 取值与默认值见 model/moderation.go 的 ModerationMode* 常量。
	SettingModerationMode = "moderation_mode"
)

// Setting represents a key-value system setting.
type Setting struct {
	Key       string `gorm:"primaryKey;size:64"`
	Value     string `gorm:"type:text"`
	UpdatedAt time.Time
}

func (Setting) TableName() string { return "settings" }

// DefaultSettings are seeded on first startup.
var DefaultSettings = map[string]string{
	"site_name":        "OrinForum",
	"site_description": "NoelOrin 的个人博客",
	"site_logo":        "",
	"site_url":         "",
	"seo_title":        "OrinForum",
	"seo_keywords":     "OrinForum,博客,社区,技术",
	// seo_description 曾被 app/web 的 generateMetadata 读取，却既不在本表也不在
	// PublicSettingKeys 中——读取即落空、永远回退到 site_description。此处补齐种子。
	"seo_description": "",
	"og_image":        "",
	// 站长平台验证值。内容本身就要公开在 <meta> 里，不敏感，故列入公开设置。
	"google_site_verification": "",
	"baidu_site_verification":  "",
	// 他人主页是 UGC 薄内容，默认不索引；管理员可按运营需要开启。
	"user_profile_indexable": "false",
	// sitemap 单片条目数，协议上限 50000；默认 5000 兼顾单次响应体积。
	"sitemap_page_size": "5000",
	"register_enabled":  "true",
	// register_mode: all | email | username | invite | approve
	"register_mode": "all",
	// post_approval 是旧的布尔审核开关，保留只为一键兼容：true 等价于
	// moderation_mode=manual。新代码应读写 moderation_mode。
	"post_approval": "false",
	// moderation_mode: off | async | external | manual（默认 async，见 DefaultModerationMode）
	"moderation_mode": "async",
	// TL0 信任等级发帖强制待审。默认 false：TL0 是注册用户的起点等级
	// （含 admin，晋升条件为「≥1 天且 ≥1 篇」），默认 true 会把所有人的新帖
	// 静默改成 pending，而公开列表只显示 approved/closed → 发帖成功但全站不可见。
	// 该 key 此前既不在本表也不在 PublicSettingKeys 中，读取永远落回硬默认。
	// DEPRECATED（2026-10-29）：TL0 发帖门已由 gate_rules 接管；本 key 仅保留
	// 以兼容旧库，代码不再读取。需要「TL0 转待审」请建 post_create 规则。
	"trust_tl0_require_approval": "false",
	// DEPRECATED（2026-10-29）：外链门已由 gate_rules 接管（seed 了 TL0 deny +
	// 等级 ≥1 allow）；本 key 代码不再读取。
	"trust_tl0_block_links":       "true",
	"email_verification_required": "false",
	"site_maintenance":            "false",
	"site_maintenance_message":    "站点维护中，请稍后再试",
	"post_auth_required":          "false",
	"comment_auth_required":       "false",
	// Auth gate feature flags: unset (absent) gates already default to allow
	// via cachedSettingGet, but seeding them keeps /settings/public complete
	// and lets admins toggle them from the dashboard without surprises.
	"space_view_auth_required":   "false",
	"tag_list_auth_required":     "false",
	"announcement_auth_required": "false",
	"article_page_size":          "20",
	// OAuth providers
	"oauth_github_enabled":       "false",
	"oauth_github_client_id":     "",
	"oauth_github_client_secret": "",
	"oauth_github_callback":      "",
	"oauth_google_enabled":       "false",
	"oauth_google_client_id":     "",
	"oauth_google_client_secret": "",
	"oauth_google_callback":      "",
	"oauth_qq_enabled":           "false",
	"oauth_qq_client_id":         "",
	"oauth_qq_client_secret":     "",
	"oauth_qq_callback":          "",
	"oauth_wechat_enabled":       "false",
	"oauth_wechat_client_id":     "",
	"oauth_wechat_client_secret": "",
	"oauth_wechat_callback":      "",
	"oauth_weibo_enabled":        "false",
	"oauth_weibo_client_id":      "",
	"oauth_weibo_client_secret":  "",
	"oauth_weibo_callback":       "",
	// Discord
	"oauth_discord_enabled":       "false",
	"oauth_discord_client_id":     "",
	"oauth_discord_client_secret": "",
	"oauth_discord_callback":      "",
	// SSO/OIDC
	"sso_oidc_enabled":       "false",
	"sso_oidc_issuer":        "",
	"sso_oidc_client_id":     "",
	"sso_oidc_client_secret": "",
	// API Key
	"api_key_enabled":            "false",
	"api_key_default_rate_limit": "100",
	// Passkey
	// MFA
	"mfa_totp_enabled": "false",
	// Security/Risk Control
	// rate_limit_enabled gates the gateway's global per-IP ceiling
	// (internal/gateway.RateLimiter). It used to be read but never seeded:
	// every request took a miss on the settings table and logged GORM's
	// "record not found", because a miss is not cacheable. Opt-in, matching
	// the gateway's documented default (loadConfig treats "absent" as false).
	"rate_limit_enabled":                "false",
	"security_brute_force_max_attempts": "5",
	"security_brute_force_window":       "300",
	"security_unusual_location_enabled": "false",
	"backup_retention_days":             "30",
	// OAuth endpoint/scope overrides (M-26). Empty = use the hardcoded
	// default in OAuthProviderConfigs. Set to customize endpoints/scopes
	// for self-hosted or enterprise OAuth providers.
	"oauth_github_auth_url":     "",
	"oauth_github_token_url":    "",
	"oauth_github_user_api_url": "",
	"oauth_github_scopes":       "",
	"oauth_google_auth_url":     "",
	"oauth_google_token_url":    "",
	"oauth_google_user_api_url": "",
	"oauth_google_scopes":       "",
	"oauth_qq_auth_url":         "",
	"oauth_qq_token_url":        "",
	"oauth_qq_user_api_url":     "",
	"oauth_qq_scopes":           "",
	"oauth_wechat_auth_url":     "",
	"oauth_wechat_token_url":    "",
	"oauth_wechat_user_api_url": "",
	"oauth_wechat_scopes":       "",
	"oauth_weibo_auth_url":      "",
	"oauth_weibo_token_url":     "",
	"oauth_weibo_user_api_url":  "",
	"oauth_weibo_scopes":        "",
	// Discord
	"oauth_discord_auth_url":     "",
	"oauth_discord_token_url":    "",
	"oauth_discord_user_api_url": "",
	"oauth_discord_scopes":       "",
	// Email
	"email_enabled":   "false",
	"email_provider":  "log",
	"email_smtp_host": "",
	"email_smtp_port": "587",
	"email_username":  "",
	"email_password":  "",
	"email_from":      "",
	"email_from_name": "OrinForum",
	"email_use_tls":   "false",
	"email_api_key":   "",
	// Email code
	"email_code_ttl":        "600",
	"email_code_length":     "6",
	"email_resend_cooldown": "60",
	"email_daily_limit":     "10",
	// OAuth2 / OIDC（access token 为 opaque，仅 id_token 是 JWT）
	"oauth_access_token_ttl": "86400",
	"sso_id_token_ttl":       "3600",
	// 分页
	"page_max_size":     "100",
	"page_default_size": "20",
	// 上传
	"upload_max_size":       "10485760",
	"upload_allowed_types":  "image/jpeg,image/png,image/gif,image/webp",
	"upload_presign_expiry": "900",
	// SSE / 浏览量
	"sse_polling_interval":      "5",
	"view_count_flush_interval": "60",
	// 安全
	"bcrypt_cost":         "10",
	"password_min_length": "8",
	// 存储（S3/本地）唯一规范键集；启动时由 pkg.BuildStorageConfig 读取，
	// 空值回退到环境变量 STORAGE_*/S3_*。后台修改后需重启服务生效。
	"storage_provider":            "local",
	"storage_local_dir":           "uploads",
	"s3_endpoint":                 "",
	"s3_region":                   "us-east-1",
	"s3_bucket":                   "",
	"s3_access_key_id":            "",
	"s3_access_key_secret":        "",
	"s3_use_ssl":                  "true",
	"s3_custom_domain":            "",
	"banned_status_visible":       "true",
	"banned_status_public_access": "true",
}

// PublicSettingKeys are settings readable without authentication.
var PublicSettingKeys = []string{
	"site_name",
	"site_description",
	"site_logo",
	"site_url",
	"seo_title",
	"seo_keywords",
	"seo_description",
	"og_image",
	"google_site_verification",
	"baidu_site_verification",
	"user_profile_indexable",
	"register_enabled",
	"register_mode",
	"post_auth_required",
	"comment_auth_required",
	"space_view_auth_required",
	"tag_list_auth_required",
	"announcement_auth_required",
	"article_page_size",
	"page_default_size",
}

// secretSettingKeyMarkers are substrings that identify settings whose values
// must never be persisted in audit logs.
var secretSettingKeyMarkers = []string{
	"password",
	"secret",
	"api_key",
	"access_key",
	"token",
}

// IsSecretSettingKey reports whether a setting key stores a secret value
// that must not appear in audit logs.
func IsSecretSettingKey(key string) bool {
	k := strings.ToLower(key)
	for _, marker := range secretSettingKeyMarkers {
		if strings.Contains(k, marker) {
			return true
		}
	}
	return false
}

// RedactSettingAuditDetail returns an audit-safe detail for a setting update.
// Secret keys are replaced with a fixed redaction marker; non-secret values
// are returned unchanged.
func RedactSettingAuditDetail(key, value string) string {
	if IsSecretSettingKey(key) {
		return "[REDACTED]"
	}
	return value
}
