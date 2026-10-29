package db

import "context"

// TransactionManager provides a unified transaction abstraction
// backed by gorm.DB.Transaction.
type TransactionManager interface {
	// WithTx executes fn within a transaction.
	// If fn returns an error, the transaction is rolled back; otherwise committed.
	WithTx(ctx context.Context, fn func(txCtx context.Context) error) error
}

// txContextKey is used to propagate the transaction context.
type txContextKey struct{}

// NewTransactionManager creates a TransactionManager for the given store.
func NewTransactionManager(store Store) TransactionManager {
	return &sqlTxManager{db: store.SQL()}
}
