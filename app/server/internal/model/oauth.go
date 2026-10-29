package model

import (
	"time"

	"gorm.io/gorm"
)

// OAuthProvider represents a supported OAuth provider.
type OAuthProvider string

const (
	ProviderGitHub  OAuthProvider = "github"
	ProviderGoogle  OAuthProvider = "google"
	ProviderQQ      OAuthProvider = "qq"
	ProviderWeChat  OAuthProvider = "wechat"
	ProviderWeibo   OAuthProvider = "weibo"
	ProviderDiscord OAuthProvider = "discord"
)

// OAuthAccount stores OAuth account associations.
type OAuthAccount struct {
	ID           string         `gorm:"primaryKey" json:"id"`
	UserID       string         `gorm:"index;not null" json:"user_id"`
	Provider     OAuthProvider  `gorm:"not null;uniqueIndex:idx_oauth_provider_account" json:"provider"`
	ProviderID   string         `gorm:"not null;uniqueIndex:idx_oauth_provider_account" json:"provider_id"`
	OpenID       string         `gorm:"size:255;index" json:"openid"`
	UnionID      string         `gorm:"size:255" json:"unionid"`
	Avatar       string         `json:"avatar"`
	Name         string         `gorm:"size:100" json:"name"`
	Email        string         `gorm:"size:255" json:"email"`
	Token        string         `json:"-"` // OAuth access token (stored separately)
	RefreshToken string         `json:"-"` // OAuth refresh token
	ExpiresAt    *time.Time     `json:"expires_at"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggertype:"string"`
}

func (OAuthAccount) TableName() string { return "oauth_accounts" }

// OAuthConfig represents OAuth provider configuration.
type OAuthConfig struct {
	Enabled      bool     `json:"enabled"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	Scopes       []string `json:"scopes"`
	AuthURL      string   `json:"auth_url"`
	TokenURL     string   `json:"token_url"`
	UserAPIURL   string   `json:"user_api_url"`
	CallbackURL  string   `json:"callback_url"`
}

// OAuthProviderConfigs stores OAuth configurations for all providers.
var OAuthProviderConfigs = map[OAuthProvider]OAuthConfig{
	ProviderGitHub: {
		Enabled:      false,
		ClientID:     "",
		ClientSecret: "",
		Scopes:       []string{"user:email"},
		AuthURL:      "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		UserAPIURL:   "https://api.github.com/user",
		CallbackURL:  "",
	},
	ProviderGoogle: {
		Enabled:      false,
		ClientID:     "",
		ClientSecret: "",
		Scopes:       []string{"email", "profile"},
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		UserAPIURL:   "https://www.googleapis.com/oauth2/v2/userinfo",
		CallbackURL:  "",
	},
	ProviderQQ: {
		Enabled:      false,
		ClientID:     "",
		ClientSecret: "",
		Scopes:       []string{"get_user_info"},
		AuthURL:      "https://graph.qq.com/oauth2.0/authorize",
		TokenURL:     "https://graph.qq.com/oauth2.0/token",
		UserAPIURL:   "https://graph.qq.com/user/get_user_info",
		CallbackURL:  "",
	},
	ProviderWeChat: {
		Enabled:      false,
		ClientID:     "",
		ClientSecret: "",
		Scopes:       []string{"snsapi_login"},
		AuthURL:      "https://open.weixin.qq.com/connect/qrconnect",
		TokenURL:     "https://api.weixin.qq.com/sns/oauth2/access_token",
		UserAPIURL:   "https://api.weixin.qq.com/sns/userinfo",
		CallbackURL:  "",
	},
	ProviderWeibo: {
		Enabled:      false,
		ClientID:     "",
		ClientSecret: "",
		Scopes:       []string{},
		AuthURL:      "https://api.weibo.com/oauth2/authorize",
		TokenURL:     "https://api.weibo.com/oauth2/access_token",
		UserAPIURL:   "https://api.weibo.com/2/users/show.json",
		CallbackURL:  "",
	},
	ProviderDiscord: {
		Enabled:      false,
		ClientID:     "",
		ClientSecret: "",
		Scopes:       []string{"identify", "email"},
		AuthURL:      "https://discord.com/api/oauth2/authorize",
		TokenURL:     "https://discord.com/api/oauth2/token",
		UserAPIURL:   "https://discord.com/api/users/@me",
		CallbackURL:  "",
	},
}

// OAuthUser represents user info returned by OAuth provider.
// Token fields are populated internally and not exposed in API responses.
type OAuthUser struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar"`
	Provider string `json:"provider"`

	AccessToken  string     `json:"-"`
	RefreshToken string     `json:"-"`
	ExpiresAt    *time.Time `json:"-"`
}
