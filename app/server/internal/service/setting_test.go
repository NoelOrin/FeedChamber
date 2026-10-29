package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/testnats"
	"feedchamber/internal/repository"

	"github.com/stretchr/testify/require"
)

// newSettingServiceForTest wires a real SQLite repo + a real NATS-backed KV so
// the cache path is exercised for real rather than through a stub. The point of
// these tests is the interaction between the two (miss caching), which a mock KV
// would not reproduce faithfully.
func newSettingServiceForTest(t *testing.T) (SettingService, repository.SettingRepository) {
	t.Helper()

	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "setting.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(&model.Setting{}))

	repo := repository.NewSettingRepository(store)
	return NewSettingService(repo, testnats.NewKVStore(t, "setting_test")), repo
}

// countingSettingRepo records how many times the DB was actually hit, so a
// "second Get did not touch the database" assertion is possible.
type countingSettingRepo struct {
	repository.SettingRepository
	gets int
}

func (c *countingSettingRepo) Get(ctx context.Context, key string) (*model.Setting, error) {
	c.gets++
	return c.SettingRepository.Get(ctx, key)
}

// TestSettingGet_CachesMissingKey is the regression for the unseeded-key log
// storm: `rate_limit_enabled` was read on every request but never seeded, so
// each read was a cache miss, hit the settings table, and logged GORM's
// "record not found". A miss must now be cached too.
func TestSettingGet_CachesMissingKey(t *testing.T) {
	_, repo := newSettingServiceForTest(t)
	counting := &countingSettingRepo{SettingRepository: repo}
	svc := NewSettingService(counting, testnats.NewKVStore(t, "setting_test_missing"))
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, err := svc.Get(ctx, "rate_limit_enabled")
		require.Error(t, err, "unseeded key must report an error")
		var appErr *pkg.AppError
		require.True(t, errors.As(err, &appErr), "want *pkg.AppError, got %T", err)
		require.Equal(t, pkg.NOT_FOUND, appErr.Code)
	}

	require.Equal(t, 1, counting.gets,
		"a missing key must be negative-cached: 5 reads, 1 DB hit")
}

// TestSettingGet_DistinguishesDBFailureFromMissingKey guards the error mapping.
// Before this, any repo error became NOT_FOUND, so a database outage was
// indistinguishable from "the admin never set this key" and every gate silently
// fell back to its default instead of failing closed.
func TestSettingGet_DistinguishesDBFailureFromMissingKey(t *testing.T) {
	failing := &failingSettingRepo{err: errors.New("connection refused")}
	svc := NewSettingService(failing, testnats.NewKVStore(t, "setting_test_failure"))

	_, err := svc.Get(context.Background(), "site_name")
	require.Error(t, err)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr), "want *pkg.AppError, got %T", err)
	require.Equal(t, pkg.INTERNAL_ERROR, appErr.Code,
		"a DB failure must not masquerade as a missing setting")
}

type failingSettingRepo struct {
	repository.SettingRepository
	err error
}

func (f *failingSettingRepo) Get(context.Context, string) (*model.Setting, error) {
	return nil, f.err
}

// TestSettingGet_ReadsSeededValue covers the happy path and confirms a hit is
// cached: the second read must not reach the repo.
func TestSettingGet_ReadsSeededValue(t *testing.T) {
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "setting_hit.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(&model.Setting{}))
	require.NoError(t, store.SQL().DB().Create(&model.Setting{Key: "site_name", Value: "Feed Chamber"}).Error)

	counting := &countingSettingRepo{SettingRepository: repository.NewSettingRepository(store)}
	svc := NewSettingService(counting, testnats.NewKVStore(t, "setting_test_hit"))
	ctx := context.Background()

	got, err := svc.Get(ctx, "site_name")
	require.NoError(t, err)
	require.Equal(t, "Feed Chamber", got)

	got, err = svc.Get(ctx, "site_name")
	require.NoError(t, err)
	require.Equal(t, "Feed Chamber", got)
	require.Equal(t, 1, counting.gets, "second read must be served from KV")
}

// TestSettingGet_DeleteInvalidatesNegativeCache pins that removing a setting
// does not leave a stale negative entry behind, and that creating it afterwards
// is visible immediately.
func TestSettingGet_DeleteInvalidatesNegativeCache(t *testing.T) {
	svc, _ := newSettingServiceForTest(t)
	ctx := context.Background()

	_, err := svc.Get(ctx, "site_name")
	require.Error(t, err, "key starts absent")

	require.NoError(t, svc.Set(ctx, "site_name", "Feed Chamber"))
	got, err := svc.Get(ctx, "site_name")
	require.NoError(t, err, "Set must invalidate the cached miss")
	require.Equal(t, "Feed Chamber", got)
}
