package service

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/repository"
	"feedchamber/internal/storage"

	"github.com/stretchr/testify/require"
)

func TestBackupCreateAndRestore_SQLite(t *testing.T) {
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "backup.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	require.NoError(t, store.AutoMigrate(&model.Backup{}, &model.User{}, &model.Post{}))
	user := &model.User{
		ID:       "user-1",
		Username: "before",
		Email:    "before@example.com",
		Role:     "user",
	}
	post := &model.Post{
		ID:        "post-1",
		Title:     "before title",
		AuthorID:  "user-1",
		Status:    model.PostStatusApproved,
		ContentID: "content-1",
	}
	require.NoError(t, store.SQL().DB().Create(user).Error)
	require.NoError(t, store.SQL().DB().Create(post).Error)

	ctx := context.Background()
	storageDir := t.TempDir()
	st := storage.NewLocalStorage(storageDir)
	require.NoError(t, st.Put(ctx, "uploads/avatar.png", bytes.NewReader([]byte("avatar-data")), 11, "image/png"))
	svc := NewBackupService(repository.NewBackupRepository(store), st, store, nil)
	created, err := svc.Create(ctx, "admin-1", "roundtrip")
	require.NoError(t, err)
	require.Equal(t, "ready", created.Status)
	require.Positive(t, created.Size)
	require.NotEmpty(t, created.UploadsKey)
	require.NotEmpty(t, created.UploadsChecksum)

	require.NoError(t, store.SQL().DB().Model(&model.User{}).Where("id = ?", user.ID).Update("username", "after").Error)
	require.NoError(t, store.SQL().DB().Model(&model.Post{}).Where("id = ?", post.ID).Update("title", "after title").Error)
	require.NoError(t, st.Delete(ctx, "uploads/avatar.png"))

	require.NoError(t, svc.Restore(ctx, created.ID, "admin-1"))

	var restoredUser model.User
	require.NoError(t, store.SQL().DB().First(&restoredUser, "id = ?", user.ID).Error)
	require.Equal(t, "before", restoredUser.Username)
	var restoredPost model.Post
	require.NoError(t, store.SQL().DB().First(&restoredPost, "id = ?", post.ID).Error)
	require.Equal(t, "before title", restoredPost.Title)
	reader, err := st.Get(ctx, "uploads/avatar.png")
	require.NoError(t, err)
	defer reader.Close()
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "avatar-data", string(content))
}

func TestBackupRestore_RejectsWrongBackend(t *testing.T) {
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "backup.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(&model.Backup{}))

	svc := NewBackupService(repository.NewBackupRepository(store), storage.NewLocalStorage(t.TempDir()), store, nil)
	err = svc.Restore(context.Background(), "missing", "admin-1")
	require.Error(t, err)
}

func TestBackupCleanup_RemovesExpired(t *testing.T) {
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "backup.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(&model.Backup{}))

	ctx := context.Background()
	repo := repository.NewBackupRepository(store)
	st := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, st.Put(ctx, "backups/old.json", bytes.NewReader([]byte("old")), 3, "application/json"))
	require.NoError(t, repo.Create(ctx, &model.Backup{
		ID:        "old-backup",
		Name:      "old",
		FileKey:   "backups/old.json",
		Status:    "ready",
		CreatedAt: time.Now().Add(-40 * 24 * time.Hour),
	}))

	svc := NewBackupService(repo, st, store, nil)
	removed, err := svc.Cleanup(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, removed)
	_, err = st.Get(ctx, "backups/old.json")
	require.Error(t, err)
}
