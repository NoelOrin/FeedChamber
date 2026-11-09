package model

import "time"

// TopicReadState tracks a user's reading relationship with a topic/post.
//
// 一行同时承载三个语义不同的事实，各自的写入方不同——把它们混成一列是这个模块
// 历史上最容易出事的地方（被动浏览会写 last_viewed_at，若顺手把 last_read_at
// 也写上，「有新回复」角标与「没读过」筛选当场失真）：
//
//   - LastViewedAt    上次打开该帖；由服务端在详情读取与阅读位置上报时写。
//   - LastReadAt      上次显式「标记已读」；**只有** MarkRead 那条路可以写，
//     可空即「看过但没标记读完」，NULL 行不参与 read/unread/new 三档比较。
//   - LastContentID   续读游标（该帖下最后读到的一条评论）；空串表示没有可用位置。
type TopicReadState struct {
	ID            string     `gorm:"primaryKey" json:"id"`
	UserID        string     `gorm:"uniqueIndex:idx_user_post;not null" json:"user_id"`
	PostID        string     `gorm:"uniqueIndex:idx_user_post;not null;index" json:"post_id"`
	LastReadAt    *time.Time `json:"last_read_at"`
	LastViewedAt  *time.Time `json:"last_viewed_at"`
	LastContentID string     `json:"last_content_id"`
}

func (TopicReadState) TableName() string { return "topic_read_states" }
