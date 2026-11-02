package container

import (
	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/mcp"
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
	// MCP 是站点对外暴露的 MCP Server（P2）。为 nil 时 /mcp 不注册。
	//
	// 由 composition root 构造后注入：它的 Enforcer 需要 gateway 用的同一套
	// SettingReader / PermissionChecker / BannedChecker，以及限流引擎。
	MCP MCPComponents
}

// MCPComponents 是 MCP Server 的装配结果。
//
// 拆成字段而不是整包传 *mcp.Server：container 只需要「协议入口」与
// 「工具注册表」两样，注册表还要给后台管理页读工具状态。
type MCPComponents struct {
	Handler  *mcp.Handler
	Registry *mcp.Registry
}
