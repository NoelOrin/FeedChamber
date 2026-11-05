package model

import "time"

// 壳层插槽枚举。
//
// 固定集合的原因：前端只认这些名字，未知名字被忽略。让插槽名自由化
// 等于让前端无法预渲染，也失去了「哪里能插」的契约。
const (
	SlotSidebarTop          = "sidebar.top"
	SlotSidebarNavAfter     = "sidebar.nav.after"
	SlotSidebarSpacesBefore = "sidebar.spaces.before"
	SlotSidebarSpacesAfter  = "sidebar.spaces.after"
	SlotSidebarTagsAfter    = "sidebar.tags.after"
	SlotSidebarBottom       = "sidebar.bottom"
	SlotHeaderLeft          = "header.left"
	SlotHeaderRightBefore   = "header.right.before"
	SlotHeaderRightAfter    = "header.right.after"
	SlotFooterLeft          = "footer.left"
	SlotFooterRight         = "footer.right"
)

// AllSlots 是全部合法插槽名，顺序与前端枚举一致。
var AllSlots = []string{
	SlotSidebarTop, SlotSidebarNavAfter, SlotSidebarSpacesBefore,
	SlotSidebarSpacesAfter, SlotSidebarTagsAfter, SlotSidebarBottom,
	SlotHeaderLeft, SlotHeaderRightBefore, SlotHeaderRightAfter,
	SlotFooterLeft, SlotFooterRight,
}

// ValidSlot 报告 s 是否为合法插槽名。
func ValidSlot(s string) bool {
	for _, v := range AllSlots {
		if v == s {
			return true
		}
	}
	return false
}

// 条目类型。刻意不含 html：插槽是同源位置，
// 允许任意 HTML 等于允许全站 XSS（见 shell-slots 规范）。
const (
	SlotKindText    = "text"
	SlotKindLink    = "link"
	SlotKindImage   = "image"
	SlotKindDivider = "divider"
	SlotKindWidget  = "widget"
)

// SlotKinds 是全部合法条目类型。
var SlotKinds = []string{SlotKindText, SlotKindLink, SlotKindImage, SlotKindDivider, SlotKindWidget}

// ValidSlotKind 报告 k 是否为合法条目类型。
func ValidSlotKind(k string) bool {
	for _, v := range SlotKinds {
		if v == k {
			return true
		}
	}
	return false
}

// SlotEntry 是插槽里的一条内容。
type SlotEntry struct {
	ID   string `gorm:"primaryKey;size:36" json:"id"`
	Slot string `gorm:"size:32;not null;index" json:"slot"`
	Kind string `gorm:"size:16;not null" json:"kind"`
	// Props 是 JSON 对象，形状由 Kind 决定。
	// 不用多列存是因为五种类型的字段互不重叠，多列会留下大量 NULL。
	Props string `gorm:"type:text" json:"props"`
	Sort  int    `gorm:"not null;default:0;index" json:"sort"`

	Visibility string `gorm:"size:16;not null;default:'public'" json:"visibility"`
	MinLevel   int    `gorm:"not null;default:0" json:"min_level"`

	// Enabled 刻意**不带** gorm default：带 default:true 时，Create 传入 false
	// 会被 gorm 用默认值覆盖，症状是「新建时明明关了却仍是启用」。
	// 默认启用由 service 层在入参缺省时给出，不由 DB 默认值兜底。
	Enabled bool `gorm:"not null;index" json:"enabled"`
	// OwnerPluginID 非空表示该条目由插件写入，用于卸载时清理与审计归属。
	OwnerPluginID string    `gorm:"size:64;index" json:"owner_plugin_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (SlotEntry) TableName() string { return "slot_entries" }

// VisibleTo 判定某个访问者能否看到该条目。语义与 NavItem.VisibleTo 一致。
func (e SlotEntry) VisibleTo(isLogin bool, level int) bool {
	if !e.Enabled {
		return false
	}
	switch e.Visibility {
	case NavVisibilityPublic:
		return true
	case NavVisibilityLogin:
		return isLogin
	case NavVisibilityLevel:
		// 等级门槛隐含要求登录：游客是 TL0，MinLevel>0 时自然被挡。
		return level >= e.MinLevel
	}
	// 非法可见性按「不可见」处理，避免配置写错导致意外公开。
	return false
}
