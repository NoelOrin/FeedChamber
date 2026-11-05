package container

import (
	"testing"

	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/pkg/testnats"
	"feedchamber/internal/repository"
	"feedchamber/internal/service"

	"github.com/stretchr/testify/require"
)

// 本文件钉住「插件宿主与管理服务在 composition root 真正接线」。
//
// 两条最容易静默失效的装配：
//   - HostOptions.APIDepsFor 没设 → host.go 里 if APIDepsFor != nil 恒为假，
//     9 个宿主 API 全是空壳，插件声明了能力也调不通任何业务；
//   - PluginService 没 SetRuntime → 后台装/启/停/删只写库，进程永不起停。
//
// 两者都不会让编译或现有测试变红，所以必须有专门的装配断言。

func newTestContainer(t *testing.T) *Container {
	t.Helper()
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: ":memory:"})
	require.NoError(t, err)
	kv := testnats.NewKVStore(t, "container_plugin_wiring_test")
	bus := testnats.NewEventBus(t)

	permSvc := service.NewPermissionService(
		repository.NewPermissionRepository(store),
		repository.NewSpaceRolePermissionRepository(store),
		repository.NewGroupRepository(store),
		kv,
	)
	settingSvc := service.NewSettingService(repository.NewSettingRepository(store), kv)
	bannedSvc := service.NewBannedService(repository.NewBannedRepository(store), kv, db.NewTransactionManager(store))
	spaceSvc := service.NewSpaceService(
		repository.NewSpaceRepository(store),
		repository.NewPostRepository(store),
		db.NewTransactionManager(store),
		kv,
	)

	return New(Deps{
		Config:     config.Config{},
		Store:      store,
		KV:         kv,
		EventBus:   bus,
		Permission: permSvc,
		Setting:    settingSvc,
		Banned:     bannedSvc,
		Space:      spaceSvc,
		Sessions:   &recordingSessionStore{},
	})
}

func TestNew_WiresPluginHostRuntime(t *testing.T) {
	c := newTestContainer(t)

	require.True(t, c.PluginHost.HasAPIDeps(),
		"Host 必须有 APIDepsFor，否则宿主的 9 个 api.* 方法全是空壳")
	require.True(t, c.PluginSvc.HasRuntime(),
		"PluginService 必须注入 runtime，否则后台管理动作驱动不了子进程")
}

// 七个接缝必须全部非空：任何一个为 nil，对应 capability 的 API 都会返回
// "host API not wired"，而管理员在安装确认页仍会看到那条能力。
func TestPluginAPIDeps_AllSeamsWired(t *testing.T) {
	c := newTestContainer(t)
	deps := newPluginAPIDeps(c, "diag", nil)

	require.NotNil(t, deps.Posts, "posts.*")
	require.NotNil(t, deps.Comments, "comments.*")
	require.NotNil(t, deps.Users, "users.*")
	require.NotNil(t, deps.Notifications, "notifications.send")
	require.NotNil(t, deps.KV, "kv.*")
	require.NotNil(t, deps.Slots, "slots.*")
	require.NotNil(t, deps.Audit, "写类 API 审计")
}
