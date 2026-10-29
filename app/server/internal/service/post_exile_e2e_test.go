package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"orinforum/internal/config"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 端到端：exile 用户通过真实 Create 落库，帖子必须进暗区且 exiled=true，
// 且默认（主推荐流）列表查不到它、按暗区版面查得到。
func TestCreate_ExiledAuthorPostLandsInDarkZoneAndIsHidden(t *testing.T) {
	ctx := context.Background()
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "e2e.db")})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrate(
		&model.User{}, &model.Post{}, &model.Content{}, &model.Space{}, &model.PostTag{}, &model.Tag{},
	))

	spaceRepo := repository.NewSpaceRepository(store)
	userRepo := repository.NewUserRepository(store)
	postRepo := repository.NewPostRepository(store)
	contentRepo := repository.NewContentRepository(store)

	tech := &model.Space{ID: "tech", Name: "技术", Slug: "tech", Recommendable: true, IsActive: true}
	dz := &model.Space{ID: "dz", Name: "暗区", Slug: DarkZoneSlug, System: true, Recommendable: false, IsActive: true}
	require.NoError(t, spaceRepo.Create(ctx, tech))
	require.NoError(t, spaceRepo.Create(ctx, dz))

	future := time.Now().Add(48 * time.Hour)
	require.NoError(t, userRepo.Create(ctx, &model.User{ID: "u-exiled", Username: "ex", Email: "ex@e.com", ExiledUntil: &future}))

	svc := &postService{
		posts:     postRepo,
		contents:  contentRepo,
		users:     userRepo,
		spaceRepo: spaceRepo,
	}

	post, err := svc.Create(ctx, "u-exiled", "被隔离者的新帖", "正文内容用于验证暗区路由", "tech")
	require.NoError(t, err)
	require.NotNil(t, post)

	assert.Equal(t, "dz", post.SpaceID, "应落到暗区而非用户指定的 tech")
	assert.True(t, post.Exiled, "须置 exiled 标记")

	// 直接走仓库层验证过滤语义（service.List 的签名另有 13 个参数）
	// 主推荐流（未限定版面）应查不到
	mainItems, mainTotal, err := postRepo.ListWithTotal(ctx, repository.PostListFilter{
		StatusIn:      []string{model.PostStatusApproved, model.PostStatusClosed},
		ExcludeExiled: true,
	}, 0, 20)
	require.NoError(t, err)
	require.EqualValues(t, 0, mainTotal, "暗区内容不得出现在主推荐流，items=%d", len(mainItems))

	// 精确查暗区应查得到（暗区浏览不带 ExcludeExiled）
	dzItems, dzTotal, err := postRepo.ListWithTotal(ctx, repository.PostListFilter{
		StatusIn: []string{model.PostStatusApproved, model.PostStatusClosed},
		SpaceID:  "dz",
	}, 0, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, dzTotal, "暗区内应可见")
	require.Len(t, dzItems, 1)
}
