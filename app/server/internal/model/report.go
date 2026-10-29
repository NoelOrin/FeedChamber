package model

import "time"

type Report struct {
	ID          string     `gorm:"primaryKey" json:"id"`
	ReporterID  string     `gorm:"index;not null" json:"reporter_id"`
	TargetType  string     `gorm:"size:16;not null" json:"target_type"`
	TargetID    string     `gorm:"index;not null" json:"target_id"`
	FlagType    string     `gorm:"size:64;index" json:"flag_type"`
	Reason      string     `gorm:"size:500;not null" json:"reason"`
	TargetTitle string     `gorm:"size:255" json:"target_title"`
	TargetBody  string     `gorm:"type:text" json:"target_body"`
	Count       int        `gorm:"default:1" json:"count"`
	Status      string     `gorm:"size:16;default:pending" json:"status"`
	Action      string     `gorm:"size:32" json:"action,omitempty"`
	HandledBy   string     `gorm:"size:64" json:"handled_by,omitempty"`
	HandledAt   *time.Time `json:"handled_at,omitempty"`
	Note        string     `gorm:"type:text" json:"note,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (Report) TableName() string { return "reports" }
