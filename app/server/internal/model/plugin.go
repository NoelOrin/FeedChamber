package model

import "time"

// Plugin is a plugin registration record.
//
// 两个阶段的历史叠在这里：
//   - v1 只存声明式元数据，从不执行插件代码；
//   - v2（当前）插件是 Node 子进程，Manifest 存 plugin.json 原文，
//     Hooks / Capabilities 是从中解析结果的 JSON 数组冗余列，
//     供后台列表直接展示而不用每次重解析。
//
// 注意 Capabilities 是「安装时授予」的快照，不是「当前 manifest 声明」——
// manifest 事后被改动不会自动扩权，改权必须重新走安装审批。
type Plugin struct {
	ID          string `gorm:"primaryKey;size:64" json:"id"`
	Name        string `gorm:"size:128;not null" json:"name"`
	Version     string `gorm:"size:32" json:"version"`
	Author      string `gorm:"size:128" json:"author"`
	Description string `gorm:"type:text" json:"description"`
	Homepage    string `gorm:"size:512" json:"homepage"`
	Settings    string `gorm:"type:text" json:"settings"`
	Enabled     bool   `gorm:"default:true" json:"enabled"`

	// v2：插件从「声明式元数据」变为可执行的子进程插件。
	// Manifest 存 plugin.json 原文，改动留痕可查。
	Manifest        string `gorm:"type:text" json:"manifest,omitempty"`
	ProtocolVersion string `gorm:"size:8" json:"protocol_version,omitempty"`
	Hooks           string `gorm:"type:text" json:"hooks,omitempty"`        // JSON 数组
	Capabilities    string `gorm:"type:text" json:"capabilities,omitempty"` // JSON 数组
	SourceID        string `gorm:"size:36;index" json:"source_id,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Plugin) TableName() string { return "plugins" }
