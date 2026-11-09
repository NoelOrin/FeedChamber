package service

import (
	"context"
	"testing"
	"time"

	"feedchamber/internal/mock"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnrichWithReadState_NoRecordHasNewFalse(t *testing.T) {
	reads := mock.NewTopicReadRepository()
	svc := &postService{topicReads: reads}
	items := []model.PostListItem{{
		ID:           "p1",
		LastActiveAt: time.Now(),
	}}
	svc.EnrichWithReadState(context.Background(), "u1", items)
	assert.False(t, items[0].HasNew)
	assert.Nil(t, items[0].LastReadAt)
}

func TestEnrichWithReadState_ActiveAfterReadHasNewTrue(t *testing.T) {
	reads := mock.NewTopicReadRepository()
	readAt := time.Now().Add(-time.Hour)
	require.NoError(t, reads.Upsert(context.Background(), "u1", "p1", "c1", readAt))
	svc := &postService{topicReads: reads}
	items := []model.PostListItem{{
		ID:           "p1",
		LastActiveAt: time.Now(),
	}}
	svc.EnrichWithReadState(context.Background(), "u1", items)
	require.True(t, items[0].HasNew)
	require.NotNil(t, items[0].LastReadAt)
	assert.True(t, items[0].LastReadAt.Equal(readAt))
}

func TestEnrichWithReadState_ActiveBeforeOrEqualHasNewFalse(t *testing.T) {
	reads := mock.NewTopicReadRepository()
	readAt := time.Now()
	require.NoError(t, reads.Upsert(context.Background(), "u1", "p1", "c1", readAt))
	svc := &postService{topicReads: reads}

	items := []model.PostListItem{{
		ID:           "p1",
		LastActiveAt: readAt.Add(-time.Minute),
	}}
	svc.EnrichWithReadState(context.Background(), "u1", items)
	assert.False(t, items[0].HasNew)

	items = []model.PostListItem{{
		ID:           "p1",
		LastActiveAt: readAt,
	}}
	svc.EnrichWithReadState(context.Background(), "u1", items)
	assert.False(t, items[0].HasNew)
}

func TestEnrichWithReadState_EmptyUserNoop(t *testing.T) {
	reads := mock.NewTopicReadRepository()
	readAt := time.Now().Add(-time.Hour)
	require.NoError(t, reads.Upsert(context.Background(), "u1", "p1", "c1", readAt))
	svc := &postService{topicReads: reads}
	items := []model.PostListItem{{
		ID:           "p1",
		LastActiveAt: time.Now(),
		HasNew:       false,
	}}
	svc.EnrichWithReadState(context.Background(), "", items)
	assert.False(t, items[0].HasNew)
	assert.Nil(t, items[0].LastReadAt)
}

func TestMarkRead_UpsertAndOverwrite(t *testing.T) {
	posts := mock.NewPostRepository()
	require.NoError(t, posts.Create(context.Background(), &model.Post{
		ID:        "p1",
		ContentID: "c-root",
		AuthorID:  "author",
		Status:    model.PostStatusApproved,
	}))
	reads := mock.NewTopicReadRepository()
	svc := &postService{posts: posts, topicReads: reads}

	require.NoError(t, svc.MarkRead(context.Background(), "u1", "p1"))
	st, err := reads.FindByUserAndPost(context.Background(), "u1", "p1")
	require.NoError(t, err)
	// 标记已读不再 fallback 写主楼 content id：那是「没有位置」而不是「位置在主楼」，
	// 把游标写成主楼会让精确到评论的续读位置永远退化成帖子开头。
	assert.Empty(t, st.LastContentID)
	require.NotNil(t, st.LastReadAt)
	first := *st.LastReadAt

	time.Sleep(2 * time.Millisecond)
	require.NoError(t, svc.MarkRead(context.Background(), "u1", "p1"))
	st2, err := reads.FindByUserAndPost(context.Background(), "u1", "p1")
	require.NoError(t, err)
	require.NotNil(t, st2.LastReadAt)
	assert.True(t, !st2.LastReadAt.Before(first))
}

// 只打开过、从未标记已读的行（被动浏览的产物）：角标必须保持熄灭，
// 但续读游标要能读回给前端定位。
func TestEnrichWithReadState_ViewedOnlyFillsCursorNotBadge(t *testing.T) {
	reads := mock.NewTopicReadRepository()
	viewedAt := time.Now().Add(-time.Hour)
	require.NoError(t, reads.RecordView(context.Background(), "u1", "p1", "c-12", viewedAt))
	svc := &postService{topicReads: reads}

	items := []model.PostListItem{{ID: "p1", LastActiveAt: time.Now()}}
	svc.EnrichWithReadState(context.Background(), "u1", items)

	assert.False(t, items[0].HasNew, "被动浏览不得点亮「有新回复」角标")
	assert.Nil(t, items[0].LastReadAt)
	assert.Equal(t, "c-12", items[0].LastContentID, "游标必须读得回来")
}

// 标记已读时前端没带游标 → 保留原来的评论位置，不该抹成空。
func TestMarkReadToContent_EmptyCursorKeepsExistingCursor(t *testing.T) {
	reads := mock.NewTopicReadRepository()
	require.NoError(t, reads.RecordView(context.Background(), "u1", "p1", "c-12", time.Now().Add(-time.Hour)))
	svc := &postService{topicReads: reads}

	require.NoError(t, svc.MarkReadToContent(context.Background(), "u1", "p1", ""))

	st, err := reads.FindByUserAndPost(context.Background(), "u1", "p1")
	require.NoError(t, err)
	assert.Equal(t, "c-12", st.LastContentID)
	require.NotNil(t, st.LastReadAt)
}

func TestMarkRead_PostNotFound(t *testing.T) {
	posts := mock.NewPostRepository()
	reads := mock.NewTopicReadRepository()
	svc := &postService{posts: posts, topicReads: reads}
	err := svc.MarkRead(context.Background(), "u1", "missing")
	require.Error(t, err)
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.NOT_FOUND, appErr.Code)
}
