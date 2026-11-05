package service_test

import (
	"context"
	"path/filepath"
	"testing"

	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/repository"
	"feedchamber/internal/service"

	"github.com/stretchr/testify/require"
)

// 本文件钉住插件安装与来源审批的信任模型第一层：
// 未审批的来源不可安装、未知 capability 必须报错、entry 逃逸必须拒绝。
// 三者都是「默认放行就会静默越权」的判定，因此每条都单独一个测试。

const validManifestJSON = `{
	"name":"hello","version":"1.0.0","protocol_version":"1",
	"hooks":["post.created"],"capabilities":["posts:write"]
}`

// newPluginTestSvc 用真实 SQLite 建 service。
//
// 刻意不用 :memory:：sqlite 的 :memory: 是「每个连接一个库」，而 gorm 的连接池
// 会开多条连接，AutoMigrate 建的表在后续查询的连接上可能不存在——症状是
// 偶发 "no such table"。临时文件库与生产形态一致，且 t.TempDir 自动清理。
func newPluginTestSvc(t *testing.T) (service.PluginService, db.Store) {
	t.Helper()
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "plugin.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(
		&model.Plugin{}, &model.PluginSource{}, &model.PluginInstall{}, &model.AuditLog{},
	))
	svc := service.NewPluginService(
		repository.NewPluginRepository(store),
		repository.NewPluginSourceRepository(store),
		repository.NewPluginInstallRepository(store),
		service.NewAuditLogService(repository.NewAuditLogRepository(store)),
	)
	return svc, store
}

// 未审批来源不可安装。
func TestInstallRejectsUnapprovedSource(t *testing.T) {
	svc, _ := newPluginTestSvc(t)
	ctx := context.Background()

	src, err := svc.CreateSource(ctx, "test", "git", "https://example.com/x.git", "admin")
	require.NoError(t, err)
	require.Equal(t, model.PluginSourcePending, src.Status)

	_, err = svc.Install(ctx, service.PluginInstallInput{
		PluginID: "hello", SourceID: src.ID, Manifest: validManifestJSON,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "审批")
}

// 审批后可以安装。
func TestInstallAcceptsApprovedSource(t *testing.T) {
	svc, _ := newPluginTestSvc(t)
	ctx := context.Background()

	src, err := svc.CreateSource(ctx, "test", "git", "https://example.com/x.git", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveSource(ctx, src.ID, "admin2"))

	inst, err := svc.Install(ctx, service.PluginInstallInput{
		PluginID: "hello", SourceID: src.ID, Manifest: validManifestJSON,
	})
	require.NoError(t, err)
	require.Equal(t, model.PluginStatusInstalled, inst.Status)
	require.Equal(t, "hello", inst.PluginID)
	// capability 以授予时为准落库。
	require.JSONEq(t, `["posts:write"]`, inst.Caps)
}

// 已拒绝的来源同样不可安装——「非 approved」的一律拒绝，不枚举。
func TestInstallRejectsRejectedSource(t *testing.T) {
	svc, _ := newPluginTestSvc(t)
	ctx := context.Background()

	src, err := svc.CreateSource(ctx, "test", "git", "https://example.com/x.git", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.RejectSource(ctx, src.ID, "admin2"))

	_, err = svc.Install(ctx, service.PluginInstallInput{
		PluginID: "hello", SourceID: src.ID, Manifest: validManifestJSON,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "审批")
}

// 未知 capability 必须让安装失败。
//
// 这条很容易被写错：CapabilitySetFromNames 会静默丢弃未知名字，若实现
// 「先转换再校验」且把转换结果当作 knownCaps 传下去，未知 capability 就
// 静默通过了。因此必须校验「manifest 的能力 ⊆ 全量已知能力」。
func TestInstallRejectsUnknownCapability(t *testing.T) {
	svc, _ := newPluginTestSvc(t)
	ctx := context.Background()

	src, err := svc.CreateSource(ctx, "test", "git", "https://example.com/x.git", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveSource(ctx, src.ID, "admin2"))

	bad := `{
		"name":"evil","version":"1.0.0","protocol_version":"1",
		"hooks":["post.created"],"capabilities":["posts:write",".*:write"]
	}`
	_, err = svc.Install(ctx, service.PluginInstallInput{
		PluginID: "evil", SourceID: src.ID, Manifest: bad,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "capability")
}

// entry 逃逸必须被拒绝（ParseManifest 的 validateEntry 是唯一防线，
// 这里从 service 入口做端到端断言，防止将来有人把校验绕过去）。
func TestInstallRejectsEscapingEntry(t *testing.T) {
	svc, store := newPluginTestSvc(t)
	ctx := context.Background()

	src, err := svc.CreateSource(ctx, "test", "git", "https://example.com/x.git", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveSource(ctx, src.ID, "admin2"))

	bad := `{
		"name":"evil","version":"1.0.0","protocol_version":"1",
		"hooks":["post.created"],"capabilities":[],
		"entry":"../../etc/passwd"
	}`
	_, err = svc.Install(ctx, service.PluginInstallInput{
		PluginID: "evil", SourceID: src.ID, Manifest: bad,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "entry")

	// 失败的安装不得留下半安装状态。
	var installs []model.PluginInstall
	require.NoError(t, store.SQL().DB().Find(&installs).Error)
	require.Empty(t, installs)
}

// 添加来源必须落审计，且审计里带操作人、URL 与 kind。
func TestCreateSourceIsAudited(t *testing.T) {
	svc, store := newPluginTestSvc(t)
	ctx := context.Background()

	src, err := svc.CreateSource(ctx, "acme", "git", "https://example.com/acme.git", "operator-1")
	require.NoError(t, err)

	var logs []model.AuditLog
	require.NoError(t, store.SQL().DB().Where("action = ?", "plugin.source.create").Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, "operator-1", logs[0].ActorID)
	require.Equal(t, src.ID, logs[0].Target)
	require.Contains(t, logs[0].Detail, "https://example.com/acme.git")
	require.Contains(t, logs[0].Detail, "git")

	// 审批也要留痕，并记下审批人。
	require.NoError(t, svc.ApproveSource(ctx, src.ID, "operator-2"))
	logs = nil
	require.NoError(t, store.SQL().DB().Where("action = ?", "plugin.source.approve").Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, "operator-2", logs[0].ActorID)
}

// 审批不存在的来源必须报错，不能静默成功（否则审计里会有假「已审批」记录）。
func TestApproveSourceRejectsUnknownID(t *testing.T) {
	svc, _ := newPluginTestSvc(t)

	err := svc.ApproveSource(context.Background(), "no-such-id", "admin")
	require.Error(t, err)
}

// SetRuntimeStatus 回写运行状态，且只动状态列。
func TestSetRuntimeStatus(t *testing.T) {
	svc, _ := newPluginTestSvc(t)
	ctx := context.Background()

	src, err := svc.CreateSource(ctx, "test", "git", "https://example.com/x.git", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveSource(ctx, src.ID, "admin2"))
	_, err = svc.Install(ctx, service.PluginInstallInput{
		PluginID: "hello", SourceID: src.ID, Manifest: validManifestJSON,
	})
	require.NoError(t, err)

	require.NoError(t, svc.SetRuntimeStatus(ctx, "hello", model.PluginStatusRunning, ""))

	resumable, err := svc.ListResumable(ctx)
	require.NoError(t, err)
	require.Len(t, resumable, 1)
	require.Equal(t, "hello", resumable[0].PluginID)

	// crashed 与 stopped 都不恢复。
	require.NoError(t, svc.SetRuntimeStatus(ctx, "hello", model.PluginStatusCrashed, "boom"))
	resumable, err = svc.ListResumable(ctx)
	require.NoError(t, err)
	require.Empty(t, resumable)

	// 非法状态值被拒绝。
	require.Error(t, svc.SetRuntimeStatus(ctx, "hello", "nonsense", ""))
}

// ListResumable 必须带回 manifest 原文、能力清单与目录，宿主据此才能拉起。
func TestListResumableCarriesManifestAndCapabilities(t *testing.T) {
	svc, _ := newPluginTestSvc(t)
	ctx := context.Background()

	src, err := svc.CreateSource(ctx, "test", "git", "https://example.com/x.git", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.ApproveSource(ctx, src.ID, "admin2"))
	_, err = svc.Install(ctx, service.PluginInstallInput{
		PluginID: "hello", SourceID: src.ID, Manifest: validManifestJSON,
		Dir: filepath.Join(t.TempDir(), "hello"),
	})
	require.NoError(t, err)
	require.NoError(t, svc.SetRuntimeStatus(ctx, "hello", model.PluginStatusRunning, ""))

	got, err := svc.ListResumable(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, validManifestJSON, got[0].Manifest)
	require.Equal(t, []string{"posts:write"}, got[0].Capabilities)
	require.NotEmpty(t, got[0].Dir)
}
