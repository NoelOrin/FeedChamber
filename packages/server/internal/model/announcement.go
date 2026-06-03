package model

import "time"

// Announcement 公告表
type Announcement struct {
	ID          string     `gorm:"primaryKey" json:"id" bson:"_id"`
	Title       string     `gorm:"size:200;not null" json:"title" bson:"title"`
	Content     string     `gorm:"type:text;not null" json:"content" bson:"content"` // markdown 格式
	AuthorID    string     `gorm:"index;not null" json:"author_id" bson:"author_id"`
	Status      string     `gorm:"size:16;default:draft" json:"status" bson:"status"` // draft / published
	DisplayType int        `gorm:"default:1" json:"display_type" bson:"display_type"` // 1=toast 2=alert 3=modal
	Position    string     `gorm:"size:16;default:top-right" json:"position" bson:"position"`
	IsPinned    bool       `gorm:"default:false" json:"is_pinned" bson:"is_pinned"`
	Sort        int        `gorm:"default:0" json:"sort" bson:"sort"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty" bson:"expires_at,omitempty"`
	ViewCount   int        `gorm:"default:0" json:"view_count" bson:"view_count"`
	CreatedAt   time.Time  `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" bson:"updated_at"`
}

func (Announcement) TableName() string { return "announcements" }

// 公告状态
const (
	AnnStatusDraft     = "draft"
	AnnStatusPublished = "published"
)

// 公告显示组件类型枚举
const (
	DisplayTypeToast int = 1 // toast 短暂提示
	DisplayTypeAlert int = 2 // alert 条幅告警
	DisplayTypeModal int = 3 // modal 弹窗
)

// IsValidDisplayType 校验显示类型是否合法
func IsValidDisplayType(t int) bool {
	return t >= DisplayTypeToast && t <= DisplayTypeModal
}

// toast 位置枚举
const (
	PositionTopLeft      string = "top-left"
	PositionTopRight     string = "top-right"
	PositionTopCenter    string = "top-center"
	PositionBottomLeft   string = "bottom-left"
	PositionBottomRight  string = "bottom-right"
	PositionBottomCenter string = "bottom-center"
)

// DefaultPosition 默认位置
const DefaultPosition = PositionTopRight

// IsValidPosition 校验位置是否合法
func IsValidPosition(p string) bool {
	switch p {
	case PositionTopLeft, PositionTopRight, PositionTopCenter,
		PositionBottomLeft, PositionBottomRight, PositionBottomCenter:
		return true
	}
	return false
}
