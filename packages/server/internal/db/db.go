package db

import (
	"context"
	"fmt"
	"orinforum/internal/config"
)

// Store is the unified database abstraction.
// SQL backends use GORM; MongoDB uses the official driver.
type Store interface {
	// AutoMigrate creates or updates tables/collections for the given models.
	// For MongoDB this is a no-op (schema-less).
	AutoMigrate(models ...any) error

	// SQL-style repository access — returns nil for Mongo backends.
	SQL() SQLStore

	// Mongo-style repository access — returns nil for SQL backends.
	Mongo() MongoStore

	// Type returns the configured backend identifier.
	Type() BackendType

	// Close releases all connections.
	Close() error
}

type BackendType string

const (
	BackendSQLite  BackendType = "sqlite"
	BackendMySQL   BackendType = "mysql"
	BackendPostgres BackendType = "postgres"
	BackendMongo   BackendType = "mongo"
)

// New constructs the appropriate Store implementation based on cfg.Type.
func New(cfg config.DBConfig) (Store, error) {
	switch BackendType(cfg.Type) {
	case BackendSQLite:
		return newSQLStore(cfg, BackendSQLite)
	case BackendMySQL:
		return newSQLStore(cfg, BackendMySQL)
	case BackendPostgres:
		return newSQLStore(cfg, BackendPostgres)
	case BackendMongo:
		return newMongoStore(context.Background(), cfg)
	default:
		return newSQLStore(cfg, BackendSQLite) // sensible default
	}
}

// ErrUnsupported is returned when an operation is not supported by the backend.
var ErrUnsupported = fmt.Errorf("operation not supported by this backend")
