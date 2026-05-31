package model

import "time"

type Tag struct {
	ID        string    `gorm:"primaryKey" json:"id" bson:"_id"`
	Name      string    `gorm:"uniqueIndex;not null" json:"name" bson:"name"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}

func (Tag) TableName() string { return "tags" }

type PostTag struct {
	ID     uint   `gorm:"primaryKey" json:"id" bson:"_id"`
	PostID string `gorm:"index;not null" json:"post_id" bson:"post_id"`
	TagID  string `gorm:"index;not null" json:"tag_id" bson:"tag_id"`
}

func (PostTag) TableName() string { return "post_tags" }
