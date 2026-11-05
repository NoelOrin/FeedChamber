package service_test

import (
	"context"
	"errors"
	"testing"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/plugin"
	"feedchamber/internal/service"

	"github.com/stretchr/testify/require"
)

// 本文件钉住「后台管理动作必须真正驱动子进程运行时」这条最小闭环。
//
// 背景：Install / SetEnabled / Delete / Uninstall 此前只写库，Host 永远不知道
// 管理员做了什么——装完状态停在 installed（恢复逻辑只认 running），启停只翻
// 一个没人读的 enabled 列，删除留下 plugin_installs 孤儿。fakeRuntime 用一个
// 计数器把「有没有真的调用宿主」变成可断言的行为。

// fakePluginRuntime 只实现 PluginRuntime 的窄接口，不拉起任何真实进程。
type fakePluginRuntime struct {
	started   []string
	stopped   []string
	running   map[string]bool
	failStart error
}

func (f *fakePluginRuntime) StartPlugin(id string, _ *plugin.Manifest, _ plugin.CapabilitySet) error {
	if f.failStart != nil {
		return f.failStart
	}
	if f.running == nil {
		f.running = map[string]bool{}
	}
	if f.running[id] {
		return errors.New("plugin: " + id + " 已在运行")
	}
	f.started = append(f.started, id)
	f.running[id] = true
	return nil
}

func (f *fakePluginRuntime) StopPlugin(id string) error {
	f.stopped = append(f.stopped, id)
	if f.running == nil {
		f.running = map[string]bool{}
	}
	f.running[id] = false
	return nil
}

func (f *fakePluginRuntime) Running(id string) bool {
	return f.running[id]
}

// managedPlugin 铺好一条已审批来源 + 已登记插件，返回 service、fake runtime 与 db.Store。
func managedPlugin(t *testing.T) (service.PluginService, *fakePluginRuntime, db.Store) {
	t.Helper()
	svc, store := newPluginTestSvc(t)
	rt := &fakePluginRuntime{}
	svc.SetRuntime(rt)
	ctx := context.Background()
	src, err := svc.CreateSource(ctx, "test", "git", "https://example.com/x.git", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveSource(ctx, src.ID, "admin2"))
	_, err = svc.Install(ctx, service.PluginInstallInput{
		PluginID: "analytics", SourceID: src.ID, Manifest: validManifestJSON,
	})
	require.NoError(t, err)
	return svc, rt, store
}

// 安装成功后必须真的拉起插件，且落库状态推进到 running（重启恢复只认 running）。
func TestInstallStartsRuntimeAndMarksRunning(t *testing.T) {
	svc, rt, store := managedPlugin(t)
	_ = svc

	require.Equal(t, []string{"analytics"}, rt.started, "Install 必须调用 StartPlugin")

	var inst model.PluginInstall
	require.NoError(t, store.SQL().DB().Where("plugin_id = ?", "analytics").First(&inst).Error)
	require.Equal(t, model.PluginStatusRunning, inst.Status,
		"安装落库状态必须是 running，否则 ListResumable 永远捞不到它")
}

// 禁用必须停进程，启用必须重新拉起（用落库的 manifest/caps 快照复原）。
func TestSetEnabledDrivesRuntime(t *testing.T) {
	svc, rt, store := managedPlugin(t)
	ctx := context.Background()

	require.NoError(t, svc.SetEnabled(ctx, "analytics", false))
	require.Equal(t, []string{"analytics"}, rt.stopped, "禁用必须 StopPlugin")

	require.NoError(t, svc.SetEnabled(ctx, "analytics", true))
	require.Equal(t, []string{"analytics", "analytics"}, rt.started, "启用必须重新 StartPlugin")

	var inst model.PluginInstall
	require.NoError(t, store.SQL().DB().Where("plugin_id = ?", "analytics").First(&inst).Error)
	require.Equal(t, model.PluginStatusRunning, inst.Status)
}

// 卸载必须停进程并清掉两张表，不能留下孤儿安装记录。
func TestUninstallStopsRuntimeAndCleansBothTables(t *testing.T) {
	svc, rt, store := managedPlugin(t)
	ctx := context.Background()

	require.NoError(t, svc.Uninstall(ctx, "analytics"))
	require.Equal(t, []string{"analytics"}, rt.stopped, "卸载必须 StopPlugin")

	var plugins []model.Plugin
	var installs []model.PluginInstall
	require.NoError(t, store.SQL().DB().Find(&plugins).Error)
	require.NoError(t, store.SQL().DB().Find(&installs).Error)
	require.Empty(t, plugins)
	require.Empty(t, installs, "卸载必须删掉 plugin_installs 行")
}

// 后台的「删除」按钮走 Delete；它也必须是完整卸载（停进程 + 双表清理）。
func TestDeleteStopsRuntimeAndCleansBothTables(t *testing.T) {
	svc, rt, store := managedPlugin(t)
	ctx := context.Background()

	require.NoError(t, svc.Delete(ctx, "analytics"))
	require.Equal(t, []string{"analytics"}, rt.stopped, "Delete 必须 StopPlugin")

	var installs []model.PluginInstall
	require.NoError(t, store.SQL().DB().Find(&installs).Error)
	require.Empty(t, installs, "Delete 不能留下 plugin_installs 孤儿")
}

// 宿主拉起失败时不能留下「装了但跑不起来」的半安装状态。
func TestInstallRollsBackWhenRuntimeFails(t *testing.T) {
	svc, store := newPluginTestSvc(t)
	rt := &fakePluginRuntime{failStart: errors.New("node not found")}
	svc.SetRuntime(rt)
	ctx := context.Background()
	src, err := svc.CreateSource(ctx, "test", "git", "https://example.com/x.git", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveSource(ctx, src.ID, "admin2"))

	_, err = svc.Install(ctx, service.PluginInstallInput{
		PluginID: "analytics", SourceID: src.ID, Manifest: validManifestJSON,
	})
	require.Error(t, err)

	var plugins []model.Plugin
	var installs []model.PluginInstall
	require.NoError(t, store.SQL().DB().Find(&plugins).Error)
	require.NoError(t, store.SQL().DB().Find(&installs).Error)
	require.Empty(t, plugins, "启动失败必须回滚 plugins 行")
	require.Empty(t, installs, "启动失败必须回滚 plugin_installs 行")
}

// 未接 runtime 时（如纯元数据查询场景）保持原行为，不 panic。
func TestServiceWorksWithoutRuntime(t *testing.T) {
	svc, store := newPluginTestSvc(t)
	ctx := context.Background()
	src, err := svc.CreateSource(ctx, "test", "git", "https://example.com/x.git", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveSource(ctx, src.ID, "admin2"))
	_, err = svc.Install(ctx, service.PluginInstallInput{
		PluginID: "analytics", SourceID: src.ID, Manifest: validManifestJSON,
	})
	require.NoError(t, err)

	var inst model.PluginInstall
	require.NoError(t, store.SQL().DB().Where("plugin_id = ?", "analytics").First(&inst).Error)
	require.Equal(t, model.PluginStatusInstalled, inst.Status,
		"没有 runtime 时不该虚报 running；登记仍成功，由后续接线补拉起")
}

// 元数据编辑（Update）不得改动 enabled：启停只能走 SetEnabled，否则会
// 出现「库里的 enabled 变了、进程还在跑」的不一致，而且漏传字段的客户端
// 会把插件静默禁用（Go 零值 false）。
func TestUpdateDoesNotChangeEnabled(t *testing.T) {
	svc, rt, store := managedPlugin(t)
	ctx := context.Background()

	// 初始：装好即启用、进程 running。
	var before model.Plugin
	require.NoError(t, store.SQL().DB().Where("id = ?", "analytics").First(&before).Error)
	require.True(t, before.Enabled)

	// 一次不带 enabled 的元数据编辑（零值 false）：不得把插件静默禁用。
	require.NoError(t, svc.Update(ctx, service.PluginInput{ID: "analytics", Name: "Renamed"}))

	var row model.Plugin
	require.NoError(t, store.SQL().DB().Where("id = ?", "analytics").First(&row).Error)
	require.Equal(t, "Renamed", row.Name)
	require.True(t, row.Enabled, "Update 不得因零值把已启用的插件静默禁用")
	require.Equal(t, 0, len(rt.stopped), "Update 不得停止进程")
}

// 管理动作的审计必须记录**真实操作人**，而不是写死 "system"：事后追责与
// 「谁在什么时候装了什么」的查询都依赖 actor_id。
func TestAdminActionsAuditRealActor(t *testing.T) {
	svc, _, store := managedPlugin(t)
	ctx := context.Background()
	src, err := svc.CreateSource(ctx, "s2", "git", "https://example.com/y.git", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveSource(ctx, src.ID, "admin"))

	_, err = svc.InstallAs(ctx, service.PluginInstallInput{
		PluginID: "analytics2", SourceID: src.ID, Manifest: validManifestJSON,
	}, "alice")
	require.NoError(t, err)
	require.NoError(t, svc.SetEnabledAs(ctx, "analytics2", false, "bob"))
	require.NoError(t, svc.UninstallAs(ctx, "analytics2", "carol"))

	var logs []model.AuditLog
	require.NoError(t, store.SQL().DB().Where("action IN ?", []string{
		"plugin.install", "plugin.uninstall",
	}).Order("created_at").Find(&logs).Error)

	actors := map[string]string{}
	for _, l := range logs {
		actors[l.Action+":"+l.Target] = l.ActorID
	}
	require.Equal(t, "alice", actors["plugin.install:analytics2"], "安装审计必须记真实操作人")
	require.Equal(t, "carol", actors["plugin.uninstall:analytics2"], "卸载审计必须记真实操作人")
}

// 每次安装/升级都要留下历史快照，回滚才能从历史里取。
func TestInstallRecordsVersionHistory(t *testing.T) {
	svc, _, _ := managedPlugin(t)
	ctx := context.Background()

	versions, err := svc.ListVersions(ctx, "analytics")
	require.NoError(t, err)
	require.Len(t, versions, 1, "首次安装应留下一条历史")
	require.Equal(t, "install", versions[0].Note)
	require.Equal(t, "1.0.0", versions[0].Version)
	require.NotEmpty(t, versions[0].Manifest)
	require.Equal(t, "system", versions[0].ActorID)
}

// 升级（同 ID 再装）追加历史并标记为 upgrade。
func TestReinstallMarksUpgrade(t *testing.T) {
	svc, _, _ := managedPlugin(t)
	ctx := context.Background()

	src, err := svc.CreateSource(ctx, "s3", "git", "https://example.com/z.git", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveSource(ctx, src.ID, "admin"))

	_, err = svc.Install(ctx, service.PluginInstallInput{
		PluginID: "analytics", SourceID: src.ID, Manifest: validManifestJSON,
	})
	require.NoError(t, err)

	versions, err := svc.ListVersions(ctx, "analytics")
	require.NoError(t, err)
	require.Len(t, versions, 2)
	require.Equal(t, "upgrade", versions[0].Note, "最新一条是升级")
}

// 回滚到指定历史版本必须成功并再落一条 rollback 历史。
func TestRollbackToVersion(t *testing.T) {
	svc, _, _ := managedPlugin(t)
	ctx := context.Background()
	src, err := svc.CreateSource(ctx, "s4", "git", "https://example.com/w.git", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveSource(ctx, src.ID, "admin"))
	_, err = svc.Install(ctx, service.PluginInstallInput{
		PluginID: "analytics", SourceID: src.ID, Manifest: validManifestJSON,
	})
	require.NoError(t, err)

	versions, err := svc.ListVersions(ctx, "analytics")
	require.NoError(t, err)
	require.Len(t, versions, 2)
	oldest := versions[len(versions)-1]

	require.NoError(t, svc.RollbackAs(ctx, "analytics", oldest.ID, "dave"))

	after, err := svc.ListVersions(ctx, "analytics")
	require.NoError(t, err)
	// 回滚本身是一次重装（upgrade 记录）+ 一条显式的 rollback 标记。
	require.Len(t, after, 4)
	require.Equal(t, "rollback", after[0].Note)
	require.Equal(t, "dave", after[0].ActorID)
}

// 预览必须返回能力清单、依赖清单与「是否已安装」，且不落库。
func TestPreviewReturnsCapsAndDeps(t *testing.T) {
	svc, store := newPluginTestSvc(t)
	ff := &fakeFetcher{}
	svc.SetFetcher(ff)
	ctx := context.Background()
	src, err := svc.CreateSource(ctx, "s", "git", "https://example.com/p.git", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveSource(ctx, src.ID, "admin"))

	prev, err := svc.Preview(ctx, service.PluginInstallInput{
		PluginID: "previewme", SourceID: src.ID, Manifest: validManifestJSON,
	})
	require.NoError(t, err)
	require.Equal(t, "previewme", prev.PluginID)
	require.Equal(t, []string{"posts:write"}, prev.Capabilities)
	require.False(t, prev.AlreadyInstalled)
	require.Equal(t, "https://example.com/p.git", prev.SourceURL)

	var plugins []model.Plugin
	require.NoError(t, store.SQL().DB().Find(&plugins).Error)
	require.Empty(t, plugins, "预览不得落库")
}

// 未审批来源不可预览（与安装同一信任门）。
func TestPreviewRejectsUnapprovedSource(t *testing.T) {
	svc, _ := newPluginTestSvc(t)
	ctx := context.Background()
	src, err := svc.CreateSource(ctx, "s", "git", "https://example.com/p.git", "admin")
	require.NoError(t, err)

	_, err = svc.Preview(ctx, service.PluginInstallInput{
		PluginID: "previewme", SourceID: src.ID, Manifest: validManifestJSON,
	})
	require.Error(t, err)
}
