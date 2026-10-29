package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/pkg/kvstore"
	"orinforum/internal/repository"

	"github.com/sirupsen/logrus"
)

// UserTrust 是某个用户等级快照的对外投影。
type UserTrust struct {
	UserID           string             `json:"user_id"`
	TotalXP          int                `json:"total_xp"`
	Level            int                `json:"trust_level"`
	LevelKey         string             `json:"level_key"`
	LevelName        string             `json:"level_name"`
	LevelManual      bool               `json:"level_manual"`
	LevelEvaluatedAt *time.Time         `json:"level_evaluated_at,omitempty"`
	Metrics          model.LevelMetrics `json:"metrics"`
}

// XPMetricContribution 是单个指标的 XP 分解行（后台用户详情展示）。
type XPMetricContribution struct {
	Metric string `json:"metric"`
	Label  string `json:"label"`
	Count  int    `json:"count"`
	Delta  int    `json:"delta"`
	XP     int    `json:"xp"`
}

// XPStreakContribution 是连续签到阶梯的 XP 分解行。
type XPStreakContribution struct {
	Days  int `json:"days"`
	Hits  int `json:"hits"`
	Bonus int `json:"bonus"`
	XP    int `json:"xp"`
}

// XPBreakdown 是「指标分解」：各项之和（含人工调整）必须等于 TotalXP。
type XPBreakdown struct {
	UserTrust
	Metrics     []XPMetricContribution `json:"xp_metrics"`
	Streaks     []XPStreakContribution `json:"xp_streaks"`
	Adjustments int                    `json:"xp_adjustments"`
}

type trustConfig struct {
	rules    []model.XPRule
	bonuses  []model.XPStreakBonus
	levels   []model.TrustLevel
	loadedAt time.Time
}

const trustConfigTTL = 60 * time.Second

// trustSyncChannel 是配置失效广播：写配置的实例发，其它实例清本地快照。
const trustSyncChannel = "trust:sync"

type trustService struct {
	users      repository.UserRepository
	trustRepo  repository.TrustRepository
	settingSvc SettingService
	kv         kvstore.Store

	mu  sync.RWMutex
	cfg *trustConfig

	// now 允许测试固定时钟。
	now func() time.Time
}

func NewTrustService(users repository.UserRepository, trustRepo repository.TrustRepository, settingSvc SettingService, kv kvstore.Store) TrustService {
	return &trustService{users: users, trustRepo: trustRepo, settingSvc: settingSvc, kv: kv, now: time.Now}
}

// ── 配置缓存 ────────────────────────────────────────────────────────────────

func (s *trustService) config(ctx context.Context) (*trustConfig, error) {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	if cfg != nil && s.now().Sub(cfg.loadedAt) < trustConfigTTL {
		return cfg, nil
	}

	rules, err := s.trustRepo.ListXPRules(ctx)
	if err != nil {
		return nil, err
	}
	bonuses, err := s.trustRepo.ListStreakBonuses(ctx)
	if err != nil {
		return nil, err
	}
	levels, err := s.trustRepo.ListLevels(ctx)
	if err != nil {
		return nil, err
	}
	cfg = &trustConfig{rules: rules, bonuses: bonuses, levels: levels, loadedAt: s.now()}

	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	return cfg, nil
}

// invalidateConfig 清本地配置快照，并通过 trust:sync 让其它实例一起失效。
// XP 规则/等级是「改分值立即对全员生效」的配置，不能靠 60s TTL 慢慢过期。
func (s *trustService) invalidateConfig(ctx context.Context) {
	s.mu.Lock()
	s.cfg = nil
	s.mu.Unlock()
	if s.kv != nil {
		_ = s.kv.Publish(ctx, trustSyncChannel, []byte("reload"))
	}
}

// ── XP 求值 ────────────────────────────────────────────────────────────────

// computeXP 按「自然日 × 指标」截断聚合 XP，再叠加连续签到阶梯与人工调整。
func (s *trustService) computeXP(ctx context.Context, userID string, cfg *trustConfig) (int, []XPMetricContribution, []XPStreakContribution, int, error) {
	total := 0
	contributions := make([]XPMetricContribution, 0, len(cfg.rules))
	epoch := time.Unix(0, 0)

	for _, rule := range cfg.rules {
		if !rule.Enabled || rule.Delta == 0 {
			continue
		}
		var count int
		if rule.DailyCap > 0 {
			daily, err := s.trustRepo.MetricDailyCounts(ctx, userID, rule.Metric, epoch)
			if err != nil {
				return 0, nil, nil, 0, err
			}
			for _, c := range daily {
				if c > rule.DailyCap {
					c = rule.DailyCap
				}
				count += c
			}
		} else {
			c, err := s.trustRepo.CountMetric(ctx, userID, rule.Metric)
			if err != nil {
				return 0, nil, nil, 0, err
			}
			count = c
		}
		xp := count * rule.Delta
		total += xp
		contributions = append(contributions, XPMetricContribution{
			Metric: rule.Metric,
			Label:  metricLabel(rule.Metric),
			Count:  count,
			Delta:  rule.Delta,
			XP:     xp,
		})
	}

	hits, err := s.trustRepo.StreakHits(ctx, userID)
	if err != nil {
		return 0, nil, nil, 0, err
	}
	streaks := make([]XPStreakContribution, 0, len(cfg.bonuses))
	for _, bonus := range cfg.bonuses {
		if !bonus.Enabled || bonus.Bonus == 0 {
			continue
		}
		n := hits[bonus.Days]
		if n == 0 {
			continue
		}
		xp := n * bonus.Bonus
		total += xp
		streaks = append(streaks, XPStreakContribution{Days: bonus.Days, Hits: n, Bonus: bonus.Bonus, XP: xp})
	}

	adjustments, err := s.trustRepo.SumAdjustments(ctx, userID)
	if err != nil {
		return 0, nil, nil, 0, err
	}
	total += adjustments
	if total < 0 {
		total = 0
	}
	return total, contributions, streaks, adjustments, nil
}

// evaluateLevel 用累计 XP 定级并按滚动窗口保级，可升可降。
func (s *trustService) evaluateLevel(user *model.User, totalXP int, metricsFor func(windowDays int) (model.LevelMetrics, error), levels []model.TrustLevel) (int, model.LevelMetrics, error) {
	enabled := make([]model.TrustLevel, 0, len(levels))
	for _, l := range levels {
		if l.Enabled {
			enabled = append(enabled, l)
		}
	}
	if len(enabled) == 0 {
		return 0, model.LevelMetrics{}, nil
	}
	sort.SliceStable(enabled, func(i, j int) bool {
		if enabled[i].MinXP == enabled[j].MinXP {
			return enabled[i].Level < enabled[j].Level
		}
		return enabled[i].MinXP < enabled[j].MinXP
	})

	// 管理员手工设级 / 仅人工授予的等级：保持现状，不自动升降；指标仍按当前
	// 等级自己的窗口刷新，后台详情才不会显示成全 0。
	if user.LevelManual || isManualOnly(enabled, user.TrustLevel) {
		m, err := metricsFor(windowDaysFor(enabled, user.TrustLevel))
		return user.TrustLevel, m, err
	}

	for i := len(enabled) - 1; i >= 0; i-- {
		lvl := enabled[i]
		if lvl.ManualOnly || lvl.MinXP > totalXP {
			continue
		}
		metrics, err := metricsFor(lvl.WindowDays)
		if err != nil {
			return user.TrustLevel, model.LevelMetrics{}, err
		}
		if meetsRetention(lvl, metrics) {
			return lvl.Level, metrics, nil
		}
	}
	return enabled[0].Level, model.LevelMetrics{}, nil
}

func windowDaysFor(levels []model.TrustLevel, level int) int {
	for _, l := range levels {
		if l.Level == level {
			return l.WindowDays
		}
	}
	return 0
}

func isManualOnly(levels []model.TrustLevel, level int) bool {
	for _, l := range levels {
		if l.Level == level {
			return l.ManualOnly
		}
	}
	return false
}

// meetsRetention 校验保级条件。WindowDays=0 表示该等级没有保级条件。
func meetsRetention(level model.TrustLevel, metrics model.LevelMetrics) bool {
	if level.WindowDays <= 0 {
		return true
	}
	if metrics.ActiveDays < level.MinActiveDays {
		return false
	}
	if metrics.Posts < level.MinPosts {
		return false
	}
	if metrics.Likes < level.MinLikes {
		return false
	}
	if level.MaxReports != nil && metrics.UpheldReports > *level.MaxReports {
		return false
	}
	return true
}

func (s *trustService) windowMetrics(ctx context.Context, userID string, windowDays int) (model.LevelMetrics, error) {
	if windowDays <= 0 {
		return model.LevelMetrics{}, nil
	}
	since := s.now().AddDate(0, 0, -windowDays)
	active, err := s.trustRepo.CountActiveDays(ctx, userID, since)
	if err != nil {
		return model.LevelMetrics{}, err
	}
	posts, err := s.trustRepo.CountMetric(ctx, userID, model.MetricPostApproved)
	if err != nil {
		return model.LevelMetrics{}, err
	}
	likes, err := s.trustRepo.CountMetric(ctx, userID, model.MetricLikesReceived)
	if err != nil {
		return model.LevelMetrics{}, err
	}
	upheld, err := s.trustRepo.CountUpheldReports(ctx, userID, since)
	if err != nil {
		return model.LevelMetrics{}, err
	}
	return model.LevelMetrics{ActiveDays: active, Posts: posts, Likes: likes, UpheldReports: upheld}, nil
}

// ── 公开方法 ────────────────────────────────────────────────────────────────

// Recompute 是等级 / XP 快照的唯一写入点。
func (s *trustService) Recompute(ctx context.Context, userID string) (*UserTrust, error) {
	if userID == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "用户 ID 不能为空")
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.USER_NOT_FOUND)
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	cfg, err := s.config(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	totalXP, _, _, _, err := s.computeXP(ctx, userID, cfg)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	cache := map[int]model.LevelMetrics{}
	metricsFor := func(windowDays int) (model.LevelMetrics, error) {
		if m, ok := cache[windowDays]; ok {
			return m, nil
		}
		m, err := s.windowMetrics(ctx, userID, windowDays)
		if err != nil {
			return model.LevelMetrics{}, err
		}
		cache[windowDays] = m
		return m, nil
	}
	level, metrics, err := s.evaluateLevel(user, totalXP, metricsFor, cfg.levels)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	evaluatedAt := s.now()
	if err := s.trustRepo.SaveLevelSnapshot(ctx, userID, totalXP, level, metrics.MetricsJSON(), evaluatedAt); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return &UserTrust{
		UserID:           userID,
		TotalXP:          totalXP,
		Level:            level,
		LevelKey:         levelKey(cfg.levels, level),
		LevelName:        levelName(cfg.levels, level),
		LevelManual:      user.LevelManual,
		LevelEvaluatedAt: &evaluatedAt,
		Metrics:          metrics,
	}, nil
}

// RecomputeAll 全量重算，可重复执行（迁移与每日对账共用）。
func (s *trustService) RecomputeAll(ctx context.Context) (int, error) {
	const pageSize = 200
	page := 1
	done := 0
	for {
		users, err := s.users.List(ctx, (page-1)*pageSize, pageSize)
		if err != nil {
			return done, err
		}
		for i := range users {
			if _, err := s.Recompute(ctx, users[i].ID); err != nil {
				logrus.Warnf("trust: recompute %s: %v", users[i].ID, err)
				continue
			}
			done++
		}
		if len(users) < pageSize {
			break
		}
		page++
	}
	return done, nil
}

// LevelOf 读取等级字段，不做任何求值：可见性判定只需要一个数字，
// 不能因为读等级而触发一次全量聚合。
func (s *trustService) LevelOf(ctx context.Context, userID string) int {
	if userID == "" || s.users == nil {
		return 0
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil || user == nil {
		return 0
	}
	return user.TrustLevel
}

// GetSnapshot 读取等级快照；快照缺失或过期时惰性重算。
func (s *trustService) GetSnapshot(ctx context.Context, userID string) (*UserTrust, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.USER_NOT_FOUND)
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if user.LevelEvaluatedAt == nil || s.now().Sub(*user.LevelEvaluatedAt) > 24*time.Hour {
		return s.Recompute(ctx, userID)
	}
	var metrics model.LevelMetrics
	_ = json.Unmarshal([]byte(user.LevelMetrics), &metrics)
	cfg, err := s.config(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return &UserTrust{
		UserID:           user.ID,
		TotalXP:          user.TotalXP,
		Level:            user.TrustLevel,
		LevelKey:         levelKey(cfg.levels, user.TrustLevel),
		LevelName:        levelName(cfg.levels, user.TrustLevel),
		LevelManual:      user.LevelManual,
		LevelEvaluatedAt: user.LevelEvaluatedAt,
		Metrics:          metrics,
	}, nil
}

// GetBreakdown 返回指标分解（后台用户详情）。实时求值，不写库。
func (s *trustService) GetBreakdown(ctx context.Context, userID string) (*XPBreakdown, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.USER_NOT_FOUND)
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	cfg, err := s.config(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	totalXP, contribs, streaks, adjustments, err := s.computeXP(ctx, userID, cfg)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	cache := map[int]model.LevelMetrics{}
	metricsFor := func(windowDays int) (model.LevelMetrics, error) {
		if m, ok := cache[windowDays]; ok {
			return m, nil
		}
		m, err := s.windowMetrics(ctx, userID, windowDays)
		if err != nil {
			return model.LevelMetrics{}, err
		}
		cache[windowDays] = m
		return m, nil
	}
	level, metrics, err := s.evaluateLevel(user, totalXP, metricsFor, cfg.levels)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return &XPBreakdown{
		UserTrust: UserTrust{
			UserID:           user.ID,
			TotalXP:          totalXP,
			Level:            level,
			LevelKey:         levelKey(cfg.levels, level),
			LevelName:        levelName(cfg.levels, level),
			LevelManual:      user.LevelManual,
			LevelEvaluatedAt: user.LevelEvaluatedAt,
			Metrics:          metrics,
		},
		Metrics:     contribs,
		Streaks:     streaks,
		Adjustments: adjustments,
	}, nil
}

// AdjustXP 写入一条人工/活动加减分记录并立即重算。
func (s *trustService) AdjustXP(ctx context.Context, actorID, userID string, delta int, reason string) (*model.XPAdjustment, error) {
	if userID == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "用户 ID 不能为空")
	}
	if delta == 0 {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "XP 增量不能为 0")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "必须填写调整原因")
	}
	if _, err := s.users.FindByID(ctx, userID); err != nil {
		return nil, pkg.NewAppError(pkg.USER_NOT_FOUND)
	}
	adj := model.NewXPAdjustment(userID, actorID, delta, reason)
	if err := s.trustRepo.CreateAdjustment(ctx, adj); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if _, err := s.Recompute(ctx, userID); err != nil {
		return nil, err
	}
	return adj, nil
}

func (s *trustService) ListAdjustments(ctx context.Context, userID string, page, pageSize int) ([]model.XPAdjustment, int, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	items, err := s.trustRepo.ListAdjustments(ctx, userID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	total, err := s.trustRepo.CountAdjustments(ctx, userID)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return items, int(total), nil
}

func (s *trustService) DeleteAdjustment(ctx context.Context, id string) error {
	adj, err := s.trustRepo.GetAdjustment(ctx, id)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "调整记录不存在")
	}
	if err := s.trustRepo.DeleteAdjustment(ctx, id); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	_, err = s.Recompute(ctx, adj.UserID)
	return err
}

// SetManualLevel 由管理员手工设级（manual=true）或解除手工锁定（manual=false）。
func (s *trustService) SetManualLevel(ctx context.Context, userID string, level int, manual bool) error {
	cfg, err := s.config(ctx)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	exists := false
	for _, l := range cfg.levels {
		if l.Level == level && l.Enabled {
			exists = true
			break
		}
	}
	if !exists {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "等级不存在或已停用")
	}
	if _, err := s.users.FindByID(ctx, userID); err != nil {
		return pkg.NewAppError(pkg.USER_NOT_FOUND)
	}
	if err := s.trustRepo.SetManualLevel(ctx, userID, level, manual); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if !manual {
		_, err = s.Recompute(ctx, userID)
		return err
	}
	return nil
}

func (s *trustService) UpdateLevel(ctx context.Context, userID string, level int) error {
	return s.SetManualLevel(ctx, userID, level, true)
}

// StartSyncSubscriber 订阅 trust:sync，清除本地配置快照。
func (s *trustService) StartSyncSubscriber(ctx context.Context) error {
	if s.kv == nil {
		return nil
	}
	ch, err := s.kv.Subscribe(ctx, trustSyncChannel)
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-ch:
				if !ok {
					return
				}
				s.mu.Lock()
				s.cfg = nil
				s.mu.Unlock()
			}
		}
	}()
	return nil
}

// StartDailyRecomputeTicker 启动每日全量重算，处理不活跃导致的衰减降级。
func (s *trustService) StartDailyRecomputeTicker(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	run := func() {
		if n, err := s.RecomputeAll(ctx); err != nil {
			logrus.Warnf("trust recompute: %v", err)
		} else {
			logrus.Infof("trust recompute: evaluated %d users", n)
		}
	}
	go func() {
		timer := time.NewTimer(2 * time.Minute)
		ticker := time.NewTicker(24 * time.Hour)
		defer timer.Stop()
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				run()
			case <-ticker.C:
				run()
			}
		}
	}()
	return cancel
}

// ── 配置读写（后台） ────────────────────────────────────────────────────────

func (s *trustService) ListLevels(ctx context.Context) ([]model.TrustLevel, error) {
	levels, err := s.trustRepo.ListLevels(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return levels, nil
}

func (s *trustService) SaveLevel(ctx context.Context, level *model.TrustLevel) error {
	if level == nil || level.Level < 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "等级必须为非负整数")
	}
	level.Name = strings.TrimSpace(level.Name)
	if level.Name == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "等级名称不能为空")
	}
	if level.MinXP < 0 || level.WindowDays < 0 || level.MinActiveDays < 0 || level.MinPosts < 0 || level.MinLikes < 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "门槛与窗口条件不能为负数")
	}
	if level.MaxReports != nil && *level.MaxReports < 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "举报上限不能为负数")
	}
	if level.Key == "" {
		level.Key = "tl" + strconv.Itoa(level.Level)
	}
	if err := s.trustRepo.UpsertLevel(ctx, level); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.invalidateConfig(ctx)
	return nil
}

func (s *trustService) DeleteLevel(ctx context.Context, level int) error {
	if level <= 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "不能删除等级 0（游客与新手所在等级）")
	}
	levels, err := s.trustRepo.ListLevels(ctx)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if len(levels) <= 1 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "至少保留一个等级")
	}
	if err := s.trustRepo.DeleteLevel(ctx, level); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.invalidateConfig(ctx)
	return nil
}

func (s *trustService) ListXPRules(ctx context.Context) ([]model.XPRule, error) {
	rules, err := s.trustRepo.ListXPRules(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	for i := range rules {
		rules[i].Label = metricLabel(rules[i].Metric)
	}
	return rules, nil
}

func (s *trustService) SaveXPRule(ctx context.Context, rule *model.XPRule) error {
	if rule == nil || !isKnownMetric(rule.Metric) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "未知指标")
	}
	if rule.Delta < 0 || rule.DailyCap < 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "XP 分值与日上限不能为负数")
	}
	if err := s.trustRepo.UpsertXPRule(ctx, rule); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.invalidateConfig(ctx)
	return nil
}

func (s *trustService) ListStreakBonuses(ctx context.Context) ([]model.XPStreakBonus, error) {
	rows, err := s.trustRepo.ListStreakBonuses(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return rows, nil
}

func (s *trustService) SaveStreakBonus(ctx context.Context, bonus *model.XPStreakBonus) error {
	if bonus == nil || bonus.Days <= 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "里程碑天数必须为正整数")
	}
	if bonus.Bonus < 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "奖励不能为负数")
	}
	if err := s.trustRepo.UpsertStreakBonus(ctx, bonus); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.invalidateConfig(ctx)
	return nil
}

func (s *trustService) DeleteStreakBonus(ctx context.Context, days int) error {
	if err := s.trustRepo.DeleteStreakBonus(ctx, days); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.invalidateConfig(ctx)
	return nil
}

// ── 辅助 ────────────────────────────────────────────────────────────────────

func metricLabel(metric string) string {
	if label, ok := model.XPMetricLabels[metric]; ok {
		return label
	}
	return metric
}

func isKnownMetric(metric string) bool {
	_, ok := model.XPMetricLabels[metric]
	return ok
}

func levelName(levels []model.TrustLevel, level int) string {
	for _, l := range levels {
		if l.Level == level {
			return l.Name
		}
	}
	return ""
}

func levelKey(levels []model.TrustLevel, level int) string {
	for _, l := range levels {
		if l.Level == level {
			return l.Key
		}
	}
	return ""
}
