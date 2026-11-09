package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/testnats"
	"feedchamber/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// historyTestEnv 把浏览记录/续读游标跑在真实 sqlite 上：这两条出口的要点
// （可见性谓词、COALESCE 排序、软删过滤、ON CONFLICT 列裁剪）全都发生在 SQL 里，
// 用内存替身测等于测替身。
type historyTestEnv struct {
	store      db.Store
	svc        *postService
	topicReads repository.TopicReadRepository
	contents   repository.ContentRepository
}

func newHistoryTestEnv(t *testing.T) *historyTestEnv {
	t.Helper()
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "history.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(
		&model.User{}, &model.Post{}, &model.Content{}, &model.Space{}, &model.TopicReadState{},
	))

	spaces := []model.Space{
		{ID: "sp-public", Name: "public", Slug: "public", Visibility: model.SpaceVisibilityPublic, IsActive: true},
		{ID: "sp-level3", Name: "level3", Slug: "level3", Visibility: model.SpaceVisibilityLevel, MinLevel: 3, IsActive: true},
	}
	for i := range spaces {
		require.NoError(t, store.SQL().DB().Create(&spaces[i]).Error)
	}
	users := []*model.User{
		{ID: "u-low", Username: "low", Email: "l@x.com", TrustLevel: 1},
		{ID: "u-high", Username: "high", Email: "h@x.com", TrustLevel: 3},
	}
	for _, u := range users {
		require.NoError(t, store.SQL().DB().Create(u).Error)
	}

	mkPost := func(id, spaceID string, deleted bool) {
		p := &model.Post{
			ID: id, Title: id, Slug: id, ContentID: "c-" + id,
			AuthorID: "u-high", Status: model.PostStatusApproved, SpaceID: spaceID,
		}
		require.NoError(t, store.SQL().DB().Create(p).Error)
		// 主楼 content：评论的 root_id 指向它，游标归属校验据此判定
		require.NoError(t, store.SQL().DB().Create(&model.Content{
			ID: "c-" + id, AuthorID: "u-high", Body: "root", RootID: "c-" + id, Status: "approved",
		}).Error)
		if deleted {
			require.NoError(t, store.SQL().DB().Delete(&model.Post{}, "id = ?", id).Error)
		}
	}
	mkPost("p-public", "sp-public", false)
	mkPost("p-old", "sp-public", false)
	mkPost("p-level3", "sp-level3", false)
	mkPost("p-gone", "sp-public", true)

	// 被拒稿的帖子：非作者非审核者读不到它，历史里也不该出现（点进去必然 404，
	// 且等于向访问者泄露「这篇存在过」）。
	require.NoError(t, store.SQL().DB().Create(&model.Post{
		ID: "p-rejected", Title: "p-rejected", Slug: "p-rejected", ContentID: "c-p-rejected",
		AuthorID: "u-high", Status: model.PostStatusRejected, SpaceID: "sp-public",
	}).Error)

	postRepo := repository.NewPostRepository(store)
	contentRepo := repository.NewContentRepository(store)
	readRepo := repository.NewTopicReadRepository(store)
	svc := &postService{
		posts:      postRepo,
		contents:   contentRepo,
		topicReads: readRepo,
		spaceRepo:  repository.NewSpaceRepository(store),
		users:      repository.NewUserRepository(store),
		// GetDetail 会走 view-count 缓冲，kv 为空会当场 panic
		kv: testnats.NewKVStore(t, "history_"+t.Name()),
	}
	return &historyTestEnv{store: store, svc: svc, topicReads: readRepo, contents: contentRepo}
}

func idsOf(items []model.PostListItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

// 历史列表是第 7 个帖子读取出口：必须与列表/详情共用可见性谓词。
// 在 Go 里「取一页再筛掉看不见的」会让 total 与页码说谎，也会泄露受限帖的存在性。
func TestHistory_ListOnlyShowsVisiblePosts(t *testing.T) {
	env := newHistoryTestEnv(t)
	ctx := context.Background()
	low := WithViewerID(ctx, "u-low")
	high := WithViewerID(ctx, "u-high")

	for _, uid := range []string{"u-low", "u-high"} {
		for _, pid := range []string{"p-public", "p-level3", "p-gone", "p-rejected"} {
			require.NoError(t, env.topicReads.RecordView(ctx, uid, pid, "", time.Now()))
		}
	}

	items, total, err := env.svc.ListHistory(low, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, []string{"p-public"}, idsOf(items))
	assert.Equal(t, 1, total, "不可见的帖子不得计入 total，否则分页条会给出翻不到的页")

	hItems, hTotal, err := env.svc.ListHistory(high, 1, 20)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"p-public", "p-level3"}, idsOf(hItems))
	assert.Equal(t, 2, hTotal)
}

// 历史列表必须把三个读状态字段一起带回来：游标是「点进去回到哪条评论」的
// 唯一凭据，时间与读标记用于在列表里显示「何时看过 / 标过已读没」。
// 只写不读正是这个模块之前的毛病，这里把回读链路钉死。
func TestHistory_ListReturnsReadStateFields(t *testing.T) {
	env := newHistoryTestEnv(t)
	ctx := context.Background()
	viewed := time.Now().Add(-time.Hour)
	high := WithViewerID(ctx, "u-high")

	require.NoError(t, env.topicReads.RecordView(ctx, "u-high", "p-public", "c-p-public", viewed))

	items, _, err := env.svc.ListHistory(high, 1, 20)
	require.NoError(t, err)
	require.Len(t, items, 1)

	assert.Equal(t, "c-p-public", items[0].LastContentID, "续读游标必须回传")
	require.NotNil(t, items[0].LastViewedAt, "浏览时间必须回传")
	assert.Nil(t, items[0].LastReadAt, "没标记已读的行不应凭空多出读标记")
}

// 排序键是 COALESCE(last_viewed_at, last_read_at)：只有读标记的存量行
// （升级前 markRead 写的）不能从历史里消失。
func TestHistory_ListOrdersByMostRecentTouch(t *testing.T) {
	env := newHistoryTestEnv(t)
	ctx := context.Background()
	now := time.Now()
	low := WithViewerID(ctx, "u-low")

	require.NoError(t, env.topicReads.RecordView(ctx, "u-low", "p-public", "", now.Add(-30*time.Minute)))
	// 旧数据：只有读标记、没有浏览时间（升级前 markRead 留下的行）
	require.NoError(t, env.topicReads.Upsert(ctx, "u-low", "p-old", "", now.Add(-10*time.Minute)))
	// 最近浏览的那条已被删除，必须整个从历史里消失
	require.NoError(t, env.topicReads.RecordView(ctx, "u-low", "p-gone", "", now.Add(-5*time.Minute)))

	items, _, err := env.svc.ListHistory(low, 1, 20)
	require.NoError(t, err)
	require.Equal(t, []string{"p-old", "p-public"}, idsOf(items),
		"COALESCE 排序：10 分钟前标读的旧行应排在 30 分钟前浏览的行之前")
}

// 浏览上报只写浏览时间与游标，绝不写读标记；不可见帖不落记录；
// 别人的评论 id 不能盲写进来。
func TestRecordView_RecordsBrowseWithoutReadMark(t *testing.T) {
	env := newHistoryTestEnv(t)
	ctx := context.Background()
	low := WithViewerID(ctx, "u-low")

	require.NoError(t, env.svc.RecordView(low, "p-public", "c-p-public"))

	st, err := env.topicReads.FindByUserAndPost(ctx, "u-low", "p-public")
	require.NoError(t, err)
	require.NotNil(t, st.LastViewedAt)
	assert.Nil(t, st.LastReadAt, "看过 ≠ 读完，不得写读标记")
	assert.Equal(t, "c-p-public", st.LastContentID)
}

func TestRecordView_RejectsForeignCursor(t *testing.T) {
	env := newHistoryTestEnv(t)
	ctx := context.Background()
	low := WithViewerID(ctx, "u-low")

	// 另一篇帖子的评论：不属于本帖的 content 一律拒绝
	err := env.svc.RecordView(low, "p-public", "c-p-level3")
	require.Error(t, err)
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.NOT_FOUND, appErr.Code)

	_, err = env.topicReads.FindByUserAndPost(ctx, "u-low", "p-public")
	require.ErrorIs(t, err, repository.ErrNotFound, "被拒绝的上报不得留下记录")
}

func TestRecordView_SkipsInvisiblePost(t *testing.T) {
	env := newHistoryTestEnv(t)
	low := WithViewerID(context.Background(), "u-low")

	err := env.svc.RecordView(low, "p-level3", "")
	require.Error(t, err)
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.FORBIDDEN, appErr.Code)

	_, err = env.topicReads.FindByUserAndPost(context.Background(), "u-low", "p-level3")
	require.ErrorIs(t, err, repository.ErrNotFound)
}

func TestHistory_DeleteAndClearAreIdempotentAndScoped(t *testing.T) {
	env := newHistoryTestEnv(t)
	ctx := context.Background()
	now := time.Now()
	low := WithViewerID(ctx, "u-low")

	require.NoError(t, env.topicReads.RecordView(ctx, "u-low", "p-public", "c-a", now))
	require.NoError(t, env.topicReads.RecordView(ctx, "u-low", "p-level3", "c-b", now))
	require.NoError(t, env.topicReads.RecordView(ctx, "u-high", "p-public", "c-c", now))

	require.NoError(t, env.svc.DeleteHistory(low, "p-public"))
	_, err := env.topicReads.FindByUserAndPost(ctx, "u-low", "p-public")
	require.ErrorIs(t, err, repository.ErrNotFound)

	// 幂等：再删一次同样成功，不是 404
	require.NoError(t, env.svc.DeleteHistory(low, "p-public"))

	require.NoError(t, env.svc.ClearHistory(low))
	items, _, err := env.svc.ListHistory(low, 1, 20)
	require.NoError(t, err)
	assert.Empty(t, items)

	// 清空只影响自己：u-high 的那条还在
	other, err := env.topicReads.FindByUserAndPost(ctx, "u-high", "p-public")
	require.NoError(t, err)
	assert.Equal(t, "c-c", other.LastContentID)
}

// 浏览记录必须自动产生：只靠前端「标记已读」按钮那叫书签，不叫浏览记录。
// GetDetail 与 GetByID 是两个公开的帖子详情出口（/posts/{id}/detail 与
// /posts/{id}），都要记，漏一个就会让走另一条入口的页面读不到自己的历史。
func TestGetDetail_RecordsBrowseForSignedInViewer(t *testing.T) {
	env := newHistoryTestEnv(t)
	ctx := context.Background()

	_, err := env.svc.GetDetail(WithViewerID(ctx, "u-low"), "p-public")
	require.NoError(t, err)

	st, err := env.topicReads.FindByUserAndPost(ctx, "u-low", "p-public")
	require.NoError(t, err, "打开详情即应留下浏览记录")
	require.NotNil(t, st.LastViewedAt)
	assert.Nil(t, st.LastReadAt, "自动记录不得写成已读")
}

func TestGetByID_AlsoRecordsBrowse(t *testing.T) {
	env := newHistoryTestEnv(t)
	ctx := context.Background()

	_, err := env.svc.GetByID(WithViewerID(ctx, "u-low"), "p-public")
	require.NoError(t, err)

	st, err := env.topicReads.FindByUserAndPost(ctx, "u-low", "p-public")
	require.NoError(t, err, "第二个详情出口同样要留下浏览记录")
	require.NotNil(t, st.LastViewedAt)
	assert.Nil(t, st.LastReadAt)
}

// 匿名访问没有归属主体，不记；不可见帖同样不记（门槛在记录之前就失败）。
func TestGetDetail_DoesNotRecordAnonymousOrInvisible(t *testing.T) {
	env := newHistoryTestEnv(t)
	ctx := context.Background()

	_, err := env.svc.GetDetail(ctx, "p-public")
	require.NoError(t, err)
	_, err = env.topicReads.FindByUserAndPost(ctx, "u-low", "p-public")
	require.ErrorIs(t, err, repository.ErrNotFound, "匿名访问不得写任何人的浏览记录")

	_, err = env.svc.GetDetail(WithViewerID(ctx, "u-low"), "p-level3")
	require.Error(t, err)
	_, err = env.topicReads.FindByUserAndPost(ctx, "u-low", "p-level3")
	require.ErrorIs(t, err, repository.ErrNotFound, "被 403 挡下的访问不得留下记录")
}
