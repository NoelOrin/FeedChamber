package service

import (
	"context"

	"feedchamber/internal/model"
	"feedchamber/internal/repository"
)

// PermissionChecker 轻量查询接口——只读权限检查，供其他 Service 注入。
type PermissionChecker interface {
	HasPermission(roleName, permCode string) bool
	// HasPermissionInSpace 覆盖式解析 + NGA 层级继承：从当前版面向上找最近覆盖，否则回退全局。
	HasPermissionInSpace(roleName, spaceID, permCode string) bool
	// HasPermissionForUser 聚合用户角色权限与所属组权限。
	HasPermissionForUser(ctx context.Context, userID, roleName, permCode string) bool
	// HasPermissionInSpaceForUser 用户维度的版块覆盖权限检查（层级继承）。
	HasPermissionInSpaceForUser(ctx context.Context, userID, roleName, spaceID, permCode string) bool
	GetRolePermissions(roleName string) []string
}

// PermissionService 完整的权限管理接口，嵌入 PermissionChecker。
type PermissionService interface {
	PermissionChecker

	LoadCache(ctx context.Context) error
	ReloadGroupCache(ctx context.Context) error
	StartSyncSubscriber(ctx context.Context) error
	ListPermissions(ctx context.Context) ([]model.Permission, error)
	SyncRolePermissions(ctx context.Context, roleName string, permCodes []string) error
	// GetSpaceRolePermissions 返回 (spaceID, roleName) 覆盖权限码与是否存在覆盖（仅当前版面，不继承）。
	GetSpaceRolePermissions(ctx context.Context, spaceID, roleName string) ([]string, bool, error)
	// GetEffectiveSpaceRolePermissions 返回层级继承后的有效权限及来源。
	GetEffectiveSpaceRolePermissions(ctx context.Context, spaceID, roleName string) ([]string, string, bool, bool, error)
	// ListSpacesWithEffectivePermissions 批量返回多个版面的有效权限（用于树 UI）。
	ListSpacesWithEffectivePermissions(ctx context.Context, roleName string, spaceIDs []string) (map[string][]string, error)
	// SyncSpaceRolePermissions 设置/更新某版块某角色的覆盖（覆盖式，需 ≥1 权限）。
	SyncSpaceRolePermissions(ctx context.Context, spaceID, roleName string, permCodes []string) error
	// ClearSpaceRoleOverride 删除某版块某角色的覆盖，回退全局/继承。
	ClearSpaceRoleOverride(ctx context.Context, spaceID, roleName string) error
	// Space hierarchy cache (NGA)
	SetSpaceRepository(repo repository.SpaceRepository)
	RefreshSpaceParents(ctx context.Context) error
	UpdateSpaceParent(spaceID, parentID string)
	RemoveSpace(spaceID string)
}
