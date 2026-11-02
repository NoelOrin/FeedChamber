package repository

import (
	"context"
	"errors"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"time"
)

type sqlOAuthRepo struct{ db db.SQLStore }

func (r *sqlOAuthRepo) Create(ctx context.Context, account *model.OAuthAccount) error {
	if account.CreatedAt.IsZero() {
		account.CreatedAt = time.Now()
	}
	if account.UpdatedAt.IsZero() {
		account.UpdatedAt = time.Now()
	}
	return r.db.DB().WithContext(ctx).Create(account).Error
}

func (r *sqlOAuthRepo) FindByProviderAndProviderID(ctx context.Context, provider model.OAuthProvider, providerID string) (*model.OAuthAccount, error) {
	var account model.OAuthAccount
	err := WrapDBErr(r.db.DB().WithContext(ctx).
		Where("provider = ? AND provider_id = ?", provider, providerID).
		First(&account).Error)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return &account, err
}

func (r *sqlOAuthRepo) FindByUserID(ctx context.Context, userID string) ([]*model.OAuthAccount, error) {
	var accounts []*model.OAuthAccount
	err := r.db.DB().WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&accounts).Error
	return accounts, err
}

func (r *sqlOAuthRepo) FindByUserIDAndProvider(ctx context.Context, userID string, provider model.OAuthProvider) (*model.OAuthAccount, error) {
	var account model.OAuthAccount
	err := WrapDBErr(r.db.DB().WithContext(ctx).
		Where("user_id = ? AND provider = ?", userID, provider).
		First(&account).Error)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return &account, err
}

func (r *sqlOAuthRepo) Update(ctx context.Context, account *model.OAuthAccount) error {
	account.UpdatedAt = time.Now()
	return r.db.DB().WithContext(ctx).Where(SQLSoftDeleteCondition()).Save(account).Error
}

func (r *sqlOAuthRepo) Delete(ctx context.Context, accountID string) error {
	return r.db.DB().WithContext(ctx).Unscoped().Delete(&model.OAuthAccount{}, "id = ?", accountID).Error
}
