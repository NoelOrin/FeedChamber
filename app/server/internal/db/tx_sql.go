package db

import (
	"context"

	"gorm.io/gorm"
)

type sqlTxManager struct {
	db SQLStore
}

func (m *sqlTxManager) WithTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	return m.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txContextKey{}, tx)
		return fn(txCtx)
	})
}

// GetTxDB returns the transaction *gorm.DB from ctx if present, otherwise the
// base db; the returned db always has ctx set via WithContext.
func GetTxDB(ctx context.Context, base *gorm.DB) *gorm.DB {
	return GetTx(ctx, base).WithContext(ctx)
}

func GetTx(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txContextKey{}).(*gorm.DB); ok {
		return tx
	}
	return db
}
