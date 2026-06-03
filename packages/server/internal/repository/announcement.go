package repository

import (
	"context"
	"time"

	"orinforum/internal/db"
	"orinforum/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gorm.io/gorm"
)

type AnnouncementRepository interface {
	FindByID(ctx context.Context, id string) (*model.Announcement, error)
	List(ctx context.Context, offset, limit int) ([]model.Announcement, error)
	ListPublished(ctx context.Context, offset, limit int) ([]model.Announcement, error)
	Create(ctx context.Context, ann *model.Announcement) error
	Update(ctx context.Context, ann *model.Announcement) error
	Delete(ctx context.Context, id string) error
	IncrementViewCount(ctx context.Context, id string) error
}

func NewAnnouncementRepository(store db.Store) AnnouncementRepository {
	if store.SQL() != nil {
		return &sqlAnnouncementRepo{db: store.SQL()}
	}
	return &mongoAnnouncementRepo{store: store.Mongo()}
}

// ── SQL implementation ──────────────────────────────────────────────────────

type sqlAnnouncementRepo struct{ db db.SQLStore }

func (r *sqlAnnouncementRepo) FindByID(ctx context.Context, id string) (*model.Announcement, error) {
	var a model.Announcement
	return &a, r.db.DB().WithContext(ctx).Where("id = ?", id).First(&a).Error
}

func (r *sqlAnnouncementRepo) List(ctx context.Context, offset, limit int) ([]model.Announcement, error) {
	var announcements []model.Announcement
	err := r.db.DB().WithContext(ctx).Order("sort desc, created_at desc").Offset(offset).Limit(limit).Find(&announcements).Error
	return announcements, err
}

func (r *sqlAnnouncementRepo) ListPublished(ctx context.Context, offset, limit int) ([]model.Announcement, error) {
	var announcements []model.Announcement
	now := time.Now()
	err := r.db.DB().WithContext(ctx).
		Where("status = ? AND (expires_at IS NULL OR expires_at > ?)", model.AnnStatusPublished, now).
		Order("is_pinned desc, sort desc, created_at desc").
		Offset(offset).
		Limit(limit).
		Find(&announcements).Error
	return announcements, err
}

func (r *sqlAnnouncementRepo) Create(ctx context.Context, ann *model.Announcement) error {
	return r.db.DB().WithContext(ctx).Create(ann).Error
}

func (r *sqlAnnouncementRepo) Update(ctx context.Context, ann *model.Announcement) error {
	return r.db.DB().WithContext(ctx).Save(ann).Error
}

func (r *sqlAnnouncementRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Announcement{}, "id = ?", id).Error
}

func (r *sqlAnnouncementRepo) IncrementViewCount(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Model(&model.Announcement{}).Where("id = ?", id).UpdateColumn("view_count", gorm.Expr("view_count + ?", 1)).Error
}

// ── MongoDB implementation ──────────────────────────────────────────────────

type mongoAnnouncementRepo struct{ store db.MongoStore }

func (r *mongoAnnouncementRepo) FindByID(ctx context.Context, id string) (*model.Announcement, error) {
	var a model.Announcement
	err := r.store.Collection("announcements").FindOne(ctx, bson.M{"_id": id}).Decode(&a)
	return &a, err
}

func (r *mongoAnnouncementRepo) List(ctx context.Context, offset, limit int) ([]model.Announcement, error) {
	opts := options.Find().SetSort(bson.D{{Key: "sort", Value: -1}, {Key: "created_at", Value: -1}}).SetSkip(int64(offset)).SetLimit(int64(limit))
	cursor, err := r.store.Collection("announcements").Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var announcements []model.Announcement
	err = cursor.All(ctx, &announcements)
	return announcements, err
}

func (r *mongoAnnouncementRepo) ListPublished(ctx context.Context, offset, limit int) ([]model.Announcement, error) {
	now := time.Now()
	filter := bson.M{
		"status": model.AnnStatusPublished,
		"$or": bson.A{
			bson.M{"expires_at": bson.M{"$exists": false}},
			bson.M{"expires_at": bson.M{"$gt": now}},
		},
	}
	opts := options.Find().SetSort(bson.D{{Key: "is_pinned", Value: -1}, {Key: "sort", Value: -1}, {Key: "created_at", Value: -1}}).SetSkip(int64(offset)).SetLimit(int64(limit))
	cursor, err := r.store.Collection("announcements").Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	var announcements []model.Announcement
	err = cursor.All(ctx, &announcements)
	return announcements, err
}

func (r *mongoAnnouncementRepo) Create(ctx context.Context, ann *model.Announcement) error {
	_, err := r.store.Collection("announcements").InsertOne(ctx, ann)
	return err
}

func (r *mongoAnnouncementRepo) Update(ctx context.Context, ann *model.Announcement) error {
	_, err := r.store.Collection("announcements").ReplaceOne(ctx, bson.M{"_id": ann.ID}, ann)
	return err
}

func (r *mongoAnnouncementRepo) Delete(ctx context.Context, id string) error {
	_, err := r.store.Collection("announcements").DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *mongoAnnouncementRepo) IncrementViewCount(ctx context.Context, id string) error {
	_, err := r.store.Collection("announcements").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$inc": bson.M{"view_count": 1}})
	return err
}
