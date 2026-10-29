package server

import (
	"context"
	"path/filepath"
	"testing"

	"orinforum/internal/config"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/repository"

	"github.com/stretchr/testify/require"
)

func newSeedTestStore(t *testing.T) db.Store {
	t.Helper()
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "seed.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(&model.Space{}, &model.Tag{}, &model.PostTag{}))
	return store
}

func TestSeedBaselineSpacesAndTags_FirstBoot(t *testing.T) {
	store := newSeedTestStore(t)

	seedBaselineSpaces(store, false)
	seedBaselineTags(store, false)

	var spaces []model.Space
	require.NoError(t, store.SQL().DB().Find(&spaces).Error)
	require.Len(t, spaces, len(seedSpaces))

	sortOrders := map[int]bool{}
	for _, s := range spaces {
		require.NotEmpty(t, s.ID)
		require.NotEmpty(t, s.Description, "space %s", s.Slug)
		require.NotEmpty(t, s.Icon, "space %s", s.Slug)
		require.Equal(t, 0, s.Depth, "space %s", s.Slug)
		require.Equal(t, "/"+s.ID, s.Path, "space %s", s.Slug)
		require.True(t, s.IsActive, "space %s", s.Slug)
		require.Positive(t, s.SortOrder, "space %s", s.Slug)
		require.False(t, sortOrders[s.SortOrder], "duplicate sort order %d", s.SortOrder)
		sortOrders[s.SortOrder] = true
	}

	var tagCount int64
	require.NoError(t, store.SQL().DB().Model(&model.Tag{}).Count(&tagCount).Error)
	require.EqualValues(t, len(seedTagNames), tagCount)
}

func TestSeedBaselineSpacesAndTags_Idempotent(t *testing.T) {
	store := newSeedTestStore(t)
	ctx := context.Background()

	seedBaselineSpaces(store, false)
	seedBaselineTags(store, false)

	// 第二次启动（已 seeded、表非空）走 skip 分支，不重复插入
	seedBaselineSpaces(store, true)
	seedBaselineTags(store, true)

	// 表非空但走强制路径（例如恢复场景）也只补缺失行
	ensureSeedSpaces(ctx, repository.NewSpaceRepository(store))

	var spaceCount, tagCount int64
	require.NoError(t, store.SQL().DB().Model(&model.Space{}).Count(&spaceCount).Error)
	require.NoError(t, store.SQL().DB().Model(&model.Tag{}).Count(&tagCount).Error)
	require.EqualValues(t, len(seedSpaces), spaceCount)
	require.EqualValues(t, len(seedTagNames), tagCount)
}

func TestSeedBaselineSpaces_DoesNotOverwriteAdminEdits(t *testing.T) {
	store := newSeedTestStore(t)
	ctx := context.Background()
	seedBaselineSpaces(store, false)

	repo := repository.NewSpaceRepository(store)
	space, err := repo.FindBySlug(ctx, "tech")
	require.NoError(t, err)
	space.Name = "技术讨论"
	space.Icon = "terminal"
	space.SortOrder = 5
	require.NoError(t, repo.Update(ctx, space))

	// 首次启动路径 + 强制补齐路径都不能覆盖已有版面
	seedBaselineSpaces(store, false)
	ensureSeedSpaces(ctx, repo)

	reloaded, err := repo.FindBySlug(ctx, "tech")
	require.NoError(t, err)
	require.Equal(t, "技术讨论", reloaded.Name)
	require.Equal(t, "terminal", reloaded.Icon)
	require.Equal(t, 5, reloaded.SortOrder)
}

func TestSeedBaselineTags_RestoresDeletedTag(t *testing.T) {
	store := newSeedTestStore(t)
	ctx := context.Background()
	seedBaselineTags(store, false)

	repo := repository.NewTagRepository(store)
	tag, err := repo.GetByName(ctx, "Go")
	require.NoError(t, err)
	require.NoError(t, repo.Delete(ctx, tag.ID))

	// 标签表非空，但缺失的那条会在恢复路径被补回（软删除记录不阻塞唯一索引）
	ensureSeedTags(ctx, repo)

	restored, err := repo.GetByName(ctx, "Go")
	require.NoError(t, err)
	require.Equal(t, "Go", restored.Name)
}

// 暗区是系统版面：首次启动、老站点升级（alreadySeeded=true）、重复调用都必须存在且
// 只有一份，且 system=true / recommendable=false 恒定。
func TestEnsureSeedDarkZone_IdempotentAndFlags(t *testing.T) {
	store := newSeedTestStore(t)
	ctx := context.Background()
	repo := repository.NewSpaceRepository(store)

	first := ensureSeedDarkZone(ctx, repo)
	require.NotNil(t, first)
	require.True(t, first.System)
	require.False(t, first.Recommendable)
	require.Equal(t, darkZoneSlug, first.Slug)

	// 老站点路径：基线 seed 因 alreadySeeded 跳过，暗区仍须被确保
	seedBaselineSpaces(store, true)
	second := ensureSeedDarkZone(ctx, repo)
	require.NotNil(t, second)
	require.Equal(t, first.ID, second.ID, "重复调用不得新建第二份暗区")

	var count int64
	require.NoError(t, store.SQL().DB().Model(&model.Space{}).Where("slug = ?", darkZoneSlug).Count(&count).Error)
	require.EqualValues(t, 1, count)

	// 管理员误改标记也会被校正回系统版面语义
	second.System = false
	second.Recommendable = true
	require.NoError(t, repo.Update(ctx, second))
	fixed := ensureSeedDarkZone(ctx, repo)
	require.NotNil(t, fixed)
	require.True(t, fixed.System)
	require.False(t, fixed.Recommendable)
}
