package model

import (
	"time"

	"gorm.io/gorm"
)

type Category struct {
	ID          string    `gorm:"primaryKey" json:"id" bson:"_id"`
	Name        string    `gorm:"uniqueIndex;not null" json:"name" bson:"name"`
	Description string    `json:"description" bson:"description"`
	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
}

type Post struct {
	ID            string         `gorm:"primaryKey" json:"id" bson:"_id"`
	Title         string         `gorm:"not null" json:"title" bson:"title"`
	ContentID     string         `gorm:"uniqueIndex;not null" json:"content_id" bson:"content_id"`
	ContentDigest string         `gorm:"size:200" json:"content_digest" bson:"content_digest"`
	AuthorID      string         `gorm:"not null;index" json:"author_id" bson:"author_id"`
	CategoryID    string         `gorm:"index" json:"category_id" bson:"category_id"`
	Status        string         `gorm:"size:16;default:approved;index" json:"status" bson:"status"`
	Pinned        bool           `json:"pinned" bson:"pinned"`
	Locked        bool           `json:"locked" bson:"locked"`
	ViewCount     int64          `json:"view_count" bson:"view_count"`
	CommentCount  int64          `gorm:"default:0" json:"comment_count" bson:"comment_count"`
	LikeCount     int64          `gorm:"default:0" json:"like_count" bson:"like_count"`
	FavoriteCount int64          `gorm:"default:0" json:"favorite_count" bson:"favorite_count"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty" bson:"deleted_at" swaggertype:"string"`
	CreatedAt     time.Time      `json:"created_at" bson:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at" bson:"updated_at"`
}

// PostListItem is the DTO for post list queries (no JOIN needed).
type PostListItem struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	ContentDigest string    `json:"content_digest"`
	AuthorID      string    `json:"author_id"`
	AuthorName    string    `json:"author_name"`
	AuthorAvatar  string    `json:"author_avatar"`
	CategoryID    string    `json:"category_id"`
	CategoryName  string    `json:"category_name"`
	Status        string    `json:"status"`
	Pinned        bool      `json:"pinned"`
	Locked        bool      `json:"locked"`
	ViewCount     int64     `json:"view_count"`
	CommentCount  int64     `json:"comment_count"`
	LikeCount     int64     `json:"like_count"`
	FavoriteCount int64     `json:"favorite_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
