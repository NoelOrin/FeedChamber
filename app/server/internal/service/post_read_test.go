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
	assert.Equal(t, "c-root", st.LastContentID)
	first := st.LastReadAt

	time.Sleep(2 * time.Millisecond)
	require.NoError(t, svc.MarkRead(context.Background(), "u1", "p1"))
	st2, err := reads.FindByUserAndPost(context.Background(), "u1", "p1")
	require.NoError(t, err)
	assert.True(t, !st2.LastReadAt.Before(first))
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
