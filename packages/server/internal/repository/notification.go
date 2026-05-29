package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type NotificationRepository interface {
	Create(ctx context.Context, n *model.Notification) error
	FindByID(ctx context.Context, id string) (*model.Notification, error)
	ListByUser(ctx context.Context, userID string, offset, limit int) ([]model.Notification, error)
	CountUnread(ctx context.Context, userID string) (int64, error)
	MarkRead(ctx context.Context, id string) error
	MarkAllRead(ctx context.Context, userID string) error
	Delete(ctx context.Context, id string) error
}

func NewNotificationRepository(store db.Store) NotificationRepository {
	if store.SQL() != nil {
		return &sqlNotificationRepo{db: store.SQL()}
	}
	return &mongoNotificationRepo{store: store.Mongo()}
}

// ── SQL implementation ──────────────────────────────────────────────────────

type sqlNotificationRepo struct{ db db.SQLStore }

func (r *sqlNotificationRepo) Create(ctx context.Context, n *model.Notification) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	return r.db.DB().WithContext(ctx).Create(n).Error
}

func (r *sqlNotificationRepo) FindByID(ctx context.Context, id string) (*model.Notification, error) {
	var n model.Notification
	err := r.db.DB().WithContext(ctx).First(&n, "id = ?", id).Error
	return &n, err
}

func (r *sqlNotificationRepo) ListByUser(ctx context.Context, userID string, offset, limit int) ([]model.Notification, error) {
	var notifications []model.Notification
	err := r.db.DB().WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&notifications).Error
	return notifications, err
}

func (r *sqlNotificationRepo) CountUnread(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).
		Model(&model.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&count).Error
	return count, err
}

func (r *sqlNotificationRepo) MarkRead(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).
		Model(&model.Notification{}).
		Where("id = ?", id).
		UpdateColumn("is_read", true).Error
}

func (r *sqlNotificationRepo) MarkAllRead(ctx context.Context, userID string) error {
	return r.db.DB().WithContext(ctx).
		Model(&model.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		UpdateColumn("is_read", true).Error
}

func (r *sqlNotificationRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).
		Delete(&model.Notification{}, "id = ?", id).Error
}

// ── MongoDB implementation ──────────────────────────────────────────────────

type mongoNotificationRepo struct{ store db.MongoStore }

func (r *mongoNotificationRepo) Create(ctx context.Context, n *model.Notification) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	_, err := r.store.Collection("notifications").InsertOne(ctx, n)
	return err
}

func (r *mongoNotificationRepo) FindByID(ctx context.Context, id string) (*model.Notification, error) {
	var n model.Notification
	err := r.store.Collection("notifications").FindOne(ctx, bson.M{"_id": id}).Decode(&n)
	return &n, err
}

func (r *mongoNotificationRepo) ListByUser(ctx context.Context, userID string, offset, limit int) ([]model.Notification, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))
	cursor, err := r.store.Collection("notifications").Find(ctx, bson.M{"user_id": userID}, opts)
	if err != nil {
		return nil, err
	}
	var notifications []model.Notification
	err = cursor.All(ctx, &notifications)
	return notifications, err
}

func (r *mongoNotificationRepo) CountUnread(ctx context.Context, userID string) (int64, error) {
	count, err := r.store.Collection("notifications").CountDocuments(ctx, bson.M{
		"user_id": userID,
		"is_read": false,
	})
	return count, err
}

func (r *mongoNotificationRepo) MarkRead(ctx context.Context, id string) error {
	_, err := r.store.Collection("notifications").UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"is_read": true}},
	)
	return err
}

func (r *mongoNotificationRepo) MarkAllRead(ctx context.Context, userID string) error {
	_, err := r.store.Collection("notifications").UpdateMany(ctx,
		bson.M{"user_id": userID, "is_read": false},
		bson.M{"$set": bson.M{"is_read": true}},
	)
	return err
}

func (r *mongoNotificationRepo) Delete(ctx context.Context, id string) error {
	_, err := r.store.Collection("notifications").DeleteOne(ctx, bson.M{"_id": id})
	return err
}
