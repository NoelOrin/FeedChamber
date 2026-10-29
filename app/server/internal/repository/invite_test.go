package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"feedchamber/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newInviteTestRepo(t *testing.T) *sqlInviteRepo {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:invite_atomic?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.Invite{}))
	return &sqlInviteRepo{db: &mockSQLStore{db: db}}
}

func TestInviteRepo_ConsumeByCode_AtomicExhaustion(t *testing.T) {
	repo := newInviteTestRepo(t)
	ctx := context.Background()

	inv := model.NewInvite("ONEUSE", "admin1", 1, nil)
	require.NoError(t, repo.Create(ctx, inv))

	require.NoError(t, repo.ConsumeByCode(ctx, inv.Code))
	err := repo.ConsumeByCode(ctx, inv.Code)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInviteExhausted))

	got, err := repo.FindByCode(ctx, inv.Code)
	require.NoError(t, err)
	assert.Equal(t, 1, got.UsedCount)
}

func TestInviteRepo_ConsumeByCode_ExpiredAndMissing(t *testing.T) {
	repo := newInviteTestRepo(t)
	ctx := context.Background()

	past := time.Now().Add(-time.Minute)
	require.NoError(t, repo.Create(ctx, model.NewInvite("OLDEXP", "admin1", 0, &past)))

	err := repo.ConsumeByCode(ctx, "OLDEXP")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInviteExpired))

	err = repo.ConsumeByCode(ctx, "NOTEXIST")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestInviteRepo_ConsumeByCode_ConcurrentMaxUses(t *testing.T) {
	repo := newInviteTestRepo(t)
	ctx := context.Background()

	inv := model.NewInvite("RACECODE", "admin1", 1, nil)
	require.NoError(t, repo.Create(ctx, inv))

	const workers = 16
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		success  int
		failures int
	)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			err := repo.ConsumeByCode(ctx, inv.Code)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				success++
				return
			}
			failures++
			assert.True(t, errors.Is(err, ErrInviteExhausted))
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, success)
	assert.Equal(t, workers-1, failures)

	got, err := repo.FindByCode(ctx, inv.Code)
	require.NoError(t, err)
	assert.Equal(t, 1, got.UsedCount)
}
