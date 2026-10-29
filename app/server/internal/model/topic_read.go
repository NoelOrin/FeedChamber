package model

import "time"

// TopicReadState tracks a user's last read position for a topic/post.
type TopicReadState struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	UserID        string    `gorm:"uniqueIndex:idx_user_post;not null" json:"user_id"`
	PostID        string    `gorm:"uniqueIndex:idx_user_post;not null;index" json:"post_id"`
	LastReadAt    time.Time `json:"last_read_at"`
	LastContentID string    `json:"last_content_id"`
}

func (TopicReadState) TableName() string { return "topic_read_states" }
