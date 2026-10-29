package service

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// trustEnv 是 XP / 等级求值的真实 SQLite 测试环境：指标聚合走仓库层 SQL，
// 因此日上限、软删除回收、点赞/收藏回收这些行为得到的是端到端验证，而不是
// 在内存替身上自说自话。
func newTrustEnv(t *testing.T) (TrustService, db.Store) {
	t.Helper()
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "trust.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(
		&model.User{}, &model.Post{}, &model.Content{}, &model.ContentLike{},
		&model.PostFavorite{}, &model.CheckIn{}, &model.Report{}, &model.LoginHistory{},
		&model.XPRule{}, &model.XPStreakBonus{}, &model.TrustLevel{}, &model.XPAdjustment{},
	))
	svc := NewTrustService(repository.NewUserRepository(store), repository.NewTrustRepository(store), nil, nil)
	return svc, store
}

func createTrustUser(t *testing.T, store db.Store, id string) *model.User {
	t.Helper()
	u := &model.User{ID: id, Username: id, Email: id + "@example.com", Role: "user"}
	require.NoError(t, store.SQL().DB().Create(u).Error)
	return u
}

func seedRules(t *testing.T, store db.Store, rules ...model.XPRule) {
	t.Helper()
	require.NoError(t, store.SQL().DB().Exec("DELETE FROM xp_rules").Error)
	for i := range rules {
		require.NoError(t, store.SQL().DB().Create(&rules[i]).Error)
	}
}

func seedLevels(t *testing.T, store db.Store, levels ...model.TrustLevel) {
	t.Helper()
	require.NoError(t, store.SQL().DB().Exec("DELETE FROM trust_levels").Error)
	for i := range levels {
		require.NoError(t, store.SQL().DB().Create(&levels[i]).Error)
	}
}

func intp(v int) *int { return &v }

// TestTrustXP_DailyCapTruncatesPerNaturalDay 验证「按自然日 × 指标」截断：
// 同一天超过 daily_cap 的部分不计分，分两天则各自重新计数。
func TestTrustXP_DailyCapTruncatesPerNaturalDay(t *testing.T) {
	svc, store := newTrustEnv(t)
	ctx := context.Background()
	createTrustUser(t, store, "u1")
	seedRules(t, store, model.XPRule{Metric: model.MetricPostApproved, Delta: 10, DailyCap: 25, Enabled: true})

	now := time.Now()
	for i := 0; i < 30; i++ {
		p := &model.Post{
			ID: fmt.Sprintf("p-today-%d", i), Title: "t", ContentID: fmt.Sprintf("c-today-%d", i),
			AuthorID: "u1", Status: model.PostStatusApproved, CreatedAt: now, UpdatedAt: now,
		}
		require.NoError(t, store.SQL().DB().Create(p).Error)
	}
	trust, err := svc.Recompute(ctx, "u1")
	require.NoError(t, err)
	assert.Equal(t, 25*10, trust.TotalXP, "同一自然日只计 daily_cap 条")

	// 昨天的 30 条不受今日上限影响。
	yesterday := now.AddDate(0, 0, -1)
	for i := 0; i < 30; i++ {
		p := &model.Post{
			ID: fmt.Sprintf("p-yst-%d", i), Title: "t", ContentID: fmt.Sprintf("c-yst-%d", i),
			AuthorID: "u1", Status: model.PostStatusApproved, CreatedAt: yesterday, UpdatedAt: yesterday,
		}
		require.NoError(t, store.SQL().DB().Create(p).Error)
	}
	trust, err = svc.Recompute(ctx, "u1")
	require.NoError(t, err)
	assert.Equal(t, 2*25*10, trust.TotalXP)
}

// TestTrustXP_ReclaimsOnDeleteAndUnlike 验证撤销行为自动回收：删帖、取消点赞、
// 取消收藏都让 XP 下降，不需要任何「扣分」路径。
func TestTrustXP_ReclaimsOnDeleteAndUnlike(t *testing.T) {
	svc, store := newTrustEnv(t)
	ctx := context.Background()
	createTrustUser(t, store, "author")
	createTrustUser(t, store, "fan")
	seedRules(t, store,
		model.XPRule{Metric: model.MetricPostApproved, Delta: 10, Enabled: true},
		model.XPRule{Metric: model.MetricLikesReceived, Delta: 1, Enabled: true},
		model.XPRule{Metric: model.MetricFavoritesRecv, Delta: 2, Enabled: true},
	)

	now := time.Now()
	post := &model.Post{ID: "p1", Title: "t", ContentID: "pc1", AuthorID: "author", Status: model.PostStatusApproved, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, store.SQL().DB().Create(post).Error)
	body := &model.Content{ID: "pc1", AuthorID: "author", RootID: "p1", Status: model.PostStatusApproved, Body: "body", CreatedAt: now}
	require.NoError(t, store.SQL().DB().Create(body).Error)
	like := &model.ContentLike{ID: "l1", ContentID: "pc1", UserID: "fan", CreatedAt: now}
	require.NoError(t, store.SQL().DB().Create(like).Error)
	fav := &model.PostFavorite{ID: "f1", PostID: "p1", UserID: "fan", CreatedAt: now}
	require.NoError(t, store.SQL().DB().Create(fav).Error)

	trust, err := svc.Recompute(ctx, "author")
	require.NoError(t, err)
	assert.Equal(t, 10+1+2, trust.TotalXP)

	require.NoError(t, store.SQL().DB().Delete(&model.ContentLike{}, "id = ?", "l1").Error)
	trust, err = svc.Recompute(ctx, "author")
	require.NoError(t, err)
	assert.Equal(t, 10+2, trust.TotalXP, "取消点赞 MUST 回收")

	require.NoError(t, store.SQL().DB().Delete(&model.PostFavorite{}, "id = ?", "f1").Error)
	require.NoError(t, store.SQL().DB().Delete(&model.Post{}, "id = ?", "p1").Error)
	trust, err = svc.Recompute(ctx, "author")
	require.NoError(t, err)
	assert.Equal(t, 0, trust.TotalXP, "删帖与取消收藏 MUST 回收")
}

// TestTrustXP_OnlyApprovedContentCounts 验证待审 / 被拒内容不计分。
func TestTrustXP_OnlyApprovedContentCounts(t *testing.T) {
	svc, store := newTrustEnv(t)
	ctx := context.Background()
	createTrustUser(t, store, "u1")
	seedRules(t, store, model.XPRule{Metric: model.MetricPostApproved, Delta: 10, Enabled: true})

	now := time.Now()
	for i, status := range []string{model.PostStatusApproved, model.PostStatusPending, model.PostStatusRejected} {
		p := &model.Post{
			ID: fmt.Sprintf("p%d", i), Title: "t", ContentID: fmt.Sprintf("c%d", i),
			AuthorID: "u1", Status: status, CreatedAt: now, UpdatedAt: now,
		}
		require.NoError(t, store.SQL().DB().Create(p).Error)
	}
	trust, err := svc.Recompute(ctx, "u1")
	require.NoError(t, err)
	assert.Equal(t, 10, trust.TotalXP)
}

// TestTrustXP_RuleChangeAppliesToEveryoneImmediately 验证改分值后下一次求值按新
// 分值重算（而不是只影响新内容）。
func TestTrustXP_RuleChangeAppliesToEveryoneImmediately(t *testing.T) {
	svc, store := newTrustEnv(t)
	ctx := context.Background()
	createTrustUser(t, store, "u1")
	seedRules(t, store, model.XPRule{Metric: model.MetricPostApproved, Delta: 10, Enabled: true})

	now := time.Now()
	require.NoError(t, store.SQL().DB().Create(&model.Post{ID: "p1", Title: "t", ContentID: "c1", AuthorID: "u1", Status: model.PostStatusApproved, CreatedAt: now, UpdatedAt: now}).Error)

	trust, err := svc.Recompute(ctx, "u1")
	require.NoError(t, err)
	require.Equal(t, 10, trust.TotalXP)

	require.NoError(t, svc.SaveXPRule(ctx, &model.XPRule{Metric: model.MetricPostApproved, Delta: 20, Enabled: true}))
	trust, err = svc.Recompute(ctx, "u1")
	require.NoError(t, err)
	assert.Equal(t, 20, trust.TotalXP)
}

// TestTrustXP_StreakMilestonesCountedExactlyOnce 验证连续签到按「恰好达到」计一次，
// 同一轮不重复领取，断签重来可再得。
func TestTrustXP_StreakMilestonesCountedExactlyOnce(t *testing.T) {
	svc, store := newTrustEnv(t)
	ctx := context.Background()
	createTrustUser(t, store, "u1")
	seedRules(t, store) // 不配普通指标，只看阶梯
	require.NoError(t, store.SQL().DB().Exec("DELETE FROM xp_streak_bonuses").Error)
	require.NoError(t, store.SQL().DB().Create(&model.XPStreakBonus{Days: 3, Bonus: 3, Enabled: true}).Error)
	require.NoError(t, store.SQL().DB().Create(&model.XPStreakBonus{Days: 7, Bonus: 10, Enabled: true}).Error)

	// 一轮连续签到经过第 3、7 天；断签后第二轮又到第 7 天。
	rows := []model.CheckIn{
		{ID: "c1", UserID: "u1", CheckInDate: "2026-01-03", Streak: 3},
		{ID: "c2", UserID: "u1", CheckInDate: "2026-01-07", Streak: 7},
		{ID: "c3", UserID: "u1", CheckInDate: "2026-02-07", Streak: 7},
	}
	for i := range rows {
		require.NoError(t, store.SQL().DB().Create(&rows[i]).Error)
	}

	trust, err := svc.Recompute(ctx, "u1")
	require.NoError(t, err)
	assert.Equal(t, 3+10+10, trust.TotalXP, "3 天档一次 + 7 天档两次（两轮连续期）")
}

// TestTrustLevel_EvaluatesByXPAndRetentionWindow 验证定级 + 保级窗口：
// 达标但不满足活跃天数时不越级升级；活跃天数掉出窗口时逐级下探而非归零。
func TestTrustLevel_EvaluatesByXPAndRetentionWindow(t *testing.T) {
	svc, store := newTrustEnv(t)
	ctx := context.Background()
	createTrustUser(t, store, "u1")
	seedRules(t, store) // 只用人工调整控制 XP
	seedLevels(t, store,
		model.TrustLevel{Level: 0, Key: "new", Name: "新手上路", MinXP: 0, Enabled: true},
		model.TrustLevel{Level: 1, Key: "basic", Name: "基础", MinXP: 50, WindowDays: 100, Enabled: true},
		model.TrustLevel{Level: 2, Key: "member", Name: "成员", MinXP: 200, WindowDays: 100, MinActiveDays: 10, MaxReports: intp(5), Enabled: true},
	)

	_, err := svc.AdjustXP(ctx, "admin", "u1", 210, "活动奖励")
	require.NoError(t, err)
	trust, err := svc.GetSnapshot(ctx, "u1")
	require.NoError(t, err)
	assert.Equal(t, 1, trust.Level, "XP 到 200 但活跃天数不足，MUST NOT 越级升到 2")

	// 补 10 个活跃日（签到）后满足 TL2 保级条件。
	now := time.Now()
	for i := 0; i < 10; i++ {
		d := now.AddDate(0, 0, -i).Format("2006-01-02")
		require.NoError(t, store.SQL().DB().Create(&model.CheckIn{ID: fmt.Sprintf("ci%d", i), UserID: "u1", CheckInDate: d, Streak: 1}).Error)
	}
	trust, err = svc.Recompute(ctx, "u1")
	require.NoError(t, err)
	require.Equal(t, 2, trust.Level)

	// 活跃日掉出窗口（把签到日期全部改成 200 天前）→ 降级到仍合格的最高级 1。
	require.NoError(t, store.SQL().DB().Model(&model.CheckIn{}).Where(`user_id = ? AND check_in_date < ?`, "u1", now.AddDate(0, 0, -1).Format("2006-01-02")).
		Update("created_at", now.AddDate(0, 0, -200)).Error)
	for i := 0; i < 10; i++ {
		d := now.AddDate(0, 0, -200-i).Format("2006-01-02")
		require.NoError(t, store.SQL().DB().Model(&model.CheckIn{}).Where("id = ?", fmt.Sprintf("ci%d", i)).Update("check_in_date", d).Error)
	}
	trust, err = svc.Recompute(ctx, "u1")
	require.NoError(t, err)
	assert.Equal(t, 1, trust.Level, "保级失败 MUST 逐级下探而不是一次到底")
}

// TestTrustLevel_ManualOnlyAndAdminOverride 验证仅人工等级不自动升降，且管理员
// 手工设级不会被随后重算静默打回。
func TestTrustLevel_ManualOnlyAndAdminOverride(t *testing.T) {
	svc, store := newTrustEnv(t)
	ctx := context.Background()
	createTrustUser(t, store, "u1")
	createTrustUser(t, store, "u2")
	seedRules(t, store)
	seedLevels(t, store,
		model.TrustLevel{Level: 0, Key: "new", Name: "新手上路", MinXP: 0, Enabled: true},
		model.TrustLevel{Level: 1, Key: "basic", Name: "基础", MinXP: 50, Enabled: true},
		model.TrustLevel{Level: 2, Key: "leader", Name: "意见领袖", MinXP: 100, ManualOnly: true, Enabled: true},
	)

	_, err := svc.AdjustXP(ctx, "admin", "u1", 500, "活动奖励")
	require.NoError(t, err)
	trust, err := svc.GetSnapshot(ctx, "u1")
	require.NoError(t, err)
	assert.Equal(t, 1, trust.Level, "manual_only 等级 MUST NOT 自动进入")

	// 管理员手工设级 → 锁定；随后重算不得改回 1。
	require.NoError(t, svc.UpdateLevel(ctx, "u2", 2))
	_, err = svc.Recompute(ctx, "u2")
	require.NoError(t, err)
	user, err := repository.NewUserRepository(store).FindByID(ctx, "u2")
	require.NoError(t, err)
	assert.Equal(t, 2, user.TrustLevel, "管理员意图 MUST 优先")
	assert.True(t, user.LevelManual)

	// 解除锁定后回到自动求值。
	require.NoError(t, svc.SetManualLevel(ctx, "u2", 0, false))
	user, err = repository.NewUserRepository(store).FindByID(ctx, "u2")
	require.NoError(t, err)
	assert.Equal(t, 0, user.TrustLevel)
	assert.False(t, user.LevelManual)
}

// TestTrustRecompute_WritesSnapshotFields 验证派生缓存由唯一写入点落库，且
// 指标分解之和等于 total_xp。
func TestTrustRecompute_WritesSnapshotFields(t *testing.T) {
	svc, store := newTrustEnv(t)
	ctx := context.Background()
	createTrustUser(t, store, "u1")
	seedRules(t, store,
		model.XPRule{Metric: model.MetricPostApproved, Delta: 10, Enabled: true},
		model.XPRule{Metric: model.MetricCheckin, Delta: 2, Enabled: true},
	)
	seedLevels(t, store, model.TrustLevel{Level: 0, Key: "new", Name: "新手", MinXP: 0, Enabled: true})

	now := time.Now()
	require.NoError(t, store.SQL().DB().Create(&model.Post{ID: "p1", Title: "t", ContentID: "c1", AuthorID: "u1", Status: model.PostStatusApproved, CreatedAt: now, UpdatedAt: now}).Error)
	require.NoError(t, store.SQL().DB().Create(&model.CheckIn{ID: "ci1", UserID: "u1", CheckInDate: now.Format("2006-01-02"), Streak: 1}).Error)

	breakdown, err := svc.GetBreakdown(ctx, "u1")
	require.NoError(t, err)
	require.Equal(t, 12, breakdown.TotalXP)
	sum := breakdown.Adjustments
	for _, row := range breakdown.Metrics {
		sum += row.XP
	}
	for _, row := range breakdown.Streaks {
		sum += row.XP
	}
	assert.Equal(t, breakdown.TotalXP, sum, "指标分解之和 MUST 等于 total_xp")

	_, err = svc.Recompute(ctx, "u1")
	require.NoError(t, err)
	user, err := repository.NewUserRepository(store).FindByID(ctx, "u1")
	require.NoError(t, err)
	assert.Equal(t, 12, user.TotalXP)
	assert.NotNil(t, user.LevelEvaluatedAt)
	assert.Contains(t, user.LevelMetrics, "active_days")
}
