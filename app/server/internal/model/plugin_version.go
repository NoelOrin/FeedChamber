package model

import "time"

// PluginVersion 是一次安装/升级的历史快照。
//
// 与 PluginInstall 的分工：PluginInstall 是「当前这一份装在哪、什么状态」，
// PluginVersion 是「历次装过什么」——回滚就是从历史里再取一份装回去。
// Manifest 存原文（与 Plugin 一致），Checksum 用于判断回滚目标是否与当初一致。
type PluginVersion struct {
	ID       string `gorm:"primaryKey;size:36" json:"id"`
	PluginID string `gorm:"size:64;not null;index" json:"plugin_id"`
	Version  string `gorm:"size:32" json:"version"`
	SourceID string `gorm:"size:36;index" json:"source_id"`
	// Manifest 是装进来时的 plugin.json 原文。
	Manifest string `gorm:"type:text" json:"manifest"`
	// Caps 是当时授予的能力快照（JSON 数组）。
	Caps string `gorm:"type:text" json:"caps"`
	// Checksum 是当时插件目录内容的 sha256。
	Checksum string `gorm:"size:64" json:"checksum"`
	// ActorID 是执行这次安装/升级的人。
	ActorID string `gorm:"size:36" json:"actor_id"`
	// Note 标记来源：install / upgrade / rollback。
	Note      string    `gorm:"size:16" json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

func (PluginVersion) TableName() string { return "plugin_versions" }
