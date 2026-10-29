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

type stubPermChecker struct {
	allow map[string]bool
}

func (s *stubPermChecker) HasPermission(roleName, permCode string) bool {
	return s.allow[roleName+":"+permCode]
}

func (s *stubPermChecker) HasPermissionInSpace(roleName, spaceID, permCode string) bool {
	return s.allow[roleName+":"+permCode]
}

func (s *stubPermChecker) GetRolePermissions(roleName string) []string { return nil }

func (s *stubPermChecker) HasPermissionForUser(ctx context.Context, userID, roleName, permCode string) bool {
	return s.allow[roleName+":"+permCode]
}

func (s *stubPermChecker) HasPermissionInSpaceForUser(ctx context.Context, userID, roleName, spaceID, permCode string) bool {
	return s.allow[roleName+":"+permCode]
}

type noopTxManager struct{}

func (noopTxManager) WithTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

func TestPostSetClosed_AuthorCanCloseApproved(t *testing.T) {
	posts := mock.NewPostRepository()
	_ = posts.Create(context.Background(), &model.Post{
		ID:       "p1",
		AuthorID: "author-1",
		Status:   model.PostStatusApproved,
		SpaceID:  "s1",
	})
	svc := &postService{posts: posts}

	post, err := svc.SetClosed(context.Background(), "p1", true, "author-1", "user")
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusClosed, post.Status)

	got, err := posts.FindByID(context.Background(), "p1")
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusClosed, got.Status)
}

func TestPostSetClosed_ModeratorWithLockCanClose(t *testing.T) {
	posts := mock.NewPostRepository()
	_ = posts.Create(context.Background(), &model.Post{
		ID:       "p1",
		AuthorID: "author-1",
		Status:   model.PostStatusApproved,
		SpaceID:  "s1",
	})
	svc := &postService{
		posts: posts,
		perms: &stubPermChecker{allow: map[string]bool{"moderator:" + model.PermPostLock: true}},
	}

	post, err := svc.SetClosed(context.Background(), "p1", true, "mod-1", "moderator")
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusClosed, post.Status)
}

func TestPostSetClosed_ForbiddenWithoutAuthorOrLock(t *testing.T) {
	posts := mock.NewPostRepository()
	_ = posts.Create(context.Background(), &model.Post{
		ID:       "p1",
		AuthorID: "author-1",
		Status:   model.PostStatusApproved,
		SpaceID:  "s1",
	})
	svc := &postService{
		posts: posts,
		perms: &stubPermChecker{allow: map[string]bool{}},
	}

	_, err := svc.SetClosed(context.Background(), "p1", true, "other", "user")
	require.Error(t, err)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.FORBIDDEN, appErr.Code)
}

func TestPostSetClosed_PendingCannotClose(t *testing.T) {
	posts := mock.NewPostRepository()
	_ = posts.Create(context.Background(), &model.Post{
		ID:       "p1",
		AuthorID: "author-1",
		Status:   model.PostStatusPending,
	})
	svc := &postService{posts: posts}

	_, err := svc.SetClosed(context.Background(), "p1", true, "author-1", "user")
	require.Error(t, err)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.INVALID_PARAMS, appErr.Code)
}

func TestPostSetClosed_ReopenClosedToApproved(t *testing.T) {
	posts := mock.NewPostRepository()
	_ = posts.Create(context.Background(), &model.Post{
		ID:       "p1",
		AuthorID: "author-1",
		Status:   model.PostStatusClosed,
	})
	svc := &postService{posts: posts}

	post, err := svc.SetClosed(context.Background(), "p1", false, "author-1", "user")
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusApproved, post.Status)
}

func TestPostSetClosed_Idempotent(t *testing.T) {
	posts := mock.NewPostRepository()
	_ = posts.Create(context.Background(), &model.Post{
		ID:       "p1",
		AuthorID: "author-1",
		Status:   model.PostStatusClosed,
	})
	svc := &postService{posts: posts}

	post, err := svc.SetClosed(context.Background(), "p1", true, "author-1", "user")
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusClosed, post.Status)
}

func TestPostSetClosed_NotFound(t *testing.T) {
	svc := &postService{posts: mock.NewPostRepository()}
	_, err := svc.SetClosed(context.Background(), "missing", true, "author-1", "user")
	require.Error(t, err)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.NOT_FOUND, appErr.Code)
}

func TestContentCreate_BlockedWhenClosed(t *testing.T) {
	contentRepo := mock.NewContentRepository()
	postRepo := mock.NewPostRepository()
	userRepo := mock.NewUserRepository()
	_ = postRepo.Create(context.Background(), &model.Post{
		ID:        "p1",
		ContentID: "root-1",
		AuthorID:  "author-1",
		Status:    model.PostStatusClosed,
	})
	svc := NewContentService(ContentServiceDeps{
		Contents: contentRepo,
		Posts:    postRepo,
		Users:    userRepo,
		Tx:       noopTxManager{},
	})

	_, err := svc.Create(context.Background(), "p1", "author-1", "hello", "")
	require.Error(t, err)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.FORBIDDEN, appErr.Code)
	assert.Contains(t, appErr.Message, "关闭")
}

func TestContentCreate_BlockedWhenLocked(t *testing.T) {
	contentRepo := mock.NewContentRepository()
	postRepo := mock.NewPostRepository()
	userRepo := mock.NewUserRepository()
	_ = postRepo.Create(context.Background(), &model.Post{
		ID:        "p1",
		ContentID: "root-1",
		AuthorID:  "author-1",
		Status:    model.PostStatusApproved,
		Locked:    true,
	})
	svc := NewContentService(ContentServiceDeps{
		Contents: contentRepo,
		Posts:    postRepo,
		Users:    userRepo,
		Tx:       noopTxManager{},
	})

	_, err := svc.Create(context.Background(), "p1", "author-1", "hello", "")
	require.Error(t, err)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.FORBIDDEN, appErr.Code)
}

func TestContentCreate_TouchesLastActiveAt(t *testing.T) {
	contentRepo := mock.NewContentRepository()
	postRepo := mock.NewPostRepository()
	userRepo := mock.NewUserRepository()
	old := time.Now().Add(-2 * time.Hour)
	_ = postRepo.Create(context.Background(), &model.Post{
		ID:           "p1",
		ContentID:    "root-1",
		AuthorID:     "author-1",
		Status:       model.PostStatusApproved,
		LastActiveAt: old,
	})
	svc := NewContentService(ContentServiceDeps{
		Contents: contentRepo,
		Posts:    postRepo,
		Users:    userRepo,
		Tx:       noopTxManager{},
	})

	_, err := svc.Create(context.Background(), "p1", "author-1", "hello", "")
	require.NoError(t, err)

	got, err := postRepo.FindByID(context.Background(), "p1")
	require.NoError(t, err)
	assert.True(t, got.LastActiveAt.After(old))
}

func TestNewPost_SetsLastActiveAt(t *testing.T) {
	before := time.Now().Add(-time.Second)
	post, _ := model.NewPost("a1", "t", "body", "s1")
	require.False(t, post.LastActiveAt.IsZero())
	assert.True(t, !post.LastActiveAt.Before(before))
}
