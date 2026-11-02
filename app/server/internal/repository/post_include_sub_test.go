package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/model"

	"github.com/stretchr/testify/require"
)

// newSpaceTreeRepo 建一个带版面临时表的 SQLite store，用来验证「父版聚合子版」的 SQL 路径。
func newSpaceTreeRepo(t *testing.T) (PostRepository, SpaceRepository, db.Store) {
	t.Helper()
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "spaces.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(&model.Post{}, &model.User{}, &model.Space{}))
	return NewPostRepository(store), NewSpaceRepository(store), store
}

// Path 形状由 service.spaceService.Create 维护：根版 "/<id>"，子版 "<parentPath>/<id>"。
func TestPostList_IncludeSubFilterMatchesDescendants(t *testing.T) {
	posts, spaces, store := newSpaceTreeRepo(t)
	ctx := context.Background()

	tree := []model.Space{
		{ID: "parent", Name: "父版", Slug: "parent", Path: "/parent", IsActive: true, Recommendable: true},
		{ID: "child", Name: "子版", Slug: "child", ParentID: "parent", Depth: 1, Path: "/parent/child", IsActive: true, Recommendable: true},
		{ID: "grand", Name: "孙版", Slug: "grand", ParentID: "child", Depth: 2, Path: "/parent/child/grand", IsActive: true, Recommendable: true},
		{ID: "other", Name: "别的版", Slug: "other", Path: "/other", IsActive: true, Recommendable: true},
	}
	for i := range tree {
		require.NoError(t, store.SQL().DB().Create(&tree[i]).Error)
	}

	now := time.Now()
	for _, spaceID := range []string{"parent", "child", "grand", "other"} {
		require.NoError(t, store.SQL().DB().Create(&model.Post{
			ID: "post-" + spaceID, Title: spaceID, Slug: spaceID, AuthorID: "u1",
			ContentID: "c-" + spaceID, SpaceID: spaceID, Status: model.PostStatusApproved, CreatedAt: now, LastActiveAt: now,
		}).Error)
	}

	descendants, err := spaces.FindDescendantIDs(ctx, "parent")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"child", "grand"}, descendants)

	ids := append([]string{"parent"}, descendants...)
	items, total, err := posts.ListWithTotal(ctx, PostListFilter{SpaceIDs: ids}, 0, 50)
	require.NoError(t, err)
	require.Equal(t, 3, total, "父版 feeds 必须包含孙子版帖子，且不得跨到兄弟版")
	require.Len(t, items, 3)

	// 不聚合时仍只看本版：space_id 单值过滤语义不能被改坏。
	_, exactTotal, err := posts.ListWithTotal(ctx, PostListFilter{SpaceID: "parent"}, 0, 50)
	require.NoError(t, err)
	require.Equal(t, 1, exactTotal)
}
