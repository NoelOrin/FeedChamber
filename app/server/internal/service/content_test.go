package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"feedchamber/internal/mock"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestContentService(
	contentRepo *mock.ContentRepository,
	postRepo *mock.PostRepository,
	userRepo *mock.UserRepository,
) ContentService {
	return NewContentService(ContentServiceDeps{
		Contents: contentRepo,
		Posts:    postRepo,
		Users:    userRepo,
		Tx:       nil,
	})
}

func TestContentGetByID_Found(t *testing.T) {
	contentRepo := mock.NewContentRepository()
	postRepo := mock.NewPostRepository()
	userRepo := mock.NewUserRepository()
	svc := newTestContentService(contentRepo, postRepo, userRepo)

	_ = userRepo.Create(context.Background(), &model.User{
		ID:       "author-uuid",
		Username: "alice",
		Avatar:   "https://example.com/alice.jpg",
	})

	_ = postRepo.Create(context.Background(), &model.Post{
		ID:        "post-1",
		ContentID: "root-1",
		Title:     "Test Post",
	})

	_ = contentRepo.Create(context.Background(), &model.Content{
		ID:        "c1",
		AuthorID:  "author-uuid",
		Body:      "Hello world",
		RootID:    "root-1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	result, err := svc.GetByID(context.Background(), "c1")
	require.NoError(t, err)
	assert.Equal(t, "c1", result.ID)
	assert.Equal(t, "alice", result.AuthorName)
	assert.Equal(t, "Test Post", result.PostTitle)
	assert.Equal(t, "post-1", result.PostID)
}

func TestContentGetByID_NotFound(t *testing.T) {
	contentRepo := mock.NewContentRepository()
	postRepo := mock.NewPostRepository()
	userRepo := mock.NewUserRepository()
	svc := newTestContentService(contentRepo, postRepo, userRepo)

	_, err := svc.GetByID(context.Background(), "nonexistent")
	assert.Error(t, err)

	var appErr *pkg.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.NOT_FOUND, appErr.Code)
}

func TestContentGetByID_MissingAuthorAndPost(t *testing.T) {
	// Author/post lookup fails gracefully — returns content without enrichment
	contentRepo := mock.NewContentRepository()
	postRepo := mock.NewPostRepository()
	userRepo := mock.NewUserRepository()
	svc := newTestContentService(contentRepo, postRepo, userRepo)

	_ = contentRepo.Create(context.Background(), &model.Content{
		ID:       "c2",
		AuthorID: "gone-author",
		Body:     "orphan",
		RootID:   "gone-root",
	})

	result, err := svc.GetByID(context.Background(), "c2")
	require.NoError(t, err)
	assert.Equal(t, "c2", result.ID)
	assert.Equal(t, "", result.AuthorName) // not found, stays empty
	assert.Equal(t, "", result.PostTitle)  // not found, stays empty
}
