package container

import (
	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/pkg/kvstore"
	"feedchamber/internal/service"
	"feedchamber/internal/storage"
)

// Deps 是 container.New 的全部输入，替代此前 11 个位置参数。
//
// 其中四个 service（Permission / Setting / Banned / Space）与 Sessions、Security
// 在启动期由 server/iris.go 单独构造，因为它们同时被 middleware、gateway 和
// container 使用，必须只有一份实例（M-25 单一吊销/权限权威）。
type Deps struct {
	Config     config.Config
	Store      db.Store
	KV         kvstore.Store
	EventBus   *eventbus.EventBus
	Storage    storage.Provider
	Permission service.PermissionService
	Setting    service.SettingService
	Banned     service.BannedService
	Space      service.SpaceService
	// Sessions 是服务端会话权威：Login 经由它签发凭证，改角色/改密/封号经由它吊销。
	Sessions service.SessionStore
	// Security 是暴力破解防护管理器（internal/gateway.SecurityManager）。
	Security service.SecManager
}
