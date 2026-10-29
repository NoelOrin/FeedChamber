package model

import "time"

// UserMFA stores TOTP secret and backup codes for a user.
type UserMFA struct {
	UserID      string    `gorm:"primaryKey;size:36" json:"user_id"`
	TOTPSecret  string    `gorm:"size:64" json:"-"`
	Enabled     bool      `gorm:"default:false" json:"enabled"`
	BackupCodes string    `gorm:"type:text" json:"-"` // JSON array of bcrypt hashes
	UpdatedAt   time.Time `json:"updated_at"`
}

func (UserMFA) TableName() string { return "user_mfa" }
