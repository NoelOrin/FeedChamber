package model

import "time"

// Revision stores an immutable snapshot of a post or comment body.
type Revision struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	TargetType string    `gorm:"size:16;index;not null" json:"target_type"` // post | comment
	TargetID   string    `gorm:"index;not null" json:"target_id"`
	Title      string    `gorm:"size:200" json:"title"`
	Body       string    `gorm:"type:text" json:"body"`
	AuthorID   string    `gorm:"index;not null" json:"author_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func (Revision) TableName() string { return "revisions" }
