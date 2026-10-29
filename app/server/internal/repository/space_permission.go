package repository

import (
	"context"
	"orinforum/internal/db"
)

type SpaceRolePermissionRepository interface {
	GetSpaceRolePermissions(ctx context.Context, spaceID, roleName string) ([]string, bool, error)
	GetAllSpaceRolePermissions(ctx context.Context) (map[string]map[string][]string, error)
	SyncSpaceRolePermissions(ctx context.Context, spaceID, roleName string, permCodes []string) error
	ClearSpaceRolePermissions(ctx context.Context, spaceID, roleName string) error
}

func NewSpaceRolePermissionRepository(store db.Store) SpaceRolePermissionRepository {
	return &sqlSpaceRolePermRepo{db: store.SQL()}
}
