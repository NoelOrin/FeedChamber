package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type OAuthRepository interface {
	Create(ctx context.Context, account *model.OAuthAccount) error
	FindByProviderAndProviderID(ctx context.Context, provider model.OAuthProvider, providerID string) (*model.OAuthAccount, error)
	FindByUserID(ctx context.Context, userID string) ([]*model.OAuthAccount, error)
	FindByUserIDAndProvider(ctx context.Context, userID string, provider model.OAuthProvider) (*model.OAuthAccount, error)
	Update(ctx context.Context, account *model.OAuthAccount) error
	Delete(ctx context.Context, accountID string) error
}

func NewOAuthRepository(store db.Store) OAuthRepository {
	return &sqlOAuthRepo{db: store.SQL()}
}
