package service

import (
	"context"
	"testing"

	"feedchamber/internal/mock"
	"feedchamber/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubDescendantSpaceRepo 让 FindDescendantIDs 返回受控的子版面；
// mock.SpaceRepository 恒返回 nil，无法表达「父版面确实有子版面」。
type stubDescendantSpaceRepo struct {
	*mock.SpaceRepository
	ids []string
	err error
}

func (s *stubDescendantSpaceRepo) FindDescendantIDs(_ context.Context, _ string) ([]string, error) {
	return s.ids, s.err
}

func newIncludeSubFixture(ids []string) (*postService, *mock.PostRepository) {
	posts := mock.NewPostRepository()
	return &postService{
		posts:     posts,
		spaceRepo: &stubDescendantSpaceRepo{SpaceRepository: mock.NewSpaceRepository(), ids: ids},
	}, posts
}

// 契约：space_id + include_sub=true 时，列表过滤 MUST 展开成「本版 + 全部后代版」，
// 且不能把 space_id 再单列过滤（两者同时存在时 SpaceIDs 优先，SpaceID 必须清空）。
func TestPostList_IncludeSubExpandsDescendants(t *testing.T) {
	svc, posts := newIncludeSubFixture([]string{"child-1", "child-2"})

	_, _, err := svc.List(context.Background(), 1, 20, "", "", "", nil, nil, "", "", "parent-1", true, "")
	require.NoError(t, err)

	filter := posts.LastListFilter
	assert.ElementsMatch(t, []string{"parent-1", "child-1", "child-2"}, filter.SpaceIDs)
	assert.Empty(t, filter.SpaceID, "SpaceID 与 SpaceIDs 同时设置会让 IN 子句掩盖单值过滤")
}

// 契约：include_sub=false（默认）时只看本版，子版面帖子不进列表。
func TestPostList_WithoutIncludeSubKeepsSingleSpace(t *testing.T) {
	svc, posts := newIncludeSubFixture([]string{"child-1"})

	_, _, err := svc.List(context.Background(), 1, 20, "", "", "", nil, nil, "", "", "parent-1", false, "")
	require.NoError(t, err)

	filter := posts.LastListFilter
	assert.Empty(t, filter.SpaceIDs)
	assert.Equal(t, "parent-1", filter.SpaceID)
}

// 契约：子版查询失败时退回单版过滤，父版面不能因为一次 DB 抖动变成空列表。
func TestPostList_IncludeSubFallsBackOnRepoError(t *testing.T) {
	svc := &postService{
		posts:     mock.NewPostRepository(),
		spaceRepo: &stubDescendantSpaceRepo{SpaceRepository: mock.NewSpaceRepository(), err: repository.ErrNotFound},
	}

	_, _, err := svc.List(context.Background(), 1, 20, "", "", "", nil, nil, "", "", "parent-1", true, "")
	require.NoError(t, err)

	filter := svc.posts.(*mock.PostRepository).LastListFilter
	assert.Equal(t, []string{"parent-1"}, filter.SpaceIDs)
	assert.Empty(t, filter.SpaceID)
}
