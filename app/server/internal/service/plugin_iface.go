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

	// 来源与审批
	CreateSource(ctx context.Context, name, kind, url, actorID string) (*model.PluginSource, error)
	ListSources(ctx context.Context) ([]model.PluginSource, error)
	ApproveSource(ctx context.Context, id, actorID string) error
	RejectSource(ctx context.Context, id, actorID string) error

	// 安装与运行时
	Install(ctx context.Context, in PluginInstallInput) (*model.PluginInstall, error)
	Uninstall(ctx context.Context, pluginID string) error
	ListInstalls(ctx context.Context) ([]model.PluginInstall, error)
	SetRuntimeStatus(ctx context.Context, pluginID, status, lastError string) error
	ListResumable(ctx context.Context) ([]ResumablePlugin, error)
}
