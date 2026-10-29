package service

import (
	"context"

	"orinforum/internal/model"
	"orinforum/internal/pkg/eventbus"
)

type ModerationService interface {
	// Content review queue
	ListPending(ctx context.Context, page, pageSize int) ([]model.ContentModeration, int64, error)
	Approve(ctx context.Context, id, reviewerID string) error
	Reject(ctx context.Context, id, reviewerID, reason string) error

	// Rules management
	CreateRule(ctx context.Context, rule *model.ModerationRule) error
	UpdateRule(ctx context.Context, rule *model.ModerationRule) error
	DeleteRule(ctx context.Context, id string) error
	ListRules(ctx context.Context, page, pageSize int) ([]model.ModerationRule, int64, error)

	// Config management
	GetConfigs(ctx context.Context) (map[string]string, error)
	SetConfig(ctx context.Context, key, value string) error

	// Mode 返回当前文章审核模式（off / async / external / manual）。
	Mode(ctx context.Context) string
	// CheckContent runs moderation rules against content before persistence.
	CheckContent(ctx context.Context, targetType, targetID, body string, scene model.ModerationScene, authorRole string) (model.ModerationAction, error)
	// CheckExternal 在发布前调用外部自动化审核服务（external 模式）。
	// 返回 errExternalModerationUnavailable 时调用方应回退到发布后异步审核。
	CheckExternal(ctx context.Context, targetType, targetID, title, body string, scene model.ModerationScene) (model.ModerationAction, error)
	// RecordPending creates a pending moderation queue item for dashboard review.
	RecordPending(ctx context.Context, targetType, targetID string, scene model.ModerationScene, action model.ModerationAction, body string) error
	// RecordPendingFrom 同 RecordPending，但标注判定来源（async / external / tl0 / manual）。
	RecordPendingFrom(ctx context.Context, targetType, targetID string, scene model.ModerationScene, action model.ModerationAction, body, strategy string) error
	// SubscribeAsync 注册「发布后异步审核」消费者，由 container.Runtime 启动时调用。
	SubscribeAsync(bus *eventbus.EventBus)
	// SetTrustService 注入等级评估服务（审核通过/拒绝后重算作者 XP）。
	SetTrustService(svc TrustService)
}
