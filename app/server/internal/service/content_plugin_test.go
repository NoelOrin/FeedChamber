package service

import (
	"context"
	"testing"

	"feedchamber/internal/mock"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"

	"github.com/stretchr/testify/require"
)

// 插件宿主 API 的 comments.setStatus 必须走 service 层（与管理员操作同入口），
// 而不是给插件一个直连 repository 的后门。这里钉住取值域校验与写回。

func TestContentService_SetStatusForPlugin(t *testing.T) {
	contentRepo := mock.NewContentRepository()
	svc := NewContentService(ContentServiceDeps{Contents: contentRepo})
	ctx := context.Background()

	c := &model.Content{ID: "c1", Body: "hello", Status: model.PostStatusApproved}
	require.NoError(t, contentRepo.Create(ctx, c))

	require.NoError(t, svc.SetStatusForPlugin(ctx, "c1", model.PostStatusRejected))
	got, err := contentRepo.FindByID(ctx, "c1")
	require.NoError(t, err)
	require.Equal(t, model.PostStatusRejected, got.Status)
}

func TestContentService_SetStatusForPluginRejectsBadStatus(t *testing.T) {
	contentRepo := mock.NewContentRepository()
	svc := NewContentService(ContentServiceDeps{Contents: contentRepo})
	ctx := context.Background()

	c := &model.Content{ID: "c1", Body: "hello", Status: model.PostStatusApproved}
	require.NoError(t, contentRepo.Create(ctx, c))

	err := svc.SetStatusForPlugin(ctx, "c1", "bogus")
	require.Error(t, err)
	require.Equal(t, pkg.INVALID_PARAMS, err.(*pkg.AppError).Code)
	got, getErr := contentRepo.FindByID(ctx, "c1")
	require.NoError(t, getErr)
	require.Equal(t, model.PostStatusApproved, got.Status, "非法状态不得写回")
}
