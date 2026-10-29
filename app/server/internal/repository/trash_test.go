package repository

import (
	"context"
	"path/filepath"
	"testing"

	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/model"

	"github.com/stretchr/testify/require"
)

func TestPostTrashListAndRestore_SQLite(t *testing.T) {
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "trash.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(&model.Post{}, &model.Content{}, &model.User{}, &model.Space{}))
	require.NoError(t, store.SQL().DB().Create(&model.User{
		ID:       "user-trash-1",
		Username: "trash-user",
		Email:    "trash-user@example.com",
	}).Error)

	repo := NewPostRepository(store)
	post := &model.Post{
		ID:        "post-trash-1",
		Title:     "trash me",
		ContentID: "content-trash-1",
		AuthorID:  "user-trash-1",
		Status:    model.PostStatusApproved,
	}
	require.NoError(t, repo.Create(context.Background(), post))
	require.NoError(t, repo.Delete(context.Background(), post.ID))

	items, total, err := repo.ListTrash(context.Background(), 0, 20)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, post.ID, items[0].ID)

	require.NoError(t, repo.Restore(context.Background(), post.ID))
	items, total, err = repo.ListTrash(context.Background(), 0, 20)
	require.NoError(t, err)
	require.Equal(t, 0, total)
	require.Empty(t, items)
}
