package model

import "time"

// 插件来源审批状态。
//
// 「来源审批」是信任模型的第一层：它挡的是「谁把代码放进来」，不是
// 「代码是否安全」——同用户子进程的边界问题见 proposal 的风险节。
// 未审批（pending）与已拒绝（rejected）一律不可安装。
const (
	PluginSourcePending  = "pending"
	PluginSourceApproved = "approved"
	PluginSourceRejected = "rejected"
)

// 插件来源类型。v1 只支持从受信任代码仓库拉取。
const (
	PluginSourceGit = "git"
)

// PluginSource 是一条插件来源登记。
type PluginSource struct {
	ID     string `gorm:"primaryKey;size:36" json:"id"`
	Name   string `gorm:"size:128;not null" json:"name"`
	Kind   string `gorm:"size:16;not null;default:'git'" json:"kind"`
	URL    string `gorm:"size:512;not null" json:"url"`
	Status string `gorm:"size:16;not null;default:'pending';index" json:"status"`
	// ApprovedBy 记录审批人；同一个人新增+审批时是同一个 id，
	// 这一列存在的意义是让审计能查出「自批」的情形。
	ApprovedBy string    `gorm:"size:36" json:"approved_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (PluginSource) TableName() string { return "plugin_sources" }

// ValidPluginSourceStatus 报告 s 是否是已知的审批状态。
func ValidPluginSourceStatus(s string) bool {
	return s == PluginSourcePending || s == PluginSourceApproved || s == PluginSourceRejected
}
