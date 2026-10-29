package model

import "time"

// 首页模块类型。
const (
	HomeModuleLatest = "latest" // 最新帖子列表
	HomeModuleHot    = "hot"    // 热门帖子列表
	HomeModuleSpaces = "spaces" // 版面入口宫格
	HomeModuleHTML   = "html"   // 站长自定义 HTML（高风险，同 snippet 的 js/html 语义）
)

// HomeModule 是首页的一个可配置区块。
//
// Config 为模块类型专属的 JSON 参数（如 latest 的条数上限），
// 解析失败时按该类型的默认参数渲染，而不是让整个首页报错。
type HomeModule struct {
	ID         string    `gorm:"primaryKey;size:36" json:"id"`
	Kind       string    `gorm:"size:32;not null;index" json:"kind"`
	Title      string    `gorm:"size:128" json:"title"`
	Sort       int       `gorm:"not null;default:0;index" json:"sort"`
	Visibility string    `gorm:"size:16;not null;default:'public'" json:"visibility"`
	MinLevel   int       `gorm:"not null;default:0" json:"min_level"`
	Config     string    `gorm:"type:text" json:"config"`
	Enabled    bool      `gorm:"not null;default:true;index" json:"enabled"`
	UpdatedBy  string    `gorm:"size:36" json:"updated_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (HomeModule) TableName() string { return "home_modules" }

// VisibleTo 与 NavItem 同语义：游客 level=0。
func (m HomeModule) VisibleTo(isLogin bool, level int) bool {
	if !m.Enabled {
		return false
	}
	switch m.Visibility {
	case NavVisibilityPublic:
		return true
	case NavVisibilityLogin:
		return isLogin
	case NavVisibilityLevel:
		return level >= m.MinLevel
	}
	return false
}

// IsHighRisk 判定该模块是否会执行站长写入的任意内容。
// html 模块等价于一段可执行标记，机器凭证写入时必须走草稿。
func (m HomeModule) IsHighRisk() bool {
	return m.Kind == HomeModuleHTML
}

// ValidHomeModuleKind 校验模块类型。
func ValidHomeModuleKind(k string) bool {
	switch k {
	case HomeModuleLatest, HomeModuleHot, HomeModuleSpaces, HomeModuleHTML:
		return true
	}
	return false
}
