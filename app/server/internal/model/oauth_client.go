package model

// OAuthClient represents a registered OAuth 2.0 client (third-party application).
type OAuthClient struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	ClientID     string `gorm:"uniqueIndex;not null" json:"client_id"`
	ClientSecret string `gorm:"not null" json:"-"`
	Name         string `gorm:"not null" json:"name"`
	RedirectURIs string `gorm:"type:text" json:"redirect_uris"` // JSON 数组
	Scopes       string `gorm:"type:text" json:"scopes"`        // 允许的 scope
	CreatedBy    string `json:"created_by"`
	CreatedAt    int64  `json:"created_at"`
}

func (OAuthClient) TableName() string { return "oauth_clients" }
