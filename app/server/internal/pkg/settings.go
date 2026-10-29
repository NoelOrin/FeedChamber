package pkg

import (
	"context"
	"os"
	"strconv"
	"time"

	"feedchamber/internal/mailer"
	"feedchamber/internal/storage"
)

// SettingGetter 抽象 SettingService.Get，避免 pkg 包直接依赖 service 层。
type SettingGetter interface {
	Get(ctx context.Context, key string) (string, error)
}

// GetSettingOrDefault 读取优先级：settings 表 → 环境变量 → 硬编码默认值。
func GetSettingOrDefault(ctx context.Context, sg SettingGetter, key, envKey, hardDefault string) string {
	if sg != nil {
		if v, _ := sg.Get(ctx, key); v != "" {
			return v
		}
	}
	if envKey != "" {
		if v := os.Getenv(envKey); v != "" {
			return v
		}
	}
	return hardDefault
}

// GetSettingInt 读取整数配置项。
func GetSettingInt(ctx context.Context, sg SettingGetter, key, envKey string, hardDefault int) int {
	s := GetSettingOrDefault(ctx, sg, key, envKey, strconv.Itoa(hardDefault))
	v, err := strconv.Atoi(s)
	if err != nil {
		return hardDefault
	}
	return v
}

// GetSettingDuration 读取秒数配置项，返回 time.Duration。
func GetSettingDuration(ctx context.Context, sg SettingGetter, key, envKey string, hardDefaultSec int) time.Duration {
	return time.Duration(GetSettingInt(ctx, sg, key, envKey, hardDefaultSec)) * time.Second
}

// ── OAuth2 access token TTL（启动时从 settings 加载）────────

// OAuthAccessTokenTTL is the lifetime of opaque OAuth2/OIDC access tokens.
// Loaded once at startup; changing the setting requires a restart (issuance and
// verification both read this value, so a hot reload would only affect new rows).
var OAuthAccessTokenTTL = 24 * time.Hour

// InitOAuthTTL 从 settings 读取 OAuth access token 的有效期。
func InitOAuthTTL(ctx context.Context, sg SettingGetter) {
	OAuthAccessTokenTTL = GetSettingDuration(ctx, sg, "oauth_access_token_ttl", "OAUTH_ACCESS_TOKEN_TTL", 86400)
}

// ── 分页配置（启动时从 settings 加载）─────────────────────

var (
	PageMaxSize     = 100
	PageDefaultSize = 20
)

// InitPagination 从 settings 读取分页配置。
func InitPagination(ctx context.Context, sg SettingGetter) {
	PageMaxSize = GetSettingInt(ctx, sg, "page_max_size", "PAGE_MAX_SIZE", 100)
	PageDefaultSize = GetSettingInt(ctx, sg, "page_default_size", "PAGE_DEFAULT_SIZE", 20)
}

// BuildStorageConfig reads the storage configuration from settings.
// Storage is a startup singleton, so this is called once at startup.
func BuildStorageConfig(ctx context.Context, sg SettingGetter) storage.Config {
	return storage.Config{
		Provider:        GetSettingOrDefault(ctx, sg, "storage_provider", "STORAGE_PROVIDER", "local"),
		LocalDir:        GetSettingOrDefault(ctx, sg, "storage_local_dir", "STORAGE_LOCAL_DIR", "uploads"),
		Endpoint:        GetSettingOrDefault(ctx, sg, "s3_endpoint", "S3_ENDPOINT", ""),
		Region:          GetSettingOrDefault(ctx, sg, "s3_region", "S3_REGION", "us-east-1"),
		Bucket:          GetSettingOrDefault(ctx, sg, "s3_bucket", "S3_BUCKET", ""),
		AccessKeyID:     GetSettingOrDefault(ctx, sg, "s3_access_key_id", "S3_ACCESS_KEY_ID", ""),
		AccessKeySecret: GetSettingOrDefault(ctx, sg, "s3_access_key_secret", "S3_ACCESS_KEY_SECRET", ""),
		UseSSL:          GetSettingOrDefault(ctx, sg, "s3_use_ssl", "S3_USE_SSL", "true") == "true",
		CustomDomain:    GetSettingOrDefault(ctx, sg, "s3_custom_domain", "S3_CUSTOM_DOMAIN", ""),
	}
}

// BuildMailerConfig reads the mailer configuration from settings.
// The mailer config is re-read on every send so admin config changes take effect
// without restarting the server.
func BuildMailerConfig(ctx context.Context, sg SettingGetter) mailer.Config {
	return mailer.Config{
		Provider: GetSettingOrDefault(ctx, sg, "email_provider", "EMAIL_PROVIDER", "log"),
		From:     GetSettingOrDefault(ctx, sg, "email_from", "EMAIL_FROM", ""),
		FromName: GetSettingOrDefault(ctx, sg, "email_from_name", "EMAIL_FROM_NAME", "Feed Chamber"),
		Host:     GetSettingOrDefault(ctx, sg, "email_smtp_host", "EMAIL_SMTP_HOST", ""),
		Port:     GetSettingInt(ctx, sg, "email_smtp_port", "EMAIL_SMTP_PORT", 587),
		Username: GetSettingOrDefault(ctx, sg, "email_username", "EMAIL_USERNAME", ""),
		Password: GetSettingOrDefault(ctx, sg, "email_password", "EMAIL_PASSWORD", ""),
		UseTLS:   GetSettingOrDefault(ctx, sg, "email_use_tls", "EMAIL_USE_TLS", "false") == "true",
		APIKey:   GetSettingOrDefault(ctx, sg, "email_api_key", "EMAIL_API_KEY", ""),
	}
}
