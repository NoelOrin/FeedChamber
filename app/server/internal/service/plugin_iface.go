package service

import (
	"context"

	"feedchamber/internal/model"
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

type PluginService interface {
	// 既有声明式 CRUD（保留）
	List(ctx context.Context) ([]model.Plugin, error)
	Update(ctx context.Context, input PluginInput) error
	SetEnabled(ctx context.Context, id string, enabled bool) error
	Delete(ctx context.Context, id string) error

	// 来源与审批
	CreateSource(ctx context.Context, name, kind, url, actorID string) (*model.PluginSource, error)
	ListSources(ctx context.Context) ([]model.PluginSource, error)
	ApproveSource(ctx context.Context, id, actorID string) error
	RejectSource(ctx context.Context, id, actorID string) error

	// 安装与运行时
	Install(ctx context.Context, in PluginInstallInput) (*model.PluginInstall, error)
	Uninstall(ctx context.Context, pluginID string) error
	SetRuntimeStatus(ctx context.Context, pluginID, status, lastError string) error
	ListResumable(ctx context.Context) ([]ResumablePlugin, error)
}
