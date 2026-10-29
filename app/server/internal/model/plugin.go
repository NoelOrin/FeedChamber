package model

import "time"

// Plugin is a declarative plugin registration record. The first implementation
// intentionally stores metadata only and never executes plugin code.
type Plugin struct {
	ID          string    `gorm:"primaryKey;size:64" json:"id"`
	Name        string    `gorm:"size:128;not null" json:"name"`
	Version     string    `gorm:"size:32" json:"version"`
	Author      string    `gorm:"size:128" json:"author"`
	Description string    `gorm:"type:text" json:"description"`
	Homepage    string    `gorm:"size:512" json:"homepage"`
	Settings    string    `gorm:"type:text" json:"settings"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Plugin) TableName() string { return "plugins" }
