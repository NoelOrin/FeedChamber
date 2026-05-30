package model

import (
	"time"

	"gorm.io/gorm"
)

type Category struct {
	ID          string    `gorm:"primaryKey" bson:"_id"`
	Name        string    `gorm:"uniqueIndex;not null" bson:"name"`
	Description string    `bson:"description"`
	CreatedAt   time.Time `bson:"created_at"`
}

type Post struct {
	ID            string         `gorm:"primaryKey" bson:"_id"`
	Title         string         `gorm:"not null" bson:"title"`
	ContentID     string         `gorm:"uniqueIndex;not null" bson:"content_id"` // FK → contents.id (post body)
	ContentDigest string         `gorm:"size:200" bson:"content_digest"`         // denormalized preview for list queries
	AuthorID      string         `gorm:"not null;index" bson:"author_id"`
	CategoryID    string         `gorm:"index" bson:"category_id"`
	Status        string         `gorm:"size:16;default:approved;index" bson:"status"` // draft/pending/approved/rejected
	Pinned        bool           `bson:"pinned"`
	Locked        bool           `bson:"locked"`
	ViewCount     int64          `bson:"view_count"`
	CommentCount  int64          `gorm:"default:0" bson:"comment_count"` // denormalized reply count
	LikeCount     int64          `gorm:"default:0" bson:"like_count"`    // denormalized like count
	FavoriteCount int64          `gorm:"default:0" bson:"favorite_count"`
	DeletedAt     gorm.DeletedAt `gorm:"index" bson:"deleted_at" swaggertype:"string"`
	CreatedAt     time.Time      `bson:"created_at"`
	UpdatedAt     time.Time      `bson:"updated_at"`
}

// PostListItem is the DTO for post list queries (no JOIN needed).
type PostListItem struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	ContentDigest string    `json:"contentDigest"`
	AuthorID      string    `json:"authorId"`
	AuthorName    string    `json:"authorName"`
	AuthorAvatar  string    `json:"authorAvatar"`
	CategoryID    string    `json:"categoryId"`
	CategoryName  string    `json:"categoryName"`
	Status        string    `json:"status"`
	Pinned        bool      `json:"pinned"`
	Locked        bool      `json:"locked"`
	ViewCount     int64     `json:"viewCount"`
	CommentCount  int64     `json:"commentCount"`
	LikeCount     int64     `json:"likeCount"`
	FavoriteCount int64     `json:"favoriteCount"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
