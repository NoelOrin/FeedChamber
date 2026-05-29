package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type ReportRepository interface {
	Create(ctx context.Context, report *model.Report) error
	FindByID(ctx context.Context, id string) (*model.Report, error)
	List(ctx context.Context, offset, limit int) ([]model.Report, error)
	ListByStatus(ctx context.Context, status string, offset, limit int) ([]model.Report, error)
	UpdateStatus(ctx context.Context, id, status string) error
	Delete(ctx context.Context, id string) error
}

func NewReportRepository(store db.Store) ReportRepository {
	if store.SQL() != nil {
		return &sqlReportRepo{db: store.SQL()}
	}
	return &mongoReportRepo{store: store.Mongo()}
}

// ── SQL ──────────────────────────────────────────────────────────────────────

type sqlReportRepo struct{ db db.SQLStore }

func (r *sqlReportRepo) Create(ctx context.Context, report *model.Report) error {
	return r.db.DB().WithContext(ctx).Create(report).Error
}

func (r *sqlReportRepo) FindByID(ctx context.Context, id string) (*model.Report, error) {
	var report model.Report
	err := r.db.DB().WithContext(ctx).First(&report, "id = ?", id).Error
	return &report, err
}

func (r *sqlReportRepo) List(ctx context.Context, offset, limit int) ([]model.Report, error) {
	var reports []model.Report
	err := r.db.DB().WithContext(ctx).Order("created_at desc").Offset(offset).Limit(limit).Find(&reports).Error
	return reports, err
}

func (r *sqlReportRepo) ListByStatus(ctx context.Context, status string, offset, limit int) ([]model.Report, error) {
	var reports []model.Report
	err := r.db.DB().WithContext(ctx).Where("status = ?", status).Order("created_at desc").Offset(offset).Limit(limit).Find(&reports).Error
	return reports, err
}

func (r *sqlReportRepo) UpdateStatus(ctx context.Context, id, status string) error {
	return r.db.DB().WithContext(ctx).Model(&model.Report{}).Where("id = ?", id).UpdateColumn("status", status).Error
}

func (r *sqlReportRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Report{}, "id = ?", id).Error
}

// ── MongoDB ──────────────────────────────────────────────────────────────────

type mongoReportRepo struct{ store db.MongoStore }

func (r *mongoReportRepo) Create(ctx context.Context, report *model.Report) error {
	_, err := r.store.Collection("reports").InsertOne(ctx, report)
	return err
}

func (r *mongoReportRepo) FindByID(ctx context.Context, id string) (*model.Report, error) {
	var report model.Report
	err := r.store.Collection("reports").FindOne(ctx, bson.M{"_id": id}).Decode(&report)
	return &report, err
}

func (r *mongoReportRepo) List(ctx context.Context, offset, limit int) ([]model.Report, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetSkip(int64(offset)).SetLimit(int64(limit))
	cursor, err := r.store.Collection("reports").Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var reports []model.Report
	err = cursor.All(ctx, &reports)
	return reports, err
}

func (r *mongoReportRepo) ListByStatus(ctx context.Context, status string, offset, limit int) ([]model.Report, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetSkip(int64(offset)).SetLimit(int64(limit))
	cursor, err := r.store.Collection("reports").Find(ctx, bson.M{"status": status}, opts)
	if err != nil {
		return nil, err
	}
	var reports []model.Report
	err = cursor.All(ctx, &reports)
	return reports, err
}

func (r *mongoReportRepo) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.store.Collection("reports").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"status": status}})
	return err
}

func (r *mongoReportRepo) Delete(ctx context.Context, id string) error {
	_, err := r.store.Collection("reports").DeleteOne(ctx, bson.M{"_id": id})
	return err
}
