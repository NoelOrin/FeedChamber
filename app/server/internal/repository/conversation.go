package repository

import (
	"context"
	"time"

	"orinforum/internal/db"
	"orinforum/internal/model"
)

// ConversationRepository is the data access layer for private-message conversations.
type ConversationRepository interface {
	CreateConversation(ctx context.Context, c *model.Conversation, memberUserIDs []string) error
	FindByID(ctx context.Context, id string) (*model.Conversation, error)
	FindDirect(ctx context.Context, userA, userB string) (*model.Conversation, error)
	ListForUser(ctx context.Context, userID string, offset, limit int) ([]model.ConversationListItem, int, error)
	IsMember(ctx context.Context, conversationID, userID string) (bool, error)
	CreateMessage(ctx context.Context, m *model.Message) error
	ListMessages(ctx context.Context, conversationID string, offset, limit int) ([]model.Message, int, error)
	MarkRead(ctx context.Context, conversationID, userID string, at time.Time) error
}

// NewConversationRepository returns the SQL implementation backed by store.
func NewConversationRepository(store db.Store) ConversationRepository {
	return &sqlConversationRepo{db: store.SQL()}
}
