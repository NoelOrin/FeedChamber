package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
)

// CredentialRepository handles password credential storage separately from the User model.
// All password operations (create, verify, update) go through this repository.
type CredentialRepository interface {
	FindByUserID(ctx context.Context, userID string) (*model.Credential, error)
	Upsert(ctx context.Context, c *model.Credential) error
	Delete(ctx context.Context, userID string) error
}

func NewCredentialRepository(store db.Store) CredentialRepository {
	return &sqlCredentialRepo{db: store.SQL()}
}
