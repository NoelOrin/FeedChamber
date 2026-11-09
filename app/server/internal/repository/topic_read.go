package repository

import (
	"context"
	"time"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

// TopicReadRepository persists per-user topic reading progress.
//
// 两路写入必须分开，且都只能 upsert 自己那几列（见 model.TopicReadState 的注释）：
// Upsert = 显式已读，RecordView = 被动浏览。合成一个「全列 upsert」会让
// 后写的一方把先写的游标/读标记静默清零。
type TopicReadRepository interface {
	// Upsert records an explicit "read" mark. An empty lastContentID keeps the
	// existing cursor rather than clobbering it with "no position".
	Upsert(ctx context.Context, userID, postID, lastContentID string, lastReadAt time.Time) error
	// RecordView notes that the user opened the post (and optionally where they
	// got to). It MUST NOT touch last_read_at — viewing is not reading-done.
	RecordView(ctx context.Context, userID, postID, lastContentID string, viewedAt time.Time) error
	FindByUserAndPost(ctx context.Context, userID, postID string) (*model.TopicReadState, error)
	ListByUserAndPosts(ctx context.Context, userID string, postIDs []string) ([]model.TopicReadState, error)
	ListByUser(ctx context.Context, userID string) ([]model.TopicReadState, error)
	// DeleteByUserAndPost is idempotent: deleting a record that is not there
	// succeeds rather than returning ErrNotFound.
	DeleteByUserAndPost(ctx context.Context, userID, postID string) error
	DeleteByUser(ctx context.Context, userID string) error
}

func NewTopicReadRepository(store db.Store) TopicReadRepository {
	return &sqlTopicReadRepo{db: store.SQL()}
}
