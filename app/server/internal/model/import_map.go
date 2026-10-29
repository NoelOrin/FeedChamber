package model

import "time"

// ImportMap maps an external source ID to a local OrinForum ID for idempotent imports.
type ImportMap struct {
	ID         string    `gorm:"primaryKey;size:64" json:"id"`
	Source     string    `gorm:"uniqueIndex:idx_import_source_ext,priority:1;size:32" json:"source"`
	EntityType string    `gorm:"uniqueIndex:idx_import_source_ext,priority:2;size:32" json:"entity_type"`
	ExternalID string    `gorm:"uniqueIndex:idx_import_source_ext,priority:3;size:128" json:"external_id"`
	LocalID    string    `gorm:"index;size:64" json:"local_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func (ImportMap) TableName() string { return "import_maps" }
