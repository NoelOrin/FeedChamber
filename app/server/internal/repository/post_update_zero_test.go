package repository

import (
	"context"
	"path/filepath"
	"testing"

	"orinforum/internal/config"
	"orinforum/internal/db"
	"orinforum/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 治理标记必须能从 true 改回 false。GORM 的 Updates(struct) 会跳过零值，
// 曾导致翻案（解除 exiled）与解除 folded 静默失效。
func TestPostUpdate_ClearsGovernanceFlags(t *testing.T) {
	ctx := context.Background()
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "p.db")})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrate(&model.Post{}, &model.Content{}))

	repo := NewPostRepository(store)
	p := &model.Post{
		ID: "p1", Title: "t", ContentID: "c1", AuthorID: "a", SpaceID: "dz",
		Status: model.PostStatusApproved, Exiled: true, Folded: true,
	}
	require.NoError(t, repo.Create(ctx, p))

	// 模拟翻案：两标记都改回 false
	p.Exiled = false
	p.Folded = false
	require.NoError(t, repo.Update(ctx, p))

	got, err := repo.FindByID(ctx, "p1")
	require.NoError(t, err)
	assert.False(t, got.Exiled, "翻案必须能把 exiled 写回 false")
	assert.False(t, got.Folded, "解除折叠必须能把 folded 写回 false")
}
