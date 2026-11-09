package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type sqlTopicReadRepo struct{ db db.SQLStore }

// topicReadConflict 是 (user_id, post_id) 的 upsert 冲突目标，两路写入共用。
func topicReadConflict(columns []string) clause.OnConflict {
	return clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "post_id"}},
		DoUpdates: clause.AssignmentColumns(columns),
	}
}

// Upsert 写「显式已读」：只碰 last_read_at 与（给了值时的）游标，
// 不动 last_viewed_at；空游标保留旧值，避免点按钮把阅读位置抹成「没有位置」。
func (r *sqlTopicReadRepo) Upsert(ctx context.Context, userID, postID, lastContentID string, lastReadAt time.Time) error {
	if lastReadAt.IsZero() {
		lastReadAt = time.Now()
	}
	state := model.TopicReadState{
		ID:     pkg.NewID(),
		UserID: userID,
		PostID: postID,
	}
	columns := []string{"last_read_at"}
	state.LastReadAt = &lastReadAt
	if lastContentID != "" {
		state.LastContentID = lastContentID
		columns = append(columns, "last_content_id")
	}
	return r.db.DB().WithContext(ctx).
		Clauses(topicReadConflict(columns)).
		Create(&state).Error
}

// RecordView 写「被动浏览」：只碰 last_viewed_at 与（给了值时的）游标，
// MUST NOT 写 last_read_at——看过 ≠ 读完。
func (r *sqlTopicReadRepo) RecordView(ctx context.Context, userID, postID, lastContentID string, viewedAt time.Time) error {
	if viewedAt.IsZero() {
		viewedAt = time.Now()
	}
	state := model.TopicReadState{
		ID:     pkg.NewID(),
		UserID: userID,
		PostID: postID,
	}
	columns := []string{"last_viewed_at"}
	state.LastViewedAt = &viewedAt
	if lastContentID != "" {
		state.LastContentID = lastContentID
		columns = append(columns, "last_content_id")
	}
	return r.db.DB().WithContext(ctx).
		Clauses(topicReadConflict(columns)).
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

// DeleteByUserAndPost 幂等：没有这一行也算成功（用户视角「已删除」已经成立）。
func (r *sqlTopicReadRepo) DeleteByUserAndPost(ctx context.Context, userID, postID string) error {
	res := r.db.DB().WithContext(ctx).
		Where("user_id = ? AND post_id = ?", userID, postID).
		Delete(&model.TopicReadState{})
	if res.Error != nil && res.Error != gorm.ErrRecordNotFound {
		return WrapDBErr(res.Error)
	}
	return nil
}

// DeleteByUser 只清调用者自己的记录，作用域写死在 WHERE 里。
func (r *sqlTopicReadRepo) DeleteByUser(ctx context.Context, userID string) error {
	res := r.db.DB().WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&model.TopicReadState{})
	if res.Error != nil && res.Error != gorm.ErrRecordNotFound {
		return WrapDBErr(res.Error)
	}
	return nil
}
