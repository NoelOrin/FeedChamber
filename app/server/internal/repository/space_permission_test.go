package repository

import (
	"context"
	"testing"

	"orinforum/internal/model"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type mockSQLStore struct{ db *gorm.DB }

func (m *mockSQLStore) DB() *gorm.DB                      { return m.db }
func (m *mockSQLStore) TxDB(ctx context.Context) *gorm.DB { return m.db }

func newSpacePermTestStore(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Permission{}, &model.SpaceRolePermission{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func seedPerm(t *testing.T, db *gorm.DB, code string) uint {
	t.Helper()
	perm := model.Permission{Code: code, Name: code}
	if err := db.Where("code = ?", code).FirstOrCreate(&perm).Error; err != nil {
		t.Fatalf("seed perm: %v", err)
	}
	return perm.ID
}

func TestSpacePerm_SyncAndGet(t *testing.T) {
	gdb := newSpacePermTestStore(t)
	_ = seedPerm(t, gdb, "post:create")
	_ = seedPerm(t, gdb, "post:read")

	repo := &sqlSpaceRolePermRepo{db: &mockSQLStore{db: gdb}}

	ctx := context.Background()
	err := repo.SyncSpaceRolePermissions(ctx, "space-1", "admin", []string{"post:create", "post:read"})
	assert.NoError(t, err)

	codes, exists, err := repo.GetSpaceRolePermissions(ctx, "space-1", "admin")
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.ElementsMatch(t, []string{"post:create", "post:read"}, codes)
}

func TestSpacePerm_GetNoOverride(t *testing.T) {
	gdb := newSpacePermTestStore(t)
	repo := &sqlSpaceRolePermRepo{db: &mockSQLStore{db: gdb}}

	codes, exists, err := repo.GetSpaceRolePermissions(context.Background(), "space-none", "none")
	assert.NoError(t, err)
	assert.False(t, exists)
	assert.Empty(t, codes)
}

func TestSpacePerm_Clear(t *testing.T) {
	gdb := newSpacePermTestStore(t)
	_ = seedPerm(t, gdb, "post:create")

	repo := &sqlSpaceRolePermRepo{db: &mockSQLStore{db: gdb}}
	ctx := context.Background()

	err := repo.SyncSpaceRolePermissions(ctx, "space-1", "admin", []string{"post:create"})
	assert.NoError(t, err)

	err = repo.ClearSpaceRolePermissions(ctx, "space-1", "admin")
	assert.NoError(t, err)

	codes, exists, err := repo.GetSpaceRolePermissions(ctx, "space-1", "admin")
	assert.NoError(t, err)
	assert.False(t, exists)
	assert.Empty(t, codes)
}

func TestSpacePerm_GetAll(t *testing.T) {
	gdb := newSpacePermTestStore(t)
	_ = seedPerm(t, gdb, "post:create")
	_ = seedPerm(t, gdb, "post:read")

	repo := &sqlSpaceRolePermRepo{db: &mockSQLStore{db: gdb}}
	ctx := context.Background()

	err := repo.SyncSpaceRolePermissions(ctx, "space-1", "admin", []string{"post:create"})
	assert.NoError(t, err)

	err = repo.SyncSpaceRolePermissions(ctx, "space-2", "mod", []string{"post:read"})
	assert.NoError(t, err)

	result, err := repo.GetAllSpaceRolePermissions(ctx)
	assert.NoError(t, err)

	assert.Contains(t, result, "space-1")
	assert.Contains(t, result, "space-2")
	assert.ElementsMatch(t, []string{"post:create"}, result["space-1"]["admin"])
	assert.ElementsMatch(t, []string{"post:read"}, result["space-2"]["mod"])
}

func TestSpacePerm_SyncUnknownCode(t *testing.T) {
	gdb := newSpacePermTestStore(t)
	_ = seedPerm(t, gdb, "post:read")
	repo := &sqlSpaceRolePermRepo{db: &mockSQLStore{db: gdb}}
	err := repo.SyncSpaceRolePermissions(context.Background(), "space-1", "admin", []string{"post:read", "post:nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown codes")
}
