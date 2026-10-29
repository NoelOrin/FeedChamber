package model

import (
	"time"

	"gorm.io/gorm"
)

// Banned 禁言记录表（支持全局和版面级别）
type Banned struct {
	ID          string         `gorm:"primaryKey" json:"id"`
	UserID      string         `gorm:"index;not null" json:"user_id"`
	Scope       string         `gorm:"size:20;not null;default:'global';index" json:"scope"` // global | space
	SpaceID     string         `gorm:"index" json:"space_id,omitempty"`                      // 版面ID（scope=space时必填）
	Reason      string         `gorm:"size:500" json:"reason"`
	IsPermanent bool           `gorm:"default:false" json:"is_permanent"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
	CreatedBy   string         `gorm:"size:64" json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggertype:"string"`
}

func (Banned) TableName() string { return "banned" }

// IsExpired 检查禁言是否已过期
func (b *Banned) IsExpired() bool {
	if b.IsPermanent {
		return false
	}
	if b.ExpiresAt == nil {
		// 非永久且未设置过期时间 → 视为无期限（不过期）
		return false
	}
	return b.ExpiresAt.Before(time.Now())
}

// IsValid 检查禁言是否有效（未过期）
func (b *Banned) IsValid() bool {
	return !b.IsExpired()
}
