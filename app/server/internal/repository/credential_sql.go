package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"time"

	"gorm.io/gorm/clause"
)

type sqlCredentialRepo struct{ db db.SQLStore }

func (r *sqlCredentialRepo) FindByUserID(ctx context.Context, userID string) (*model.Credential, error) {
	var c model.Credential
	return &c, WrapDBErr(r.db.TxDB(ctx).First(&c, "user_id = ?", userID).Error)
}

func (r *sqlCredentialRepo) Upsert(ctx context.Context, c *model.Credential) error {
	c.UpdatedAt = time.Now()
	// Use OnConflict to upsert — works with PostgreSQL and SQLite.
	return r.db.TxDB(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			UpdateAll: true,
		}).
		Create(c).Error
}

func (r *sqlCredentialRepo) Delete(ctx context.Context, userID string) error {
	return r.db.TxDB(ctx).Delete(&model.Credential{}, "user_id = ?", userID).Error
}
