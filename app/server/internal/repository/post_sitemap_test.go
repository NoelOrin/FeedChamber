package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"orinforum/internal/config"
	"orinforum/internal/db"
	"orinforum/internal/model"

	"github.com/stretchr/testify/require"
)

// newSitemapTestRepo builds an isolated SQLite store with the minimum schema the
// sitemap queries touch. Posts need no user/space rows: the sitemap projection
// deliberately does not join them.
func newSitemapTestRepo(t *testing.T) (PostRepository, db.Store) {
	t.Helper()
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "sitemap.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(&model.Post{}, &model.Content{}, &model.User{}, &model.Space{}))
	return NewPostRepository(store), store
}

func TestListSitemapEntries_FiltersByStatusAndSoftDelete(t *testing.T) {
	repo, store := newSitemapTestRepo(t)
	ctx := context.Background()

	now := time.Now()
	posts := []model.Post{
		{ID: "post-approved", Title: "approved", Slug: "approved", ContentID: "c1", AuthorID: "u1", Status: model.PostStatusApproved, UpdatedAt: now},
		{ID: "post-closed", Title: "closed", Slug: "closed", ContentID: "c2", AuthorID: "u1", Status: model.PostStatusClosed, UpdatedAt: now},
		{ID: "post-pending", Title: "pending", Slug: "pending", ContentID: "c3", AuthorID: "u1", Status: model.PostStatusPending, UpdatedAt: now},
	}
	for i := range posts {
		require.NoError(t, store.SQL().DB().Create(&posts[i]).Error)
	}
	// A soft-deleted approved post must not leak into the sitemap.
	require.NoError(t, store.SQL().DB().Create(&model.Post{
		ID: "post-deleted", Title: "deleted", Slug: "deleted", ContentID: "c4", AuthorID: "u1", Status: model.PostStatusApproved,
	}).Error)
	require.NoError(t, repo.Delete(ctx, "post-deleted"))

	entries, err := repo.ListSitemapEntries(ctx, 0, 100, nil)
	require.NoError(t, err)

	ids := make([]string, len(entries))
	for i, e := range entries {
		ids[i] = e.ID
	}
	require.ElementsMatch(t, []string{"post-approved", "post-closed"}, ids,
		"only approved/closed, non-deleted posts belong in the sitemap")

	total, err := repo.CountSitemapEntries(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, 2, total)
}

func TestListSitemapEntries_PaginationAndProjection(t *testing.T) {
	repo, store := newSitemapTestRepo(t)
	ctx := context.Background()

	base := time.Now().Add(-time.Hour)
	for i := 0; i < 5; i++ {
		require.NoError(t, store.SQL().DB().Create(&model.Post{
			ID:        "post-" + string(rune('a'+i)),
			Title:     "title",
			Slug:      "slug-" + string(rune('a'+i)),
			ContentID: "content-" + string(rune('a'+i)),
			AuthorID:  "u1",
			Status:    model.PostStatusApproved,
			// Ascending UpdatedAt so ORDER BY updated_at DESC yields e,d,c,b,a.
			UpdatedAt: base.Add(time.Duration(i) * time.Minute),
		}).Error)
	}

	page1, err := repo.ListSitemapEntries(ctx, 0, 2, nil)
	require.NoError(t, err)
	require.Len(t, page1, 2)
	require.Equal(t, "post-e", page1[0].ID, "newest first")
	require.Equal(t, "post-d", page1[1].ID)

	page2, err := repo.ListSitemapEntries(ctx, 2, 2, nil)
	require.NoError(t, err)
	require.Len(t, page2, 2)
	require.Equal(t, "post-c", page2[0].ID)

	// Projection carries exactly what the sitemap emits.
	require.Equal(t, "slug-e", page1[0].Slug)
	require.False(t, page1[0].UpdatedAt.IsZero(), "updated_at must be populated for <lastmod>")
}
