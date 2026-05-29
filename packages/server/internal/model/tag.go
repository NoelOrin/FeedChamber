package model

import "time"

type Tag struct {
	ID        string    `gorm:"primaryKey" bson:"_id"`
	Name      string    `gorm:"uniqueIndex;not null" bson:"name"`
	CreatedAt time.Time `bson:"created_at"`
}

func (Tag) TableName() string { return "tags" }

type PostTag struct {
	ID     uint   `gorm:"primaryKey" bson:"_id"`
	PostID string `gorm:"index;not null" bson:"post_id"`
	TagID  string `gorm:"index;not null" bson:"tag_id"`
}

func (PostTag) TableName() string { return "post_tags" }
