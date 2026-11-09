package repository

import (
	"context"
	"testing"
	"time"

	"feedchamber/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTopicReadTestRepo(t *testing.T) *sqlTopicReadRepo {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&model.TopicReadState{}))
	return &sqlTopicReadRepo{db: &mockSQLStore{db: gdb}}
}

// 浏览记录是被动落的：打开详情只说明「看过」，不说明「读完」。
// 若它把 last_read_at 一起写上，列表的「有新回复」角标与「没读过」筛选会当场失真。
func TestTopicRead_RecordViewWritesViewedAtOnly(t *testing.T) {
	repo := newTopicReadTestRepo(t)
	ctx := context.Background()
	viewed := time.Now()

	require.NoError(t, repo.RecordView(ctx, "u1", "p1", "c-12", viewed))

	st, err := repo.FindByUserAndPost(ctx, "u1", "p1")
	require.NoError(t, err)
	require.NotNil(t, st.LastViewedAt)
	assert.True(t, st.LastViewedAt.Equal(viewed.Truncate(time.Nanosecond)) || !st.LastViewedAt.Before(viewed.Add(-time.Second)))
	assert.Nil(t, st.LastReadAt, "被动浏览不得写已读标记")
	assert.Equal(t, "c-12", st.LastContentID)
}

// 前端只在评论可见性变化时上报游标；不带游标的上报（打开首页、旧客户端）
// 只该刷新时间，不该把「读到第 12 条」抹成「没有位置」。
func TestTopicRead_RecordViewWithEmptyCursorKeepsCursor(t *testing.T) {
	repo := newTopicReadTestRepo(t)
	ctx := context.Background()
	first := time.Now().Add(-time.Hour)

	require.NoError(t, repo.RecordView(ctx, "u1", "p1", "c-12", first))
	time.Sleep(2 * time.Millisecond)
	require.NoError(t, repo.RecordView(ctx, "u1", "p1", "", time.Now()))

	st, err := repo.FindByUserAndPost(ctx, "u1", "p1")
	require.NoError(t, err)
	assert.Equal(t, "c-12", st.LastContentID, "空游标上报必须保留旧游标")
	assert.True(t, st.LastViewedAt.After(first))
}

// 同一行的两路写入必须互不覆盖：标记已读不该清掉浏览时间，
// 浏览上报也不该改掉已读标记。
func TestTopicRead_TwoWritePathsDoNotClobberEachOther(t *testing.T) {
	repo := newTopicReadTestRepo(t)
	ctx := context.Background()
	viewed := time.Now().Add(-time.Hour)
	readAt := time.Now()

	require.NoError(t, repo.RecordView(ctx, "u1", "p1", "c-12", viewed))
	require.NoError(t, repo.Upsert(ctx, "u1", "p1", "", readAt))

	st, err := repo.FindByUserAndPost(ctx, "u1", "p1")
	require.NoError(t, err)
	require.NotNil(t, st.LastReadAt, "标记已读必须写读标记")
	assert.True(t, st.LastReadAt.Equal(readAt))
	require.NotNil(t, st.LastViewedAt, "标记已读不得清掉浏览时间")
	assert.True(t, st.LastViewedAt.Before(readAt))

	// 再来一次被动浏览：读标记与游标都得原样留着
	time.Sleep(2 * time.Millisecond)
	require.NoError(t, repo.RecordView(ctx, "u1", "p1", "", time.Now()))
	st, err = repo.FindByUserAndPost(ctx, "u1", "p1")
	require.NoError(t, err)
	assert.True(t, st.LastReadAt.Equal(readAt))
	assert.Equal(t, "c-12", st.LastContentID, "标记已读传空游标时保留原游标")
}

func TestTopicRead_Delete(t *testing.T) {
	repo := newTopicReadTestRepo(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, repo.RecordView(ctx, "u1", "p1", "c-1", now))
	require.NoError(t, repo.RecordView(ctx, "u1", "p2", "c-2", now))
	require.NoError(t, repo.RecordView(ctx, "u2", "p1", "c-3", now))

	require.NoError(t, repo.DeleteByUserAndPost(ctx, "u1", "p1"))
	_, err := repo.FindByUserAndPost(ctx, "u1", "p1")
	require.ErrorIs(t, err, ErrNotFound)

	// 删不存在的记录是幂等成功，不是 404
	require.NoError(t, repo.DeleteByUserAndPost(ctx, "u1", "p1"))

	require.NoError(t, repo.DeleteByUser(ctx, "u1"))
	other, err := repo.FindByUserAndPost(ctx, "u2", "p1")
	require.NoError(t, err, "清空只影响调用者本人")
	assert.Equal(t, "c-3", other.LastContentID)
}
