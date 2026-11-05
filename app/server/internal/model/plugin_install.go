package model

import "time"

// 插件状态机。
//
//	installed → starting → running
//	                 ↓        ↓
//	              crashed ← (崩溃/超限)
//	                 ↓
//	              stopped
//
// 宿主重启只恢复 running 的插件；crashed 与 stopped 都不自动拉起。
// 理由：崩溃的插件重启宿主后立刻再崩一次，会让「插件反复崩溃」变成
// 「宿主反复重启」的表象，反而更难定位。
const (
	PluginStatusInstalled = "installed"
	PluginStatusStarting  = "starting"
	PluginStatusRunning   = "running"
	PluginStatusCrashed   = "crashed"
	PluginStatusStopped   = "stopped"
)

var pluginStatuses = []string{
	PluginStatusInstalled, PluginStatusStarting, PluginStatusRunning,
	PluginStatusCrashed, PluginStatusStopped,
}

// ValidPluginStatus 报告 s 是否是状态机里的合法状态。
func ValidPluginStatus(s string) bool {
	for _, v := range pluginStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// PluginStatusShouldResume 决定宿主重启后是否拉起该插件。
func PluginStatusShouldResume(s string) bool {
	return s == PluginStatusRunning
}

// PluginInstall 是一次插件安装的运行时记录。
//
// 与 model.Plugin 的分工：Plugin 是「登记了什么」（清单、能力、来源），
// PluginInstall 是「这一份装在哪个目录、当前什么状态」。
// 分开是因为将来可能同一条 Plugin 多实例（多站点托管），而 v1 只是一对一。
type PluginInstall struct {
	ID       string `gorm:"primaryKey;size:36" json:"id"`
	PluginID string `gorm:"size:64;not null;uniqueIndex" json:"plugin_id"`
	SourceID string `gorm:"size:36;index" json:"source_id"`
	// Dir 是插件代码在磁盘上的绝对路径。
	Dir    string `gorm:"size:512;not null" json:"dir"`
	Entry  string `gorm:"size:128;not null;default:'index.js'" json:"entry"`
	Status string `gorm:"size:16;not null;default:'installed';index" json:"status"`
	// Caps 是已授予的 capability（JSON 数组）。以授予时为准，
	// 之后 manifest 变化不自动扩权——改权需重新审批。
	Caps          string     `gorm:"type:text" json:"caps"`
	Restarts      int        `gorm:"not null;default:0" json:"restarts"`
	LastError     string     `gorm:"type:text" json:"last_error"`
	LastStartedAt *time.Time `json:"last_started_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (PluginInstall) TableName() string { return "plugin_installs" }
