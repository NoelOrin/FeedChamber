package db

import (
	"feedchamber/internal/config"
	"fmt"
)

// Store is the unified database abstraction.
// All backends use GORM.
type Store interface {
	// AutoMigrate creates or updates tables for the given models.
	AutoMigrate(models ...any) error

	// SQL-style repository access.
	SQL() SQLStore

	// Type returns the configured backend identifier.
	Type() BackendType

	// Close releases all connections.
	Close() error

	// PoolStats returns connection pool statistics for monitoring.
	// Returns nil for backends that do not expose pool stats.
	PoolStats() *PoolStats
}

// PoolStats holds DB connection pool information for the /health and /metrics endpoints.
type PoolStats struct {
	MaxOpenConnections int `json:"max_open_connections"`
	OpenConnections    int `json:"open_connections"`
	InUse              int `json:"in_use"`
	Idle               int `json:"idle"`
	WaitCount          int `json:"wait_count"`
	WaitDurationMs     int `json:"wait_duration_ms"`
}

type BackendType string

const (
	BackendSQLite   BackendType = "sqlite"
	BackendMySQL    BackendType = "mysql"
	BackendPostgres BackendType = "postgres"
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
	default:
		return nil, fmt.Errorf("unknown database backend: %s", cfg.Type)
	}
}
