package model

import "time"

// Draft stores a server-side composer draft for a post or reply.
type Draft struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    string    `gorm:"index;not null" json:"user_id"`
	Kind      string    `gorm:"size:16;not null" json:"kind"` // post | reply
	TargetID  string    `json:"target_id,omitempty"`
	Title     string    `gorm:"size:200" json:"title"`
	Body      string    `gorm:"type:text" json:"body"`
	SpaceID   string    `json:"space_id,omitempty"`
	Tags      string    `gorm:"type:text" json:"tags"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Draft) TableName() string { return "drafts" }
