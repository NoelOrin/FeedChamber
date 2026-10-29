package service

import (
	"context"

	"orinforum/internal/model"
)

// PMService provides direct private-message operations.
type PMService interface {
	StartDirect(ctx context.Context, actorID, peerUserID, firstBody string) (*model.Conversation, *model.Message, error)
	ListConversations(ctx context.Context, userID string, page, pageSize int) ([]model.ConversationListItem, int, error)
	ListMessages(ctx context.Context, userID, conversationID string, page, pageSize int) ([]model.Message, int, error)
	Send(ctx context.Context, userID, conversationID, body string) (*model.Message, error)
	MarkRead(ctx context.Context, userID, conversationID string) error
}
