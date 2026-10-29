package db

import (
	"context"
	"fmt"
	"orinforum/internal/config"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// SQLStore exposes the underlying *gorm.DB for repositories that need it.
type SQLStore interface {
	DB() *gorm.DB
	// TxDB returns the transaction-aware *gorm.DB for ctx (falls back to base DB).
	TxDB(ctx context.Context) *gorm.DB
}

type sqlStore struct {
	db      *gorm.DB
	backend BackendType
}

func newSQLStore(cfg config.DBConfig, backend BackendType) (Store, error) {
	var dialector gorm.Dialector

	switch backend {
	case BackendSQLite:
		path := cfg.DSN
		if path == "" {
			if err := os.MkdirAll("db", 0o755); err != nil {
				return nil, fmt.Errorf("create db dir: %w", err)
			}
			path = "db/app.db?_journal_mode=DELETE&_busy_timeout=5000&_foreign_keys=ON"
		}
		dialector = sqlite.Open(path)

	case BackendMySQL:
		dsn := cfg.DSN
		if dsn == "" {
			dsn = fmt.Sprintf(
				"%s:%s@tcp(%s:%s)/orinforum?charset=utf8mb4&parseTime=True&loc=Local",
				getEnv("DB_USER", "root"),
				getEnv("DB_PASSWORD", ""),
				getEnv("DB_HOST", "localhost"),
				getEnv("DB_PORT", "3306"),
			)
		}
		dialector = mysql.Open(dsn)

	case BackendPostgres:
		dsn := cfg.DSN
		if dsn == "" {
			dsn = fmt.Sprintf(
				"host=%s user=%s password=%s dbname=orinforum port=%s sslmode=disable",
				getEnv("DB_HOST", "localhost"),
				getEnv("DB_USER", "postgres"),
				getEnv("DB_PASSWORD", "postgres"),
				getEnv("DB_PORT", "5432"),
			)
		}
		dialector = postgres.Open(dsn)

	default:
		return nil, fmt.Errorf("unknown SQL backend: %s", backend)
	}

	gdb, err := gorm.Open(dialector, &gorm.Config{
		Logger: newSlowQueryLogger(),
	})
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", backend, err)
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	if err = sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping %s: %w", backend, err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	sqlDB.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTime) * time.Second)

	return &sqlStore{db: gdb, backend: backend}, nil
}

func (s *sqlStore) AutoMigrate(models ...any) error {
	return s.db.AutoMigrate(models...)
}

func (s *sqlStore) SQL() SQLStore                     { return s }
func (s *sqlStore) Type() BackendType                 { return s.backend }
func (s *sqlStore) DB() *gorm.DB                      { return s.db }
func (s *sqlStore) TxDB(ctx context.Context) *gorm.DB { return GetTxDB(ctx, s.db) }

func (s *sqlStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (s *sqlStore) PoolStats() *PoolStats {
	sqlDB, err := s.db.DB()
	if err != nil {
		return nil
	}
	stats := sqlDB.Stats()
	return &PoolStats{
		MaxOpenConnections: stats.MaxOpenConnections,
		OpenConnections:    stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
		WaitCount:          int(stats.WaitCount),
		WaitDurationMs:     int(stats.WaitDuration.Milliseconds()),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
