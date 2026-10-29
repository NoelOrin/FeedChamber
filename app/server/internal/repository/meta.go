package repository

import (
	"context"
	"orinforum/internal/db"
)

type MetaRepository interface {
	// Get returns the value for key. found is false when the key is absent.
	Get(ctx context.Context, key string) (value string, found bool, err error)
	Set(ctx context.Context, key, value string) error
	// Delete removes the key. It is a no-op when the key is absent.
	Delete(ctx context.Context, key string) error
}

func NewMetaRepository(store db.Store) MetaRepository {
	return &sqlMetaRepo{db: store.SQL()}
}
