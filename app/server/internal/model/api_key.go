package model

// APIKey represents an API key for programmatic access.
type APIKey struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	KeyID      string `gorm:"uniqueIndex;not null" json:"key_id"`
	SecretHash string `gorm:"not null" json:"-"`
	UserUUID   string `gorm:"index;not null" json:"user_uuid"`
	Name       string `gorm:"not null" json:"name"`
	Scopes     string `gorm:"type:text" json:"scopes"`
	RateLimit  int    `gorm:"default:0" json:"rate_limit"`
	ExpiresAt  *int64 `json:"expires_at,omitempty"`
	LastUsedAt *int64 `json:"last_used_at,omitempty"`
	Status     string `gorm:"default:active" json:"status"`
	CreatedAt  int64  `json:"created_at"`
}

func (APIKey) TableName() string { return "api_keys" }
