package service

import (
	"context"
	"errors"
	"testing"

	"orinforum/internal/mock"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/pkg/testnats"
	"orinforum/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPublicPostStatus(t *testing.T) {
	require.True(t, isPublicPostStatus(model.PostStatusApproved))
	require.True(t, isPublicPostStatus(model.PostStatusClosed))
	require.False(t, isPublicPostStatus(model.PostStatusPending))
	require.False(t, isPublicPostStatus(model.PostStatusRejected))
}

func TestCanViewNonPublic(t *testing.T) {
	svc := &postService{}
	require.False(t, svc.canViewNonPublic(context.Background()))
	require.False(t, svc.canViewNonPublic(WithViewerRole(context.Background(), "user")))
	require.True(t, svc.canViewNonPublic(WithViewerRole(context.Background(), "admin")))
	require.True(t, svc.canViewNonPublic(WithViewerRole(context.Background(), "moderator")))
}

// stubDetailPostRepo 让 FindByIDWithDetails / FindDetailByID 返回受控数据。
// mock.PostRepository 的这两个方法恒返回 (nil, nil)，无法表达「帖子存在但非公开」。
type stubDetailPostRepo struct {
	*mock.PostRepository
	item   *model.PostListItem
	detail *model.PostDetail
}

func (s *stubDetailPostRepo) FindByIDWithDetails(_ context.Context, _ string) (*model.PostListItem, error) {
	if s.item == nil {
		return nil, repository.ErrNotFound
	}
	return s.item, nil
}

func (s *stubDetailPostRepo) FindDetailByID(_ context.Context, _ string) (*model.PostDetail, error) {
	if s.detail == nil {
		return nil, repository.ErrNotFound
	}
	return s.detail, nil
}

func newPendingPostFixture(t *testing.T, authorID string) *postService {
	t.Helper()
	repo := &stubDetailPostRepo{
		PostRepository: mock.NewPostRepository(),
		item: &model.PostListItem{
			ID:       "p1",
			Title:    "pending topic",
			AuthorID: authorID,
			Status:   model.PostStatusPending,
		},
		detail: &model.PostDetail{
			PostListItem: model.PostListItem{
				ID:       "p1",
				Title:    "pending topic",
				AuthorID: authorID,
				Status:   model.PostStatusPending,
			},
			Body: "pending body",
		},
	}
	return &postService{posts: repo, kv: testnats.NewKVStore(t, "post_visibility_"+t.Name())}
}

// 契约 3：作者本人 MUST 能读到自己 status 非 approved/closed 的帖子，响应带真实 status。
func TestPostGetByID_AuthorCanReadOwnPendingPost(t *testing.T) {
	svc := newPendingPostFixture(t, "author-1")
	ctx := WithViewerID(context.Background(), "author-1")

	got, err := svc.GetByID(ctx, "p1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, model.PostStatusPending, got.Status)
}

func TestPostGetDetail_AuthorCanReadOwnPendingPost(t *testing.T) {
	svc := newPendingPostFixture(t, "author-1")
	ctx := WithViewerID(context.Background(), "author-1")

	got, err := svc.GetDetail(ctx, "p1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, model.PostStatusPending, got.Status)
	assert.Equal(t, "pending body", got.Body)
}

// 契约 3：他人（含匿名）读同一篇 pending 帖 MUST 404，且 MUST NOT 泄露存在性。
func TestPostGetByID_OthersCannotReadPendingPost(t *testing.T) {
	svc := newPendingPostFixture(t, "author-1")

	for name, ctx := range map[string]context.Context{
		"anonymous":  context.Background(),
		"other user": WithViewerID(WithViewerRole(context.Background(), "user"), "other-1"),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := svc.GetByID(ctx, "p1")
			require.Error(t, err)
			var appErr *pkg.AppError
			require.True(t, errors.As(err, &appErr))
			assert.Equal(t, pkg.NOT_FOUND, appErr.Code)

			_, err = svc.GetDetail(ctx, "p1")
			require.Error(t, err)
			require.True(t, errors.As(err, &appErr))
			assert.Equal(t, pkg.NOT_FOUND, appErr.Code)
		})
	}
}

// 契约 3 的对照面：具备审核权限者仍可读非公开帖。
func TestPostGetByID_ReviewerCanReadPendingPost(t *testing.T) {
	svc := newPendingPostFixture(t, "author-1")
	svc.perms = &stubPermChecker{allow: map[string]bool{"moderator:report:manage": true}}
	ctx := WithViewerRole(context.Background(), "moderator")

	got, err := svc.GetByID(ctx, "p1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, model.PostStatusPending, got.Status)
}

// 契约 2：哨兵 "__all__" = 不过滤状态，仅对 canViewNonPublic==true 的调用者生效。
func TestPostList_AllSentinelSkipsStatusFilterForReviewer(t *testing.T) {
	posts := mock.NewPostRepository()
	svc := &postService{posts: posts, perms: &stubPermChecker{allow: map[string]bool{"user:report:manage": true}}}
	ctx := WithViewerRole(context.Background(), "user")

	_, _, err := svc.List(ctx, 1, 10, "", "", repository.StatusFilterAll, nil, nil, "", "", "", false, "")
	require.NoError(t, err)
	assert.Empty(t, posts.LastListFilter.Status, "哨兵 MUST NOT 被当作字面状态")
	assert.Empty(t, posts.LastListFilter.StatusIn, "哨兵 MUST 表示不过滤状态")
}

// 契约 2：无权限者无论传什么（含哨兵）一律只见 approved + closed。
func TestPostList_AllSentinelIgnoredForAnonymous(t *testing.T) {
	posts := mock.NewPostRepository()
	svc := &postService{posts: posts}

	_, _, err := svc.List(context.Background(), 1, 10, "", "", repository.StatusFilterAll, nil, nil, "", "", "", false, "")
	require.NoError(t, err)
	assert.Empty(t, posts.LastListFilter.Status)
	assert.Equal(t, []string{model.PostStatusApproved, model.PostStatusClosed}, posts.LastListFilter.StatusIn)
}

// 回归护栏：有权限但不传 status 时，仍沿用公开面默认（approved + closed），
// 只有显式哨兵才放开全部状态。
func TestPostList_ReviewerWithoutStatusKeepsDefault(t *testing.T) {
	posts := mock.NewPostRepository()
	svc := &postService{posts: posts, perms: &stubPermChecker{allow: map[string]bool{"admin:report:manage": true}}}
	ctx := WithViewerRole(context.Background(), "admin")

	_, _, err := svc.List(ctx, 1, 10, "", "", "", nil, nil, "", "", "", false, "")
	require.NoError(t, err)
	assert.Empty(t, posts.LastListFilter.Status)
	assert.Equal(t, []string{model.PostStatusApproved, model.PostStatusClosed}, posts.LastListFilter.StatusIn)
}
