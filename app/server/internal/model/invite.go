package model

import (
	"time"

	"orinforum/internal/pkg"
)

// Invite 注册邀请码。
// MaxUses=0 表示无限次使用；ExpiresAt=nil 表示不过期。
type Invite struct {
	ID        string     `gorm:"primaryKey;size:26" json:"id"`
	Code      string     `gorm:"uniqueIndex;size:32;not null" json:"code"`
	CreatorID string     `gorm:"index;size:26" json:"creator_id"`
	MaxUses   int        `gorm:"default:0" json:"max_uses"` // 0=无限
	UsedCount int        `gorm:"default:0" json:"used_count"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func (Invite) TableName() string { return "invites" }

// NewInvite 构造邀请码记录。
func NewInvite(code, creatorID string, maxUses int, expiresAt *time.Time) *Invite {
	return &Invite{
		ID:        pkg.NewID(),
		Code:      code,
		CreatorID: creatorID,
		MaxUses:   maxUses,
		UsedCount: 0,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
}
