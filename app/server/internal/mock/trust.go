package mock

import (
	"context"
	"sort"
	"sync"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/repository"
)

// TrustRepository 是 repository.TrustRepository 的内存替身。
//
// 指标聚合由测试直接灌值（Metrics / Daily / Streak / ActiveDays / Upheld），
// 配置表也放在内存里；派生缓存写入是 no-op —— 单点写入的行为由 trust_level
// 的 SQLite 集成测试覆盖，这里只服务「发帖 / 审核是否触发重算」这类用例。
type TrustRepository struct {
	mu sync.Mutex

	rules       map[string]model.XPRule
	bonuses     map[int]model.XPStreakBonus
	levels      map[int]model.TrustLevel
	adjustments []model.XPAdjustment

	// 测试注入的指标数据
	Metrics    map[string]int
	Daily      map[string]map[string]int
	Streak     map[int]int
	ActiveDays int
	Upheld     int

	// Snapshot 记录最后一次 SaveLevelSnapshot 的入参，便于断言。
	Snapshot *TrustSnapshotCall
}

type TrustSnapshotCall struct {
	UserID    string
	TotalXP   int
	Level     int
	Metrics   string
	Evaluated time.Time
}

func NewTrustRepository() *TrustRepository {
	return &TrustRepository{
		rules:   map[string]model.XPRule{},
		bonuses: map[int]model.XPStreakBonus{},
		levels:  map[int]model.TrustLevel{},
		Metrics: map[string]int{},
		Daily:   map[string]map[string]int{},
		Streak:  map[int]int{},
	}
}

// WithDefaultConfig 灌入定稿默认配置（等级 / XP 规则 / 连续签到阶梯）。
func (r *TrustRepository) WithDefaultConfig() *TrustRepository {
	for _, rule := range model.DefaultXPRules {
		r.rules[rule.Metric] = rule
	}
	for _, bonus := range model.DefaultXPStreakBonuses {
		r.bonuses[bonus.Days] = bonus
	}
	for _, level := range model.DefaultTrustLevels {
		r.levels[level.Level] = level
	}
	return r
}

func (r *TrustRepository) ListXPRules(context.Context) ([]model.XPRule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]model.XPRule, 0, len(r.rules))
	for _, v := range r.rules {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Metric < out[j].Metric })
	return out, nil
}

func (r *TrustRepository) UpsertXPRule(_ context.Context, rule *model.XPRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules[rule.Metric] = *rule
	return nil
}

func (r *TrustRepository) DeleteXPRule(_ context.Context, metric string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.rules, metric)
	return nil
}

func (r *TrustRepository) ListStreakBonuses(context.Context) ([]model.XPStreakBonus, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]model.XPStreakBonus, 0, len(r.bonuses))
	for _, v := range r.bonuses {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Days < out[j].Days })
	return out, nil
}

func (r *TrustRepository) UpsertStreakBonus(_ context.Context, bonus *model.XPStreakBonus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bonuses[bonus.Days] = *bonus
	return nil
}

func (r *TrustRepository) DeleteStreakBonus(_ context.Context, days int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.bonuses, days)
	return nil
}

func (r *TrustRepository) ListLevels(context.Context) ([]model.TrustLevel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]model.TrustLevel, 0, len(r.levels))
	for _, v := range r.levels {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Level < out[j].Level })
	return out, nil
}

func (r *TrustRepository) UpsertLevel(_ context.Context, level *model.TrustLevel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.levels[level.Level] = *level
	return nil
}

func (r *TrustRepository) DeleteLevel(_ context.Context, level int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.levels, level)
	return nil
}

func (r *TrustRepository) CreateAdjustment(_ context.Context, adj *model.XPAdjustment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adjustments = append(r.adjustments, *adj)
	return nil
}

func (r *TrustRepository) GetAdjustment(_ context.Context, id string) (*model.XPAdjustment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.adjustments {
		if r.adjustments[i].ID == id {
			adj := r.adjustments[i]
			return &adj, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *TrustRepository) DeleteAdjustment(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.adjustments {
		if r.adjustments[i].ID == id {
			r.adjustments = append(r.adjustments[:i], r.adjustments[i+1:]...)
			return nil
		}
	}
	return repository.ErrNotFound
}

func (r *TrustRepository) ListAdjustments(_ context.Context, userID string, offset, limit int) ([]model.XPAdjustment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []model.XPAdjustment{}
	for _, adj := range r.adjustments {
		if adj.UserID == userID {
			out = append(out, adj)
		}
	}
	if offset >= len(out) {
		return []model.XPAdjustment{}, nil
	}
	out = out[offset:]
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *TrustRepository) CountAdjustments(_ context.Context, userID string) (int64, error) {
	var n int64
	for _, adj := range r.adjustments {
		if adj.UserID == userID {
			n++
		}
	}
	return n, nil
}

func (r *TrustRepository) SumAdjustments(_ context.Context, userID string) (int, error) {
	sum := 0
	for _, adj := range r.adjustments {
		if adj.UserID == userID {
			sum += adj.Delta
		}
	}
	return sum, nil
}

func (r *TrustRepository) MetricDailyCounts(_ context.Context, _ string, metric string, _ time.Time) ([]int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []int{}
	for _, count := range r.Daily[metric] {
		out = append(out, count)
	}
	sort.Ints(out)
	return out, nil
}

func (r *TrustRepository) CountMetric(_ context.Context, _ string, metric string) (int, error) {
	return r.Metrics[metric], nil
}

func (r *TrustRepository) StreakHits(context.Context, string) (map[int]int, error) {
	out := make(map[int]int, len(r.Streak))
	for k, v := range r.Streak {
		out[k] = v
	}
	return out, nil
}

func (r *TrustRepository) CountActiveDays(context.Context, string, time.Time) (int, error) {
	return r.ActiveDays, nil
}

func (r *TrustRepository) CountUpheldReports(context.Context, string, time.Time) (int, error) {
	return r.Upheld, nil
}

func (r *TrustRepository) SaveLevelSnapshot(_ context.Context, userID string, totalXP, level int, metricsJSON string, evaluatedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Snapshot = &TrustSnapshotCall{UserID: userID, TotalXP: totalXP, Level: level, Metrics: metricsJSON, Evaluated: evaluatedAt}
	return nil
}

func (r *TrustRepository) SetManualLevel(context.Context, string, int, bool) error {
	return nil
}
