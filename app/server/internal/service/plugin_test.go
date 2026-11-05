package service_test

import (
	"context"
	"testing"

	"feedchamber/internal/model"
	"feedchamber/internal/service"

	"github.com/stretchr/testify/require"
)

// 本文件钉住既有的声明式 CRUD 行为（列表 / 启停 / 删除 / settings 校验）。
// 它们早于子进程插件存在，任何改写都不能让「登记元数据」这条路坏掉。
//
// 记录的**创建**已统一走 Install（trust model 第一层），因此这里先用一次
// 合法安装铺底，再验证其余动作。

func seedInstalledPlugin(t *testing.T, svc service.PluginService) {
	t.Helper()
	ctx := context.Background()
	src, err := svc.CreateSource(ctx, "test", "git", "https://example.com/x.git", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveSource(ctx, src.ID, "admin2"))
	_, err = svc.Install(ctx, service.PluginInstallInput{
		PluginID: "analytics", SourceID: src.ID, Manifest: validManifestJSON,
	})
	require.NoError(t, err)
}

func TestPluginService_ListToggleDelete(t *testing.T) {
	ctx := context.Background()
	svc, _ := newPluginTestSvc(t)
	seedInstalledPlugin(t, svc)

	items, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "analytics", items[0].ID)
	require.True(t, items[0].Enabled)

	require.NoError(t, svc.SetEnabled(ctx, "analytics", false))
	items, err = svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.False(t, items[0].Enabled)

	require.NoError(t, svc.Delete(ctx, "analytics"))
	items, err = svc.List(ctx)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestPluginService_RejectsInvalidSettings(t *testing.T) {
	ctx := context.Background()
	svc, _ := newPluginTestSvc(t)
	seedInstalledPlugin(t, svc)

	err := svc.Update(ctx, service.PluginInput{ID: "analytics", Name: "Analytics", Settings: "not-json"})
	require.Error(t, err)
}

// 更新不得把安装时授予的 capability 与 manifest 抹掉。
//
// Update 只覆盖声明式的展示字段；Manifest / Capabilities / SourceID 是
// 安装期的信任快照，改它们必须重新走审批，不能经由一次元数据编辑悄悄改掉。
func TestPluginService_UpdatePreservesInstallSnapshot(t *testing.T) {
	ctx := context.Background()
	svc, store := newPluginTestSvc(t)
	seedInstalledPlugin(t, svc)

	require.NoError(t, svc.Update(ctx, service.PluginInput{
		ID: "analytics", Name: "Renamed", Version: "2.0.0", Enabled: true,
	}))

	var row model.Plugin
	require.NoError(t, store.SQL().DB().Where("id = ?", "analytics").First(&row).Error)
	require.Equal(t, "Renamed", row.Name)
	require.Equal(t, "2.0.0", row.Version)
	require.Equal(t, validManifestJSON, row.Manifest)
	require.JSONEq(t, `["posts:write"]`, row.Capabilities)
	require.NotEmpty(t, row.SourceID)
}
