package service

import (
	"context"
	"testing"

	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg/testnats"
	"feedchamber/internal/repository"

	"github.com/stretchr/testify/require"
)

func newPermSpaceSvc(t *testing.T) (PermissionService, db.Store) {
	t.Helper()
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: t.TempDir() + "/test.db"})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrate(
		&model.Permission{},
		&model.RolePermission{},
		&model.SpaceRolePermission{},
		&model.Group{},
		&model.GroupMember{},
		&model.GroupPermission{},
	))
	kv := testnats.NewKVStore(t, "permission_space_test")

	permRepo := repository.NewPermissionRepository(store)
	ctx := context.Background()
	for _, code := range []string{"post:read", "post:delete", "post:pin"} {
		require.NoError(t, permRepo.CreateIfNotExists(ctx, &model.Permission{Code: code, Name: code}))
	}
	// 全局 moderator = {post:read, post:delete}
	require.NoError(t, permRepo.SyncRolePermissions(ctx, "moderator", []string{"post:read", "post:delete"}))

	spaceRepo := repository.NewSpaceRolePermissionRepository(store)
	svc := NewPermissionService(permRepo, spaceRepo, repository.NewGroupRepository(store), kv)
	require.NoError(t, svc.LoadCache(ctx))
	return svc, store
}

func TestHasPermissionInSpace_FallbackToGlobal(t *testing.T) {
	svc, _ := newPermSpaceSvc(t)
	// 无覆盖 → 回退全局
	require.True(t, svc.HasPermissionInSpace("moderator", "spaceA", "post:delete"))
	require.False(t, svc.HasPermissionInSpace("moderator", "spaceA", "post:pin"))
}

func TestHasPermissionInSpace_OverrideReplaces(t *testing.T) {
	svc, _ := newPermSpaceSvc(t)
	ctx := context.Background()
	// spaceA 覆盖 moderator = {post:read, post:pin}（去掉 delete，加上 pin）
	require.NoError(t, svc.SyncSpaceRolePermissions(ctx, "spaceA", "moderator",
		[]string{"post:read", "post:pin"}))

	// 覆盖完全替换：delete 被收回，pin 被授予
	require.False(t, svc.HasPermissionInSpace("moderator", "spaceA", "post:delete"))
	require.True(t, svc.HasPermissionInSpace("moderator", "spaceA", "post:pin"))
	require.True(t, svc.HasPermissionInSpace("moderator", "spaceA", "post:read"))

	// 其他版块不受影响，仍回退全局
	require.True(t, svc.HasPermissionInSpace("moderator", "spaceB", "post:delete"))
	require.False(t, svc.HasPermissionInSpace("moderator", "spaceB", "post:pin"))

	// 空 spaceID 永远走全局
	require.True(t, svc.HasPermissionInSpace("moderator", "", "post:delete"))
}

func TestSyncSpace_ThenClear(t *testing.T) {
	svc, _ := newPermSpaceSvc(t)
	ctx := context.Background()
	require.NoError(t, svc.SyncSpaceRolePermissions(ctx, "spaceA", "moderator", []string{"post:read"}))
	require.False(t, svc.HasPermissionInSpace("moderator", "spaceA", "post:delete"))

	require.NoError(t, svc.ClearSpaceRoleOverride(ctx, "spaceA", "moderator"))
	// 清除后回退全局
	require.True(t, svc.HasPermissionInSpace("moderator", "spaceA", "post:delete"))
}
