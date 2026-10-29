package model

import "time"

// AuditLog records sensitive admin operations for traceability.
type AuditLog struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	ActorID   string    `gorm:"index;not null" json:"actor_id"`
	Action    string    `gorm:"size:64;not null;index" json:"action"` // e.g. "permission.sync"
	Target    string    `gorm:"size:128;not null" json:"target"`      // e.g. role name
	Detail    string    `gorm:"type:text" json:"detail,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }
