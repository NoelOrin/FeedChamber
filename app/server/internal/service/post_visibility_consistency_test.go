package service

import (
	"context"
	"path/filepath"
	"testing"

	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 可见性一致性矩阵：同一批帖子在六个内容出口（列表 / 详情 / 搜索 / sitemap /
// RSS / 批量）必须得到一致结论。RSS 与 sitemap 共用 SitemapEntries，故同源。
func TestVisibility_AllOutletsAgree(t *testing.T) {
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "vis.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(
		&model.User{}, &model.Post{}, &model.Content{}, &model.Space{}, &model.Tag{}, &model.PostTag{},
	))

	// 版面：公开 / 需登录 / 需等级 3 / 「需登录」版面的公开子版（继承）
	spaces := []model.Space{
		{ID: "sp-public", Name: "public", Slug: "public", Visibility: model.SpaceVisibilityPublic, IsActive: true},
		{ID: "sp-login", Name: "login", Slug: "login", Visibility: model.SpaceVisibilityLogin, IsActive: true},
		{ID: "sp-level3", Name: "level3", Slug: "level3", Visibility: model.SpaceVisibilityLevel, MinLevel: 3, IsActive: true},
		{ID: "sp-child", Name: "child", Slug: "child", ParentID: "sp-login", Visibility: model.SpaceVisibilityPublic, IsActive: true},
	}
	for i := range spaces {
		require.NoError(t, store.SQL().DB().Create(&spaces[i]).Error)
	}
	for _, u := range []*model.User{
		{ID: "u-author", Username: "author", Email: "a@x.com", TrustLevel: 2},
		{ID: "u-low", Username: "low", Email: "l@x.com", TrustLevel: 1},
		{ID: "u-high", Username: "high", Email: "h@x.com", TrustLevel: 3},
	} {
		require.NoError(t, store.SQL().DB().Create(u).Error)
	}

	mk := func(id, spaceID string, minLevel int) {
		p := &model.Post{
			ID: id, Title: id, Slug: id, ContentID: "c-" + id, AuthorID: "u-author",
			Status: model.PostStatusApproved, SpaceID: spaceID, MinLevel: minLevel,
		}
		require.NoError(t, store.SQL().DB().Create(p).Error)
	}
	mk("p-public", "sp-public", 0)
	mk("p-login", "sp-login", 0)
	mk("p-child", "sp-child", 0)
	mk("p-level3", "sp-level3", 0)
	mk("p-override", "sp-public", 2)

	// 作者自己的受限帖：作者例外
	require.NoError(t, store.SQL().DB().Create(&model.Post{
		ID: "p-author-own", Title: "own", Slug: "own", ContentID: "c-own", AuthorID: "u-low",
		Status: model.PostStatusApproved, SpaceID: "sp-level3",
	}).Error)

	postRepo := repository.NewPostRepository(store)
	svc := &postService{
		posts:     postRepo,
		spaceRepo: repository.NewSpaceRepository(store),
		users:     repository.NewUserRepository(store),
	}

	ids := func(items []model.PostListItem) []string {
		out := make([]string, 0, len(items))
		for _, it := range items {
			out = append(out, it.ID)
		}
		return out
	}

	anonymous := context.Background()
	low := WithViewerID(context.Background(), "u-low")
	high := WithViewerID(context.Background(), "u-high")

	// 列表（匿名）：只有公开版面的、min_level=0 的帖子。
	items, total, err := svc.List(anonymous, 1, 50, "", "", "", nil, nil, "", "", "", false, "")
	require.NoError(t, err)
	assert.Equal(t, len(items), total)
	assert.ElementsMatch(t, []string{"p-public"}, ids(items))

	// 搜索：与列表一致。
	hits, _, err := svc.Search(anonymous, "p-", 1, 50)
	require.NoError(t, err)
	assert.ElementsMatch(t, ids(items), ids(hits))

	// sitemap / RSS：匿名公开语料，与列表一致（RSS 复用同一投影）。
	entries, err := svc.SitemapEntries(anonymous, 1, 500)
	require.NoError(t, err)
	sitemapIDs := make([]string, 0, len(entries))
	for _, e := range entries {
		sitemapIDs = append(sitemapIDs, e.ID)
	}
	assert.ElementsMatch(t, ids(items), sitemapIDs)
	count, err := svc.CountSitemapEntries(anonymous)
	require.NoError(t, err)
	assert.Equal(t, len(sitemapIDs), count)

	// 批量：匿名与列表一致。
	batch, err := svc.ListByIDs(anonymous, []string{"p-public", "p-login", "p-level3", "p-override"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"p-public"}, ids(batch))

	// 等级 1：可读公开 + 已登录可读 login（含继承子版），不可读 level3 与 min_level=2 覆盖帖。
	lowItems, _, err := svc.List(low, 1, 50, "", "", "", nil, nil, "", "", "", false, "")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"p-public", "p-login", "p-child", "p-author-own"}, ids(lowItems))

	// 等级 3：可读全部。
	highItems, _, err := svc.List(high, 1, 50, "", "", "", nil, nil, "", "", "", false, "")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"p-public", "p-login", "p-child", "p-level3", "p-override", "p-author-own"}, ids(highItems))

	// 作者例外：等级 1 作者能读自己在 level3 的帖子。
	ownBatch, err := svc.ListByIDs(low, []string{"p-author-own", "p-level3"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"p-author-own"}, ids(ownBatch))

	// 详情：等级 1 读 level3 → 403 且带需要/当前等级；作者 → 放行。
	detailItem, err := postRepo.FindByIDWithDetails(anonymous, "p-level3")
	require.NoError(t, err)
	require.NotNil(t, detailItem)
	err = svc.ensurePostVisible(low, *detailItem)
	require.Error(t, err)
	appErr, ok := err.(*pkg.AppError)
	require.True(t, ok, "detail gate must return AppError")
	assert.Equal(t, pkg.FORBIDDEN, appErr.Code)
	assert.Contains(t, appErr.Message, "需要等级 3")
	assert.Contains(t, appErr.Message, "当前等级 1")

	ownDetail, err := postRepo.FindByIDWithDetails(anonymous, "p-author-own")
	require.NoError(t, err)
	require.NoError(t, svc.ensurePostVisible(low, *ownDetail))
}

// 继承解析的单元契约：最严格一档胜出，level 取最大门槛。
func TestBuildVisibilityIndex_Inheritance(t *testing.T) {
	idx := BuildVisibilityIndex([]model.Space{
		{ID: "root", Visibility: model.SpaceVisibilityLogin},
		{ID: "child", ParentID: "root", Visibility: model.SpaceVisibilityPublic},
		{ID: "grand", ParentID: "child", Visibility: model.SpaceVisibilityLevel, MinLevel: 2},
		{ID: "root-l3", Visibility: model.SpaceVisibilityLevel, MinLevel: 3},
		{ID: "deep", ParentID: "root-l3", Visibility: model.SpaceVisibilityLevel, MinLevel: 1},
	})
	assert.Equal(t, model.SpaceVisibilityLogin, idx.Resolve("child").Visibility)
	assert.Equal(t, 3, idx.Resolve("deep").MinLevel, "level 门槛取祖先链最大值")
	assert.False(t, idx.Resolve("root").Allows(3, false), "login 版面游客不可读")
	assert.True(t, idx.Resolve("root").Allows(0, true), "login 版面登录用户可读")
	assert.Equal(t, 1, idx.Resolve("root").RequiredLevel())
	assert.True(t, idx.Resolve("grand").Allows(2, false))
	assert.False(t, idx.Resolve("grand").Allows(1, true))
}
