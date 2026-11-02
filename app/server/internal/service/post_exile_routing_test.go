package service

import (
	"context"
	"testing"
	"time"

	"feedchamber/internal/mock"
	"feedchamber/internal/model"
	"feedchamber/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newPostSvcForRouting 组装一个只为验证 exile 路由的 PostService。
// store 传 nil：Create 的写路径不在本测试覆盖范围，只调到 NewPost 之前的
// routeExiledAuthor。
func newPostSvcForRouting(t *testing.T, users *mock.UserRepository, spaces *mock.SpaceRepository) *postService {
	t.Helper()
	return &postService{users: users, spaceRepo: spaces}
}

func TestRouteExiledAuthor_RoutesToDarkZone(t *testing.T) {
	ctx := context.Background()
	users := mock.NewUserRepository()
	spaces := mock.NewSpaceRepository()
	require.NoError(t, spaces.Create(ctx, &model.Space{ID: "dz-1", Slug: DarkZoneSlug, Name: "暗区", System: true}))

	future := time.Now().Add(72 * time.Hour)
	require.NoError(t, users.Create(ctx, &model.User{ID: "exiled", Username: "exiled", ExiledUntil: &future}))
	require.NoError(t, users.Create(ctx, &model.User{ID: "clean", Username: "clean"}))

	svc := newPostSvcForRouting(t, users, spaces)

	got, routed := svc.routeExiledAuthor(ctx, "exiled", "tech-space")
	assert.True(t, routed, "软隔离用户的新帖应被改道")
	assert.Equal(t, "dz-1", got)

	got, routed = svc.routeExiledAuthor(ctx, "clean", "tech-space")
	assert.False(t, routed)
	assert.Equal(t, "tech-space", got, "未受隔离的用户保持原版面")
}

func TestRouteExiledAuthor_ExpiredExileIsInactive(t *testing.T) {
	ctx := context.Background()
	users := mock.NewUserRepository()
	spaces := mock.NewSpaceRepository()
	require.NoError(t, spaces.Create(ctx, &model.Space{ID: "dz-1", Slug: DarkZoneSlug, Name: "暗区"}))

	past := time.Now().Add(-time.Hour)
	require.NoError(t, users.Create(ctx, &model.User{ID: "was-exiled", Username: "was", ExiledUntil: &past}))

	svc := newPostSvcForRouting(t, users, spaces)
	got, routed := svc.routeExiledAuthor(ctx, "was-exiled", "tech-space")
	assert.False(t, routed, "隔离期满应自动失效")
	assert.Equal(t, "tech-space", got)
}

// 暗区缺失时不得阻塞发帖（治理增强不能变成发帖故障）。
func TestRouteExiledAuthor_MissingDarkZoneFallsBack(t *testing.T) {
	ctx := context.Background()
	users := mock.NewUserRepository()
	spaces := mock.NewSpaceRepository()
	_ = repository.ErrNotFound // 保持 import 使用

	future := time.Now().Add(time.Hour)
	require.NoError(t, users.Create(ctx, &model.User{ID: "exiled", Username: "exiled", ExiledUntil: &future}))

	svc := newPostSvcForRouting(t, users, spaces)
	got, routed := svc.routeExiledAuthor(ctx, "exiled", "tech-space")
	assert.False(t, routed, "暗区不存在时保持原版面，不阻断发帖")
	assert.Equal(t, "tech-space", got)
}

func TestRouteExiledAuthor_NilDepsSafe(t *testing.T) {
	svc := &postService{}
	got, routed := svc.routeExiledAuthor(context.Background(), "any", "tech")
	assert.False(t, routed)
	assert.Equal(t, "tech", got)
}
