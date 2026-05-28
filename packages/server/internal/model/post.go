package model

import "time"

type Category struct {
	ID          string    `gorm:"primaryKey" bson:"_id"`
	Name        string    `gorm:"uniqueIndex;not null" bson:"name"`
	Description string    `bson:"description"`
	CreatedAt   time.Time `bson:"created_at"`
}

type Post struct {
	ID         string    `gorm:"primaryKey" bson:"_id"`
	Title      string    `gorm:"not null" bson:"title"`
	Content    string    `gorm:"type:text" bson:"content"`
	AuthorID   string    `gorm:"not null;index" bson:"author_id"`
	CategoryID string    `gorm:"index" bson:"category_id"`
	Pinned     bool      `bson:"pinned"`
	Locked     bool      `bson:"locked"`
	ViewCount  int64     `bson:"view_count"`
	CreatedAt  time.Time `bson:"created_at"`
	UpdatedAt  time.Time `bson:"updated_at"`
}

type Comment struct {
	ID        string    `gorm:"primaryKey" bson:"_id"`
	PostID    string    `gorm:"not null;index" bson:"post_id"`
	AuthorID  string    `gorm:"not null;index" bson:"author_id"`
	Content   string    `gorm:"type:text" bson:"content"`
	ParentID  string    `gorm:"index" bson:"parent_id"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}
