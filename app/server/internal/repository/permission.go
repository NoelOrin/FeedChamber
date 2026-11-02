package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type PermissionRepository interface {
	List(ctx context.Context) ([]model.Permission, error)
	GetByCode(ctx context.Context, code string) (*model.Permission, error)
	CreateIfNotExists(ctx context.Context, perm *model.Permission) error
	GetRolePermissions(ctx context.Context, roleName string) ([]string, error)
	GetAllRolePermissions(ctx context.Context) (map[string][]string, error)
	SyncRolePermissions(ctx context.Context, roleName string, permCodes []string) error
	// EnsureRolePermissions 只追加缺失映射，不删除已有映射（幂等、非破坏性）。
	// 用于版本升级时给内置角色补齐新增的权限码。
	EnsureRolePermissions(ctx context.Context, roleName string, permCodes []string) (int, error)
}

func NewPermissionRepository(store db.Store) PermissionRepository {
	return &sqlPermRepo{db: store.SQL()}
}
