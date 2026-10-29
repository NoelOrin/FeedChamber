package model

// UserPreference stores per-user key/value preferences.
type UserPreference struct {
	UserID string `gorm:"primaryKey;size:36" json:"user_id"`
	Key    string `gorm:"primaryKey;size:64" json:"key"`
	Value  string `gorm:"type:text" json:"value"`
}

func (UserPreference) TableName() string { return "user_preferences" }

const (
	PrefEmailDigest       = "email_digest" // weekly | off
	PrefEmailDigestWeekly = "weekly"
	PrefEmailDigestOff    = "off"
)
