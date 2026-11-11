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
		// u-low 曾直接设成 TL1，恰好绕开了「已登录但等级 0」这一格——
		// 而所有新注册用户的 TrustLevel 默认就是 0，正是最容易踩到的那格。
		// 现在补一个真正的 TL0 已登录用户。
		{ID: "u-tl0", Username: "tl0", Email: "t@x.com", TrustLevel: 0},
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

	// 已登录但等级 0（= 新注册用户的默认状态）：login 版面 MUST 与「已登录即可」
	// 的语义一致 —— 列表、批量、详情三个出口给出同一答案（2026-11-11 修复前
	// 列表含、详情 403、批量过滤，三种答案）。
	tl0 := WithViewerID(context.Background(), "u-tl0")
	tl0Items, _, err := svc.List(tl0, 1, 50, "", "", "", nil, nil, "", "", "", false, "")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"p-public", "p-login", "p-child"}, ids(tl0Items),
		"TL0 已登录用户应能看到 login 版面的帖子")

	tl0Batch, err := svc.ListByIDs(tl0, []string{"p-public", "p-login", "p-child", "p-level3"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"p-public", "p-login", "p-child"}, ids(tl0Batch),
		"批量出口必须与列表出口一致")

	loginDetail, err := postRepo.FindByIDWithDetails(tl0, "p-login")
	require.NoError(t, err)
	require.NotNil(t, loginDetail)
	require.NoError(t, svc.ensurePostVisible(tl0, *loginDetail),
		"详情出口必须与列表出口一致：login 版面已登录即可读")

	// 未登录用户读 login 版面详情：403 且提示的是「需要登录」而不是「需要等级 1」。
	loginForGuest, err := postRepo.FindByIDWithDetails(anonymous, "p-login")
	require.NoError(t, err)
	require.NotNil(t, loginForGuest)
	guestErr := svc.ensurePostVisible(anonymous, *loginForGuest)
	require.Error(t, guestErr)
	guestAppErr, ok := guestErr.(*pkg.AppError)
	require.True(t, ok)
	assert.Equal(t, pkg.FORBIDDEN, guestAppErr.Code)
	assert.Contains(t, guestAppErr.Message, "需要登录")

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
