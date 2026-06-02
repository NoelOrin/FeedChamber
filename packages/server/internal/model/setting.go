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
	"site_name":            "OrinForum",
	"site_description":     "NoelOrin 的个人博客",
	"site_logo":            "",
	"site_url":             "",
	"seo_title":            "OrinForum",
	"seo_keywords":         "OrinForum,博客,社区,技术",
	"register_enabled":     "true",
	"post_approval":        "false",
	"post_auth_required":   "false",
	"comment_auth_required": "false",
	"article_page_size":    "20",
	"s3_endpoint":          "",
	"s3_region":            "",
	"s3_bucket":            "",
	"s3_access_key":        "",
	"s3_secret_key":        "",
}

// PublicSettingKeys are settings readable without authentication.
var PublicSettingKeys = []string{
	"site_name",
	"site_description",
	"site_logo",
	"site_url",
	"seo_title",
	"seo_keywords",
	"register_enabled",
	"post_auth_required",
	"comment_auth_required",
	"article_page_size",
}
