package model

import "time"

// OAuthAuthorize stores authorization codes for the authorization_code grant.
type OAuthAuthorize struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Code        string    `gorm:"uniqueIndex;not null" json:"code"`
	ClientID    string    `gorm:"index;not null" json:"client_id"`
	UserID      string    `gorm:"index;not null" json:"user_id"`
	Scope       string    `json:"scope"`
	RedirectURI string    `json:"redirect_uri"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func (OAuthAuthorize) TableName() string { return "oauth_authorization_codes" }
