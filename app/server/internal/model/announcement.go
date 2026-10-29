package model

import (
	"time"

	"gorm.io/gorm"
)

// Announcement 公告表
type Announcement struct {
	ID          string         `gorm:"primaryKey" json:"id"`
	Title       string         `gorm:"size:200;not null" json:"title"`
	Content     string         `gorm:"type:text;not null" json:"content"` // markdown 格式
	AuthorID    string         `gorm:"index;not null" json:"author_id"`
	Status      string         `gorm:"size:16;default:draft" json:"status"` // draft / published
	DisplayType int            `gorm:"default:1" json:"display_type"`       // 1=toast 2=alert 3=modal
	Position    string         `gorm:"size:16;default:top-right" json:"position"`
	IsPinned    bool           `gorm:"default:false" json:"is_pinned"`
	Sort        int            `gorm:"default:0" json:"sort"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
	ViewCount   int            `gorm:"default:0" json:"view_count"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggertype:"string"`
}

func (Announcement) TableName() string { return "announcements" }

// 公告状态
const (
	AnnStatusDraft     = "draft"
	AnnStatusPublished = "published"
)

// 公告显示组件类型枚举
const (
	DisplayTypeToast  int = 1 // toast 短暂提示
	DisplayTypeAlert  int = 2 // alert 条幅告警
	DisplayTypeModal  int = 3 // modal 弹窗
	DisplayTypeIframe int = 4 // iframe 嵌入（content 字段填外部 URL）
)

// IsValidDisplayType 校验显示类型是否合法
func IsValidDisplayType(t int) bool {
	return t >= DisplayTypeToast && t <= DisplayTypeIframe
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
