package model

import "time"

// UserBlock represents a block relationship: BlockerID blocks BlockedID.
type UserBlock struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	BlockerID string    `gorm:"uniqueIndex:idx_block;not null" json:"blocker_id"`
	BlockedID string    `gorm:"uniqueIndex:idx_block;not null" json:"blocked_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (UserBlock) TableName() string { return "user_blocks" }
