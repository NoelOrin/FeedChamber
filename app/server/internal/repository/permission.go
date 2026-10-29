package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
)

type PermissionRepository interface {
	List(ctx context.Context) ([]model.Permission, error)
	GetByCode(ctx context.Context, code string) (*model.Permission, error)
	CreateIfNotExists(ctx context.Context, perm *model.Permission) error
	GetRolePermissions(ctx context.Context, roleName string) ([]string, error)
	GetAllRolePermissions(ctx context.Context) (map[string][]string, error)
	SyncRolePermissions(ctx context.Context, roleName string, permCodes []string) error
}

func NewPermissionRepository(store db.Store) PermissionRepository {
	return &sqlPermRepo{db: store.SQL()}
}
