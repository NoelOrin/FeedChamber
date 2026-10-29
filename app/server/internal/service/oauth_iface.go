package service

import (
	"context"
	"time"

	"feedchamber/internal/model"
)

type OAuthService interface {
	// OAuth flow
	GetAuthURL(ctx context.Context, provider model.OAuthProvider, state string) (string, error)
	HandleCallback(ctx context.Context, provider model.OAuthProvider, code string, state string) (*model.OAuthUser, error)
	GetUserProfile(provider model.OAuthProvider, accessToken string) (*model.OAuthUser, error)

	// Account lookup
	FindByProviderAndProviderID(ctx context.Context, provider model.OAuthProvider, providerID string) (*model.OAuthAccount, error)
	FindUserByUUID(ctx context.Context, uuid string) (*model.User, error)

	// Account management
	LinkAccount(ctx context.Context, userID string, provider model.OAuthProvider, providerID, openid, unionid string, avatar, name, email, token, refreshToken string, expiresAt *time.Time) error
	UnlinkAccount(ctx context.Context, userID string, provider model.OAuthProvider) error
	GetLinkedAccounts(ctx context.Context, userID string) ([]*model.OAuthAccount, error)

	// Config
	GetProviderConfig(ctx context.Context, provider model.OAuthProvider) (model.OAuthConfig, bool)
	GetAllProviderConfigs(ctx context.Context) map[model.OAuthProvider]model.OAuthConfig
}
