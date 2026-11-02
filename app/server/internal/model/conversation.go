package model

import (
	"time"

	"feedchamber/internal/pkg"

	"gorm.io/gorm"
)

// Conversation is a private-message thread (direct for MVP).
type Conversation struct {
	ID            string         `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	LastMessageAt *time.Time     `json:"last_message_at,omitempty"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Conversation) TableName() string { return "conversations" }

// NewConversation creates a Conversation with generated ID and timestamps.
func NewConversation() *Conversation {
	now := time.Now()
	return &Conversation{ID: pkg.NewID(), CreatedAt: now, UpdatedAt: now}
}

// ConversationMember links a user to a conversation.
type ConversationMember struct {
	ID             string     `gorm:"primaryKey" json:"id"`
	ConversationID string     `gorm:"not null;uniqueIndex:idx_conv_member;index" json:"conversation_id"`
	UserID         string     `gorm:"not null;uniqueIndex:idx_conv_member;index" json:"user_id"`
	LastReadAt     *time.Time `json:"last_read_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (ConversationMember) TableName() string { return "conversation_members" }

// NewConversationMember creates a member row with generated ID and timestamp.
func NewConversationMember(conversationID, userID string) *ConversationMember {
	return &ConversationMember{
		ID:             pkg.NewID(),
		ConversationID: conversationID,
		UserID:         userID,
		CreatedAt:      time.Now(),
	}
}

// Message is a single private-message body inside a conversation.
type Message struct {
	ID             string         `gorm:"primaryKey" json:"id"`
	ConversationID string         `gorm:"index;not null" json:"conversation_id"`
	SenderID       string         `gorm:"index;not null" json:"sender_id"`
	Body           string         `gorm:"type:text;not null" json:"body"`
	CreatedAt      time.Time      `json:"created_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Message) TableName() string { return "messages" }

// NewMessage creates a Message with generated ID and timestamp.
func NewMessage(conversationID, senderID, body string) *Message {
	return &Message{
		ID:             pkg.NewID(),
		ConversationID: conversationID,
		SenderID:       senderID,
		Body:           body,
		CreatedAt:      time.Now(),
	}
}

// ConversationListItem is the DTO for a user's conversation inbox row.
// Peer fields describe the other participant in a direct conversation.
type ConversationListItem struct {
	ID            string     `json:"id"`
	PeerID        string     `json:"peer_id"`
	PeerName      string     `json:"peer_name"`
	PeerAvatar    string     `json:"peer_avatar"`
	LastMessage   string     `json:"last_message,omitempty"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty"`
	UnreadCount   int64      `json:"unread_count"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
