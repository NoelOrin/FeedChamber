package model

import "time"

// OAuthToken stores issued opaque access tokens together with the metadata
// needed to authenticate and revoke them.
//
// TokenHash holds the SHA-256 hex digest of the opaque bearer token; the
// plaintext is returned to the client once and never persisted. The column is
// still named access_token because it used to store the JWT ID (JTI) and
// renaming it would require a schema migration on sqlite/mysql/postgres alike.
type OAuthToken struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ClientID  string    `gorm:"index;not null" json:"client_id"`
	UserID    string    `gorm:"index" json:"user_id"`
	TokenHash string    `gorm:"column:access_token;uniqueIndex;not null" json:"-"`
	Scope     string    `json:"scope"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	Revoked   bool      `gorm:"default:false" json:"revoked"`
}

func (OAuthToken) TableName() string { return "oauth_tokens" }
