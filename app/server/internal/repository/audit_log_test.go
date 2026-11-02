package repository

import (
	"context"
	"testing"
	"time"

	"feedchamber/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAuditLogTestRepo(t *testing.T) *sqlAuditLogRepo {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.AuditLog{}))
	return &sqlAuditLogRepo{db: &mockSQLStore{db: db}}
}

func TestAuditLog_CreateAndList(t *testing.T) {
	repo := newAuditLogTestRepo(t)
	ctx := context.Background()

	now := time.Now()
	logs := []model.AuditLog{
		{ID: "1", ActorID: "actor-a", Action: "settings.update", Target: "site_name", Detail: "x", CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "2", ActorID: "actor-a", Action: "user.ban", Target: "user-1", Detail: "", CreatedAt: now.Add(-1 * time.Minute)},
		{ID: "3", ActorID: "actor-b", Action: "settings.update", Target: "site_url", Detail: "y", CreatedAt: now},
	}
	for i := range logs {
		require.NoError(t, repo.Create(ctx, &logs[i]))
	}

	all, total, err := repo.List(ctx, 0, 10, "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, all, 3)
	// newest first
	assert.Equal(t, "3", all[0].ID)

	byActor, total, err := repo.List(ctx, 0, 10, "actor-a", "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, byActor, 2)

	byAction, total, err := repo.List(ctx, 0, 10, "", "settings.update")
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, byAction, 2)

	filtered, total, err := repo.List(ctx, 0, 10, "actor-a", "user.ban")
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, filtered, 1)
	assert.Equal(t, "2", filtered[0].ID)

	page, total, err := repo.List(ctx, 1, 1, "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, page, 1)
	assert.Equal(t, "2", page[0].ID)
}
