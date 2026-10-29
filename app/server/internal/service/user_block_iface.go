package service

import "context"

type UserBlockService interface {
	Block(ctx context.Context, blockerID, blockedID string) error
	Unblock(ctx context.Context, blockerID, blockedID string) error
	IsBlocked(ctx context.Context, blockerID, blockedID string) (bool, error)
	EitherBlocked(ctx context.Context, a, b string) (bool, error)
	ListBlockedIDs(ctx context.Context, blockerID string, page, pageSize int) ([]string, int, error)
}
