package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type LikeRepository interface {
	TogglePostLike(ctx context.Context, postID, userID string) (bool, error)
	ToggleCommentLike(ctx context.Context, commentID, userID string) (bool, error)
	HasPostLiked(ctx context.Context, postID, userID string) (bool, error)
	HasCommentLiked(ctx context.Context, commentID, userID string) (bool, error)
	CountPostLikes(ctx context.Context, postID string) (int64, error)
	CountCommentLikes(ctx context.Context, commentID string) (int64, error)
}

func NewLikeRepository(store db.Store) LikeRepository {
	if store.SQL() != nil {
		return &sqlLikeRepo{db: store.SQL()}
	}
	return &mongoLikeRepo{store: store.Mongo()}
}

// ── SQL ──────────────────────────────────────────────────────────────────────

type sqlLikeRepo struct{ db db.SQLStore }

func (r *sqlLikeRepo) TogglePostLike(ctx context.Context, postID, userID string) (bool, error) {
	var like model.PostLike
	err := r.db.DB().WithContext(ctx).Where("post_id = ? AND user_id = ?", postID, userID).First(&like).Error
	if err == nil {
		// exists, delete it
		r.db.DB().WithContext(ctx).Delete(&like)
		return false, nil
	}
	// not exists, create
	like = model.PostLike{ID: uuid.NewString(), PostID: postID, UserID: userID, CreatedAt: time.Now()}
	if err := r.db.DB().WithContext(ctx).Create(&like).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (r *sqlLikeRepo) ToggleCommentLike(ctx context.Context, commentID, userID string) (bool, error) {
	var like model.CommentLike
	err := r.db.DB().WithContext(ctx).Where("comment_id = ? AND user_id = ?", commentID, userID).First(&like).Error
	if err == nil {
		// exists, delete it
		r.db.DB().WithContext(ctx).Delete(&like)
		return false, nil
	}
	// not exists, create
	like = model.CommentLike{ID: uuid.NewString(), CommentID: commentID, UserID: userID, CreatedAt: time.Now()}
	if err := r.db.DB().WithContext(ctx).Create(&like).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (r *sqlLikeRepo) HasPostLiked(ctx context.Context, postID, userID string) (bool, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.PostLike{}).Where("post_id = ? AND user_id = ?", postID, userID).Count(&count).Error
	return count > 0, err
}

func (r *sqlLikeRepo) HasCommentLiked(ctx context.Context, commentID, userID string) (bool, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.CommentLike{}).Where("comment_id = ? AND user_id = ?", commentID, userID).Count(&count).Error
	return count > 0, err
}

func (r *sqlLikeRepo) CountPostLikes(ctx context.Context, postID string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.PostLike{}).Where("post_id = ?", postID).Count(&count).Error
	return count, err
}

func (r *sqlLikeRepo) CountCommentLikes(ctx context.Context, commentID string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.CommentLike{}).Where("comment_id = ?", commentID).Count(&count).Error
	return count, err
}

// ── MongoDB ──────────────────────────────────────────────────────────────────

type mongoLikeRepo struct{ store db.MongoStore }

func (r *mongoLikeRepo) TogglePostLike(ctx context.Context, postID, userID string) (bool, error) {
	filter := bson.M{"post_id": postID, "user_id": userID}
	res, err := r.store.Collection("post_likes").DeleteOne(ctx, filter)
	if err != nil {
		return false, err
	}
	if res.DeletedCount > 0 {
		return false, nil
	}
	// not found, create
	like := model.PostLike{ID: uuid.NewString(), PostID: postID, UserID: userID, CreatedAt: time.Now()}
	_, err = r.store.Collection("post_likes").InsertOne(ctx, like)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *mongoLikeRepo) ToggleCommentLike(ctx context.Context, commentID, userID string) (bool, error) {
	filter := bson.M{"comment_id": commentID, "user_id": userID}
	res, err := r.store.Collection("comment_likes").DeleteOne(ctx, filter)
	if err != nil {
		return false, err
	}
	if res.DeletedCount > 0 {
		return false, nil
	}
	// not found, create
	like := model.CommentLike{ID: uuid.NewString(), CommentID: commentID, UserID: userID, CreatedAt: time.Now()}
	_, err = r.store.Collection("comment_likes").InsertOne(ctx, like)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *mongoLikeRepo) HasPostLiked(ctx context.Context, postID, userID string) (bool, error) {
	count, err := r.store.Collection("post_likes").CountDocuments(ctx, bson.M{"post_id": postID, "user_id": userID})
	return count > 0, err
}

func (r *mongoLikeRepo) HasCommentLiked(ctx context.Context, commentID, userID string) (bool, error) {
	count, err := r.store.Collection("comment_likes").CountDocuments(ctx, bson.M{"comment_id": commentID, "user_id": userID})
	return count > 0, err
}

func (r *mongoLikeRepo) CountPostLikes(ctx context.Context, postID string) (int64, error) {
	count, err := r.store.Collection("post_likes").CountDocuments(ctx, bson.M{"post_id": postID})
	return count, err
}

func (r *mongoLikeRepo) CountCommentLikes(ctx context.Context, commentID string) (int64, error) {
	count, err := r.store.Collection("comment_likes").CountDocuments(ctx, bson.M{"comment_id": commentID})
	return count, err
}
