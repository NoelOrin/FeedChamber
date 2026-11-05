package service

import (
	"context"

	"feedchamber/internal/model"
	"feedchamber/internal/plugin"
)

type PluginInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Description string `json:"description"`
	Homepage    string `json:"homepage"`
	Settings    string `json:"settings"`
	Enabled     bool   `json:"enabled"`
}

// PluginInstallInput 是一次安装请求。
//
// Manifest 是 plugin.json 的**原文**，不是已解析的结构：安装记录必须保留
// 原始文本（审计与「装的是什么」的可查性），解析结果只是派生。
type PluginInstallInput struct {
	PluginID string `json:"plugin_id"`
	SourceID string `json:"source_id"`
	Manifest string `json:"manifest"`
	// Dir 是插件代码在磁盘上的绝对路径。留空时由 service 按插件根目录
	// 拼出 <root>/<pluginID>。
	Dir string `json:"dir"`
}

// ResumablePlugin 是宿主重启时需要恢复的插件。
//
// Manifest 给的是原文而不是 *plugin.Manifest：调用方（container）要重新
// 解析一遍才能拿到钩子与 entry，这样「装的时候校验过」与「起的时候用到的」
// 是同一份输入，不会因为结构体序列化差异而漂移。
// PluginPreview 是安装前的确认信息：能力清单与依赖清单。
// 管理员必须在真正落盘/起进程之前看到这两份，否则「装的是什么」无从判断。
type PluginPreview struct {
	PluginID     string   `json:"plugin_id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	ProtocolVer  string   `json:"protocol_version"`
	Entry        string   `json:"entry"`
	Hooks        []string `json:"hooks"`
	Capabilities []string `json:"capabilities"`
	// Dependencies 来自 package.json 的 dependencies（name@range）。
	Dependencies []string `json:"dependencies"`
	// SourceURL 是被审批来源的地址。
	SourceURL string `json:"source_url"`
	// AlreadyInstalled 表示同 ID 已存在（本次将是升级而非全新安装）。
	AlreadyInstalled bool `json:"already_installed"`
}

type ResumablePlugin struct {
	PluginID     string   `json:"plugin_id"`
	Manifest     string   `json:"manifest"`
	Capabilities []string `json:"capabilities"`
	Dir          string   `json:"dir"`
}

// PluginRuntime 是 PluginService 驱动子进程宿主所需的窄接口。
//
// 用窄接口而不是直接依赖 *plugin.Host：service 层只该知道「拉起 / 停止 /
// 是否在跑」，不该知道 supervisor、Conn 或 stdio 协议。方法签名与
// plugin.Host 完全对齐，container 直接把 Host 传进来即可。
//
// 重要：plugin.Host.StartPlugin 内部用 exec.CommandContext(ctx) 管理子进程，
// 因此适配到本接口时 MUST 使用应用生命周期的 context（container 里是
// context.Background()），绝不能把 HTTP 请求的 ctx 透进来——请求一结束
// 子进程就会被杀掉。
type PluginRuntime interface {
	StartPlugin(id string, m *plugin.Manifest, caps plugin.CapabilitySet) error
	StopPlugin(id string) error
	Running(id string) bool
}

type PluginService interface {
	// 既有声明式 CRUD（保留）
	List(ctx context.Context) ([]model.Plugin, error)
	Update(ctx context.Context, input PluginInput) error
	SetEnabled(ctx context.Context, id string, enabled bool) error
	Delete(ctx context.Context, id string) error

	// SetRuntime 注入子进程宿主。未注入时管理动作退化为纯元数据登记
	//（测试与「只查库不起进程」场景），但绝不虚报 running 状态。
	SetRuntime(rt PluginRuntime)
	// HasRuntime 报告宿主是否已接线，供 composition root 装配断言用。
	HasRuntime() bool
	// SetFetcher 注入代码拉取/依赖安装实现（git + npm）。未注入时 Install
	// 只登记不拉取代码（测试与「代码已手工就位」场景）。
	SetFetcher(f PluginFetcher)

	// 安装前预览（能力 + 依赖清单，不落库）
	Preview(ctx context.Context, in PluginInstallInput) (*PluginPreview, error)

	// 升级与历史
	ListVersions(ctx context.Context, pluginID string) ([]model.PluginVersion, error)
	RollbackAs(ctx context.Context, pluginID, versionID, actorID string) error

	// 来源与审批
	CreateSource(ctx context.Context, name, kind, url, actorID string) (*model.PluginSource, error)
	ListSources(ctx context.Context) ([]model.PluginSource, error)
	ApproveSource(ctx context.Context, id, actorID string) error
	RejectSource(ctx context.Context, id, actorID string) error

	// 安装与运行时
	Install(ctx context.Context, in PluginInstallInput) (*model.PluginInstall, error)
	// InstallAs / UninstallAs / SetEnabledAs / UpdateAs / DeleteAs 带真实
	// 操作人，供 handler 透传 currentUserID；无 actor 的版本保留给内部/测试
	// 调用（审计落 "system"）。
	InstallAs(ctx context.Context, in PluginInstallInput, actorID string) (*model.PluginInstall, error)
	Uninstall(ctx context.Context, pluginID string) error
	UninstallAs(ctx context.Context, pluginID, actorID string) error
	SetEnabledAs(ctx context.Context, id string, enabled bool, actorID string) error
	UpdateAs(ctx context.Context, input PluginInput, actorID string) error
	DeleteAs(ctx context.Context, id, actorID string) error
	ListInstalls(ctx context.Context) ([]model.PluginInstall, error)
	SetRuntimeStatus(ctx context.Context, pluginID, status, lastError string) error
	ListResumable(ctx context.Context) ([]ResumablePlugin, error)
}
