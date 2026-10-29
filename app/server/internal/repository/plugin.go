package repository

import (
	"context"

	"orinforum/internal/db"
	"orinforum/internal/model"
)

type PluginRepository interface {
	Create(ctx context.Context, p *model.Plugin) error
	Update(ctx context.Context, p *model.Plugin) error
	Get(ctx context.Context, id string) (*model.Plugin, error)
	List(ctx context.Context) ([]model.Plugin, error)
	Delete(ctx context.Context, id string) error
}

func NewPluginRepository(store db.Store) PluginRepository {
	return &sqlPluginRepo{db: store.SQL()}
}
