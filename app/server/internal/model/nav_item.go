package model

import "time"

// 导航项可见性。
const (
	NavVisibilityPublic = "public" // 所有人可见（含游客）
	NavVisibilityLogin  = "login"  // 仅已登录用户
	NavVisibilityLevel  = "level"  // 指定等级及以上
)

// NavItem 是一条由配置驱动的导航项。
//
// 前台 nav.tsx 采用「配置优先，无配置回退硬编码」：nav_items 为空时渲染
// 既有默认导航，避免部署瞬间导航变空。
type NavItem struct {
	ID    string `gorm:"primaryKey;size:36" json:"id"`
	Label string `gorm:"size:128;not null" json:"label"`
	Href  string `gorm:"size:512;not null" json:"href"`
	Icon  string `gorm:"size:64" json:"icon"`
	Sort  int    `gorm:"not null;default:0;index" json:"sort"`
	// Visibility + MinLevel 决定谁能看到该项；MinLevel 只在 Visibility=level 时生效。
	Visibility string    `gorm:"size:16;not null;default:'public'" json:"visibility"`
	MinLevel   int       `gorm:"not null;default:0" json:"min_level"`
	Enabled    bool      `gorm:"not null;default:true;index" json:"enabled"`
	UpdatedBy  string    `gorm:"size:36" json:"updated_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (NavItem) TableName() string { return "nav_items" }

// VisibleTo 判定某个访问者能否看到该导航项。
// isLogin 表示已登录；level 是该用户的信任等级（游客传 0）。
func (n NavItem) VisibleTo(isLogin bool, level int) bool {
	if !n.Enabled {
		return false
	}
	switch n.Visibility {
	case NavVisibilityPublic:
		return true
	case NavVisibilityLogin:
		return isLogin
	case NavVisibilityLevel:
		// 等级门槛本身就隐含要求登录：游客是 TL0，MinLevel>0 时自然被挡。
		return level >= n.MinLevel
	}
	// 非法可见性按「不可见」处理，避免配置写错导致意外公开。
	return false
}

// ValidNavVisibility 校验可见性取值。
func ValidNavVisibility(v string) bool {
	switch v {
	case NavVisibilityPublic, NavVisibilityLogin, NavVisibilityLevel:
		return true
	}
	return false
}
