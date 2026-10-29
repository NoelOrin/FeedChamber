package repository

import (
	"context"
	"time"

	"orinforum/internal/db"
	"orinforum/internal/model"
)

// TopicReadRepository persists per-user topic reading progress.
type TopicReadRepository interface {
	Upsert(ctx context.Context, userID, postID, lastContentID string, lastReadAt time.Time) error
	FindByUserAndPost(ctx context.Context, userID, postID string) (*model.TopicReadState, error)
	ListByUserAndPosts(ctx context.Context, userID string, postIDs []string) ([]model.TopicReadState, error)
	ListByUser(ctx context.Context, userID string) ([]model.TopicReadState, error)
}

func NewTopicReadRepository(store db.Store) TopicReadRepository {
	return &sqlTopicReadRepo{db: store.SQL()}
}
