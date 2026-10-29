package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"time"

	"gorm.io/gorm/clause"
)

type sqlTopicReadRepo struct{ db db.SQLStore }

func (r *sqlTopicReadRepo) Upsert(ctx context.Context, userID, postID, lastContentID string, lastReadAt time.Time) error {
	if lastReadAt.IsZero() {
		lastReadAt = time.Now()
	}
	state := model.TopicReadState{
		ID:            pkg.NewID(),
		UserID:        userID,
		PostID:        postID,
		LastReadAt:    lastReadAt,
		LastContentID: lastContentID,
	}
	return r.db.DB().WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "post_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"last_read_at", "last_content_id"}),
		}).
		Create(&state).Error
}

func (r *sqlTopicReadRepo) FindByUserAndPost(ctx context.Context, userID, postID string) (*model.TopicReadState, error) {
	var state model.TopicReadState
	err := r.db.DB().WithContext(ctx).
		Where("user_id = ? AND post_id = ?", userID, postID).
		First(&state).Error
	if err != nil {
		return nil, WrapDBErr(err)
	}
	return &state, nil
}

func (r *sqlTopicReadRepo) ListByUserAndPosts(ctx context.Context, userID string, postIDs []string) ([]model.TopicReadState, error) {
	if len(postIDs) == 0 {
		return []model.TopicReadState{}, nil
	}
	var states []model.TopicReadState
	err := r.db.DB().WithContext(ctx).
		Where("user_id = ? AND post_id IN ?", userID, postIDs).
		Find(&states).Error
	return states, WrapDBErr(err)
}

func (r *sqlTopicReadRepo) ListByUser(ctx context.Context, userID string) ([]model.TopicReadState, error) {
	var states []model.TopicReadState
	err := r.db.DB().WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&states).Error
	return states, WrapDBErr(err)
}
