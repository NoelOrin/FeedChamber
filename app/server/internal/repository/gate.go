package repository

import (
	"context"

	"orinforum/internal/db"
	"orinforum/internal/model"
)

type GateRepository interface {
	ListRules(ctx context.Context) ([]model.GateRule, error)
	GetRule(ctx context.Context, id string) (*model.GateRule, error)
	CreateRule(ctx context.Context, rule *model.GateRule) error
	UpdateRule(ctx context.Context, rule *model.GateRule) error
	DeleteRule(ctx context.Context, id string) error

	ListDomainDefaults(ctx context.Context) ([]model.GateDomainDefault, error)
	UpsertDomainDefault(ctx context.Context, def *model.GateDomainDefault) error
}

func NewGateRepository(store db.Store) GateRepository {
	return &sqlGateRepo{db: store.SQL()}
}
