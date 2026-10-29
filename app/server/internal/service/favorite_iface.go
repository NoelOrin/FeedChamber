package service

import (
	"context"
)

type FavoriteService interface {
	Toggle(ctx context.Context, postID, userID string) (bool, error)
	HasFavorited(ctx context.Context, postID, userID string) (bool, error)
	ListByUser(ctx context.Context, userID string, page, pageSize int) ([]string, int, error)
	CountByUser(ctx context.Context, userID string) (int64, error)
	// SetTrustService 注入等级评估服务（改变被收藏指标后触发重算）。
	SetTrustService(svc TrustService)
}
