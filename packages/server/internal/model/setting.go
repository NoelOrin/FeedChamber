package model

import "time"

const (
	SettingPostAuthRequired    = "post_auth_required"
	SettingCommentAuthRequired = "comment_auth_required"
)

// Setting represents a key-value system setting.
type Setting struct {
	Key       string    `gorm:"primaryKey;size:64" bson:"_id"`
	Value     string    `gorm:"type:text" bson:"value"`
	UpdatedAt time.Time `bson:"updated_at"`
}

func (Setting) TableName() string { return "settings" }

// DefaultSettings are seeded on first startup.
var DefaultSettings = map[string]string{
	"site_name":           "OrinForum",
	"site_description":    "A community forum",
	"register_enabled":    "true",
	"post_approval":       "false",
	"post_auth_required":  "false",
	"comment_auth_required": "false",
}

// PublicSettingKeys are settings readable without authentication.
var PublicSettingKeys = []string{
	"site_name",
	"site_description",
	"register_enabled",
	"post_auth_required",
	"comment_auth_required",
}
