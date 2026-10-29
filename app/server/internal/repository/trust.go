package repository

import (
	"context"
	"time"

	"orinforum/internal/db"
	"orinforum/internal/model"
)

// TrustRepository 承担 XP/等级体系的数据访问：配置表（规则 / 阶梯 / 等级 /
// 人工调整）与行为表的指标聚合。
//
// 指标聚合只读既有业务表（posts / contents / content_likes / post_favorites /
// check_ins / login_histories / reports），不新增事件账本——撤销行为因此天然
// 回收 XP，不需要「扣分」路径。
type TrustRepository interface {
	// 配置
	ListXPRules(ctx context.Context) ([]model.XPRule, error)
	UpsertXPRule(ctx context.Context, rule *model.XPRule) error
	DeleteXPRule(ctx context.Context, metric string) error
	ListStreakBonuses(ctx context.Context) ([]model.XPStreakBonus, error)
	UpsertStreakBonus(ctx context.Context, bonus *model.XPStreakBonus) error
	DeleteStreakBonus(ctx context.Context, days int) error
	ListLevels(ctx context.Context) ([]model.TrustLevel, error)
	UpsertLevel(ctx context.Context, level *model.TrustLevel) error
	DeleteLevel(ctx context.Context, level int) error

	// 人工调整
	CreateAdjustment(ctx context.Context, adj *model.XPAdjustment) error
	GetAdjustment(ctx context.Context, id string) (*model.XPAdjustment, error)
	DeleteAdjustment(ctx context.Context, id string) error
	ListAdjustments(ctx context.Context, userID string, offset, limit int) ([]model.XPAdjustment, error)
	CountAdjustments(ctx context.Context, userID string) (int64, error)
	SumAdjustments(ctx context.Context, userID string) (int, error)

	// 指标聚合
	// MetricDailyCounts 返回某指标按自然日分组的每日计数（顺序无关）。
	// 只取计数不取日期：日上限只需要「每天多少条」，日期本身不参与计算。
	MetricDailyCounts(ctx context.Context, userID, metric string, since time.Time) ([]int, error)
	CountMetric(ctx context.Context, userID, metric string) (int, error)
	StreakHits(ctx context.Context, userID string) (map[int]int, error)
	CountActiveDays(ctx context.Context, userID string, since time.Time) (int, error)
	CountUpheldReports(ctx context.Context, userID string, since time.Time) (int, error)

	// 派生缓存写入（唯一调用方是 service.TrustService）
	SaveLevelSnapshot(ctx context.Context, userID string, totalXP, level int, metricsJSON string, evaluatedAt time.Time) error
	SetManualLevel(ctx context.Context, userID string, level int, manual bool) error
}

func NewTrustRepository(store db.Store) TrustRepository {
	return &sqlTrustRepo{db: store.SQL()}
}
