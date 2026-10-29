package service

import (
	"context"

	"orinforum/internal/model"
)

// TrustService 是 XP / 等级体系的唯一写入方与配置读写入口。
//
// 约定：任何业务服务（发帖、点赞、签到…）都只能调用 Recompute，不能直接写
// users.trust_level / total_xp / level_metrics；这些字段是派生缓存。
type TrustService interface {
	// Recompute 重算单个用户的 XP 与等级（唯一写入点）。
	Recompute(ctx context.Context, userID string) (*UserTrust, error)
	// RecomputeAll 全量重算（迁移命令、每日对账、后台手动触发）。
	RecomputeAll(ctx context.Context) (count int, err error)
	// LevelOf 廉价读取用户当前等级；游客（空 userID）返回 0。
	LevelOf(ctx context.Context, userID string) int
	// GetSnapshot 读取快照；缺失或超过 24h 时惰性重算。
	GetSnapshot(ctx context.Context, userID string) (*UserTrust, error)
	// GetBreakdown 返回 XP 指标分解（后台用户详情）。
	GetBreakdown(ctx context.Context, userID string) (*XPBreakdown, error)

	// AdjustXP 写入人工/活动加减分并立即重算（必须有原因）。
	AdjustXP(ctx context.Context, actorID, userID string, delta int, reason string) (*model.XPAdjustment, error)
	ListAdjustments(ctx context.Context, userID string, page, pageSize int) ([]model.XPAdjustment, int, error)
	DeleteAdjustment(ctx context.Context, id string) error

	// SetManualLevel 管理员手工设级；manual=false 时解除锁定并重算。
	SetManualLevel(ctx context.Context, userID string, level int, manual bool) error
	// UpdateLevel 是 SetManualLevel(manual=true) 的兼容入口。
	UpdateLevel(ctx context.Context, userID string, level int) error

	// StartDailyRecomputeTicker 每日全量重算（处理不活跃衰减降级）。
	StartDailyRecomputeTicker(ctx context.Context) context.CancelFunc
	// StartSyncSubscriber 订阅 trust:sync：配置写入后让其它实例立即失效本地缓存。
	StartSyncSubscriber(ctx context.Context) error

	// 等级 / XP 规则 / 连续签到阶梯的配置读写（后台）。
	ListLevels(ctx context.Context) ([]model.TrustLevel, error)
	SaveLevel(ctx context.Context, level *model.TrustLevel) error
	DeleteLevel(ctx context.Context, level int) error
	ListXPRules(ctx context.Context) ([]model.XPRule, error)
	SaveXPRule(ctx context.Context, rule *model.XPRule) error
	ListStreakBonuses(ctx context.Context) ([]model.XPStreakBonus, error)
	SaveStreakBonus(ctx context.Context, bonus *model.XPStreakBonus) error
	DeleteStreakBonus(ctx context.Context, days int) error
}
