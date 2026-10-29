package model

import "time"

// token 覆盖的作用模式。all 同时作用于明亮与暗色；light / dark 只作用于其一。
const (
	ThemeTokenModeAll   = "all"
	ThemeTokenModeLight = "light"
	ThemeTokenModeDark  = "dark"
)

// ThemeToken 是一条主题 token 覆盖，语义等价于覆盖 packages/tokens 产出的
// CSS 变量。以「token 名 → 值」的结构化形式存储，站长不需要写选择器或 CSS。
type ThemeToken struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	Token     string    `gorm:"size:128;not null;index" json:"token"`
	Value     string    `gorm:"size:512;not null" json:"value"`
	Mode      string    `gorm:"size:16;not null;default:'all'" json:"mode"`
	UpdatedBy string    `gorm:"size:36" json:"updated_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ThemeToken) TableName() string { return "theme_tokens" }

// ValidThemeTokenMode 校验 mode 取值。写入侧用它拒绝非法值，
// 读取侧遇到非法值应按 all 处理而不是丢弃整条覆盖。
func ValidThemeTokenMode(mode string) bool {
	switch mode {
	case ThemeTokenModeAll, ThemeTokenModeLight, ThemeTokenModeDark:
		return true
	}
	return false
}
