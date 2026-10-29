package repository

import (
	"context"

	"orinforum/internal/db"
	"orinforum/internal/model"
)

type ModerationRepository interface {
	// Content moderation records
	CreateModeration(ctx context.Context, m *model.ContentModeration) error
	GetModeration(ctx context.Context, id string) (*model.ContentModeration, error)
	ListModerations(ctx context.Context, status model.ModerationStatus, offset, limit int) ([]model.ContentModeration, int64, error)
	UpdateModerationStatus(ctx context.Context, id string, status model.ModerationStatus, reviewerID string) error

	// Rules
	CreateRule(ctx context.Context, r *model.ModerationRule) error
	GetRule(ctx context.Context, id string) (*model.ModerationRule, error)
	ListRules(ctx context.Context, offset, limit int) ([]model.ModerationRule, int64, error)
	ListEnabledRules(ctx context.Context, scene model.ModerationScene) ([]model.ModerationRule, error)
	UpdateRule(ctx context.Context, r *model.ModerationRule) error
	DeleteRule(ctx context.Context, id string) error

	// Configs
	GetConfig(ctx context.Context, key string) (*model.ModerationConfig, error)
	SetConfig(ctx context.Context, key, value string) error
	ListConfigs(ctx context.Context) ([]model.ModerationConfig, error)
}

func NewModerationRepository(store db.Store) ModerationRepository {
	return &sqlModerationRepo{db: store.SQL()}
}
