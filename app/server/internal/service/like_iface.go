package service

import (
	"context"
)

type LikeService interface {
	ToggleContentLike(ctx context.Context, contentID, userID string) (bool, error)
	HasContentLiked(ctx context.Context, contentID, userID string) (bool, error)
	CountContentLikes(ctx context.Context, contentID string) (int64, error)
	// SetTrustService 注入等级评估服务（改变被赞指标后触发重算）。
	SetTrustService(svc TrustService)
}
