package model

import "time"

// Credential stores the password hash separately from the User model.
// This ensures that password hashes are never accidentally exposed through
// User references (logs, heap dumps, JSON responses, etc.).
type Credential struct {
	UserID       string    `gorm:"primaryKey;size:36" json:"user_id"`
	PasswordHash string    `gorm:"not null" json:"-"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (c *Credential) TableName() string { return "credentials" }
