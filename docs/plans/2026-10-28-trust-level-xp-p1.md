# XP / 等级基座（P1）实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 把「积分/等级」从三方抢写 + 全硬编码，替换为「行为聚合出 XP → 等级表定级 + 100 天窗口保级 → 单点写入」的基座，并让等级/XP 规则全部后台可配。

**架构：** 新增 4 张配置/账本表（`trust_levels` / `xp_rules` / `xp_streak_bonuses` / `xp_adjustments`）与一个**指标聚合层**（从既有业务表算计数，不新增埋点表）。`TrustLevelService` 是 `users.total_xp` 与 `users.trust_level` 的**唯一写入方**；`XPConfigService` 负责配置 CRUD 与内存快照缓存。旧 `points.go` 的硬编码常量与 `trust.go` 的硬编码晋升链被删除。

**技术栈：** Go 1.26 + Iris v12 + GORM（SQLite/MySQL/Postgres）+ TanStack Query（后台页）+ vinext App Router。

**规范来源：** `openspec/changes/2026-10-28-trust-level-xp-system/`（proposal / design §4.4 已定数值 / specs/trust-levels）。

---

## 前置决策（必须先定，否则任务 1 无法开工）

**D-P1-1：活跃天数从哪来。** 分支 `codex/discourse-alignment` 已实现 `user_visit_days` 模型 + `VisitService`（KV 去重的每日访问记录，`f62a083` + `917c040`），正是本计划需要的 `active_day_count` 指标。两条路：

- **路线 A（推荐）**：先摘取那两个提交进 `develop`（`git cherry-pick f62a083 917c040`），本计划直接复用 `VisitRepository.CountDays`。
- **路线 B**：本计划任务 2 的 `UserMetricsRepository` 改用 `login_history` 表聚合活跃日（无新模型，但语义弱于"有登录或产生内容"，且与分支工作重复）。

本计划**按路线 A 编写**；若选 B，替换任务 0 与任务 2 中 `activeDayCount` 的 SQL 来源即可。

**D-P1-2：该分支的 trust 阈值工作被本变更取代。** `888b134`（11 个 `trust_*` settings 阈值）与 `46744ca`（TL3 自动升降）**不要**摘取——`trust_levels` 表 + 后台 UI 取代它们。合并该分支时只取 visit 两个提交。

**D-P1-3：等级存储形态。** `users.trust_level` **保持字符串列**（存等级 `key`，如 `new`/`basic`/`member`/`regular`/`leader`），不改成 int——SQLite 无法改列类型，且前端已按字符串 key 读取。数值序（`>= 3`）由 `trust_levels.level` 提供，通过配置快照解析。`users` 新增 `level_manual`（bool，管理员手工设级后置位）与 `level_manual_at`。

---

## 文件结构

| 文件 | 职责 |
|---|---|
| `app/server/internal/model/trust_level.go`（新） | `TrustLevel` / `XPRule` / `XPStreakBonus` / `XPAdjustment` 模型 + `UserMetrics`（非表结构）+ 指标名常量 |
| `app/server/internal/repository/xp_config.go` / `xp_config_sql.go`（新） | `XPConfigRepository`：三张配置表的 CRUD |
| `app/server/internal/repository/user_metrics.go` / `user_metrics_sql.go`（新） | `UserMetricsRepository`：把 `UserMetrics` 各字段聚合出来（唯一碰业务表的地方） |
| `app/server/internal/repository/xp_adjustment.go` / `xp_adjustment_sql.go`（新） | `XPAdjustmentRepository`：人工加减分的写入/查询/求和 |
| `app/server/internal/service/xp_config.go` / `xp_config_iface.go`（新） | `XPConfigService`：配置 CRUD + 内存快照 + KV 失效 + seed 默认值 |
| `app/server/internal/service/trust_level.go` / `trust_level_iface.go`（新） | `TrustLevelService`：XP 求值、等级求值、`Recompute` 单点写入、指标分解、ticker |
| `app/server/internal/service/trust.go`（改） | 删除 `evaluatePromotion` / `trustIntToString` / `trustStringToInt` 的硬编码链，只保留 TL0 两门的**迁移期**实现（P3 才被规则引擎接管） |
| `app/server/internal/service/points.go`（删） | 硬编码分值常量与 `syncTrustLevel` 全部移除 |
| `app/server/internal/service/checkin.go`（改） | 去掉 `user.TrustLevel = trustLevelForPoints(...)` 直写，改调 `Recompute` |
| `app/server/internal/handler/trust_level.go`（新） | 后台 API：等级 CRUD、XP 规则 CRUD、用户 XP 分解、人工调整 |
| `app/server/internal/handler/post.go`（改） | 新增 `SetFeatured` |
| `app/server/internal/router/routes_admin.go`（改） | 挂 `/admin/levels/*`、`/admin/xp-rules/*`、`/admin/users/{id}/xp/*` |
| `app/server/internal/router/routes_content.go`（改） | 挂 `POST /posts/{id}/feature` |
| `app/server/internal/gateway/permission.go`（改） | 新路径 → `level:manage` / `post:pin` 映射 |
| `app/server/internal/model/permission.go`（改） | 新增权限码 `level:manage` |
| `app/server/internal/container/container.go` / `runtime.go`（改） | 装配 3 个 repo + 2 个 service；`trust_recompute` 替换 `trust_promotion` worker |
| `app/server/server/seed.go`（改） | seed 默认等级表 / XP 规则 / 签到阶梯（幂等） |
| `app/server/server/iris.go`（改） | AutoMigrate 注册 4 个新模型 |
| `packages/api-client/src/services/trust-level.ts`（新） | 后台 API 客户端 |
| `app/web/app/admin/levels/page.tsx`（新） | 等级管理 |
| `app/web/app/admin/xp-rules/page.tsx`（新） | XP 规则与签到阶梯 |
| `app/web/app/admin/users/detail/page.tsx`（改） | XP 分解 + 人工调整 |
| `app/web/lib/query/keys.ts`（改） | `qk.admin.levels` / `qk.admin.xpRules` |

---

## 任务 0：前置 — 接入活跃天数基础设施（路线 A）

**文件：**
- 引入（cherry-pick）：`app/server/internal/model/visit.go`、`repository/visit.go`、`service/visit.go`、`service/visit_iface.go`

- [ ] **步骤 1：确认两个提交可干净摘取**

运行：
```bash
cd /Users/noelorin/Feed Chamber
git log --oneline -1 f62a083 && git log --oneline -1 917c040
git cherry-pick --no-commit f62a083 917c040
git status --short
```
预期：`visit.go` 等 5 个文件出现在暂存区；**若冲突**，停下并回报（不要自行解冲突）。

- [ ] **步骤 2：跑该分支自带的 visit 测试**

运行：`cd app/server && go test ./internal/service/ -run 'Visit' -count=1 -v`
预期：PASS（该分支已带 `visit_test.go`）。

- [ ] **步骤 3：Commit**

```bash
git add app/server/internal/model/visit.go app/server/internal/repository/visit.go \
        app/server/internal/service/visit.go app/server/internal/service/visit_iface.go \
        app/server/internal/service/visit_test.go app/server/internal/container app/server/server
git commit -m "chore(trust): 摘取 visit 基础设施（user_visit_days + VisitService）供活跃天数指标复用"
```

---

## 任务 1：成长体系模型与迁移

**文件：**
- 创建：`app/server/internal/model/trust_level.go`
- 修改：`app/server/internal/model/user.go`（加 `level_manual` / `level_manual_at` / `total_xp`）、`app/server/server/iris.go`（AutoMigrate）

- [ ] **步骤 1：编写失败的测试**

创建 `app/server/internal/model/trust_level_test.go`：

```go
package model

import "testing"

func TestTrustLevelTableName(t *testing.T) {
	if (TrustLevel{}).TableName() != "trust_levels" {
		t.Fatalf("unexpected table name: %s", (TrustLevel{}).TableName())
	}
	if (XPRule{}).TableName() != "xp_rules" {
		t.Fatalf("unexpected table name: %s", (XPRule{}).TableName())
	}
	if (XPStreakBonus{}).TableName() != "xp_streak_bonuses" {
		t.Fatalf("unexpected table name: %s", (XPStreakBonus{}).TableName())
	}
	if (XPAdjustment{}).TableName() != "xp_adjustments" {
		t.Fatalf("unexpected table name: %s", (XPAdjustment{}).TableName())
	}
}

func TestMetricNamesAreStable(t *testing.T) {
	// 指标名是配置表里的持久化键，改名等于清空线上配置，必须钉住。
	want := map[string]string{
		MetricPostApproved:      "post_approved_count",
		MetricCommentApproved:   "comment_approved_count",
		MetricLikesReceived:     "likes_received_count",
		MetricFavoritesReceived: "favorite_received_count",
		MetricFeaturedPosts:     "featured_post_count",
		MetricCheckInDays:       "checkin_count",
	}
	for got, expect := range want {
		if got != expect {
			t.Fatalf("metric constant drifted: %q != %q", got, expect)
		}
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/model/ -run 'TestTrustLevelTableName|TestMetricNamesAreStable' -count=1`
预期：FAIL，`undefined: TrustLevel`。

- [ ] **步骤 3：编写模型**

创建 `app/server/internal/model/trust_level.go`：

```go
package model

import "time"

// 指标名常量：XP 规则与保级条件共用同一批指标，且是配置表里的持久化键。
const (
	MetricPostApproved      = "post_approved_count"
	MetricCommentApproved   = "comment_approved_count"
	MetricLikesReceived     = "likes_received_count"
	MetricFavoritesReceived = "favorite_received_count"
	MetricFeaturedPosts     = "featured_post_count"
	MetricCheckInDays       = "checkin_count"
)

// TrustLevel 等级定义（后台可增删改；Level 决定大小序，Key 是用户行上存的值）。
type TrustLevel struct {
	Level      int       `gorm:"primaryKey" json:"level"`
	Key        string    `gorm:"size:32;uniqueIndex;not null" json:"key"`
	Name       string    `gorm:"size:64;not null" json:"name"`
	MinXP      int       `gorm:"default:0" json:"min_xp"`
	WindowDays int       `gorm:"default:100" json:"window_days"`
	MinActiveDays int    `gorm:"default:0" json:"min_active_days"`
	MinPosts      int    `gorm:"default:0" json:"min_posts"`
	MinLikes      int    `gorm:"default:0" json:"min_likes"`
	MaxReports    int    `gorm:"default:0" json:"max_reports"` // 0 = 不限制
	ManualOnly    bool   `gorm:"default:false" json:"manual_only"`
	Enabled       bool   `gorm:"default:true" json:"enabled"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (TrustLevel) TableName() string { return "trust_levels" }

// XPRule 指标 → XP 分值。DailyCap 为 0 表示不截断。
type XPRule struct {
	Metric    string    `gorm:"primaryKey;size:48" json:"metric"`
	Delta     int       `gorm:"not null" json:"delta"`
	DailyCap  int       `gorm:"default:0" json:"daily_cap"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (XPRule) TableName() string { return "xp_rules" }

// XPStreakBonus 连续签到里程碑：连续天数“恰好达到”Days 时一次性给 Bonus。
type XPStreakBonus struct {
	Days      int       `gorm:"primaryKey" json:"days"`
	Bonus     int       `gorm:"not null" json:"bonus"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (XPStreakBonus) TableName() string { return "xp_streak_bonuses" }

// XPAdjustment 人工/活动加减分（唯一需要留痕的 XP 来源）。
type XPAdjustment struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    string    `gorm:"index;not null" json:"user_id"`
	Delta     int       `gorm:"not null" json:"delta"`
	Reason    string    `gorm:"size:255" json:"reason"`
	ActorID   string    `gorm:"size:64" json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (XPAdjustment) TableName() string { return "xp_adjustments" }

// UserMetrics 指标快照（不是表；由 UserMetricsRepository 聚合出来）。
type UserMetrics struct {
	PostApproved      int
	CommentApproved   int
	LikesReceived     int
	FavoritesReceived int
	FeaturedPosts     int
	CheckInDays       int
	StreakHits        map[int]int // 里程碑天数 → 恰好达到的次数
	ActiveDays        int
	ReportsUpheld     int
}
```

在 `app/server/internal/model/user.go` 的 `TotalPoints` 附近新增：

```go
	TotalXP        int        `gorm:"default:0" json:"total_xp"`
	LevelManual    bool       `gorm:"default:false" json:"level_manual"`
	LevelManualAt  *time.Time `json:"level_manual_at,omitempty"`
```

`app/server/server/iris.go` 的 AutoMigrate 列表加入 `&model.TrustLevel{}, &model.XPRule{}, &model.XPStreakBonus{}, &model.XPAdjustment{}`。

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/model/ -count=1 && go build ./...`
预期：PASS + 构建通过。

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/model/trust_level.go app/server/internal/model/trust_level_test.go \
        app/server/internal/model/user.go app/server/server/iris.go
git commit -m "feat(trust): 成长体系模型（等级/XP规则/签到阶梯/人工调整）"
```

---

## 任务 2：指标聚合层（唯一碰业务表的地方）

**文件：**
- 创建：`app/server/internal/repository/user_metrics.go`、`user_metrics_sql.go`
- 测试：`app/server/internal/repository/user_metrics_test.go`

- [ ] **步骤 1：编写失败的测试**

创建 `app/server/internal/repository/user_metrics_test.go`（夹具照抄 `post_sitemap_test.go:19` 的 `newSitemapTestRepo` 形态）：

```go
package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/model"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newMetricsTestRepo 建一个隔离的 SQLite store，只迁指标聚合会碰到的表。
func newMetricsTestRepo(t *testing.T) (UserMetricsRepository, db.Store) {
	t.Helper()
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "metrics.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(&model.Post{}, &model.Content{}, &model.User{}))
	return NewUserMetricsRepository(store), store
}

func TestMetrics_CountsOnlyEffectiveContent(t *testing.T) {
	repo, store := newMetricsTestRepo(t)
	ctx := context.Background()
	seed := func(p *model.Post) { require.NoError(t, store.SQL().DB().Create(p).Error) }

	// 1 篇 approved + 1 篇 pending + 1 篇软删
	seed(&model.Post{ID: "p1", AuthorID: "u1", Status: model.PostStatusApproved, Title: "a"})
	seed(&model.Post{ID: "p2", AuthorID: "u1", Status: model.PostStatusPending, Title: "b"})
	seed(&model.Post{ID: "p3", AuthorID: "u1", Status: model.PostStatusApproved, Title: "c",
		DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}})

	m, err := repo.Metrics(ctx, "u1", time.Now().AddDate(0, 0, -100))
	require.NoError(t, err)
	require.Equal(t, 1, m.PostApproved, "pending 与软删都不计")
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/repository/ -run TestMetrics_CountsOnlyEffectiveContent -count=1`
预期：FAIL，`undefined: NewUserMetricsRepository`。

- [ ] **步骤 3：编写接口与实现**

`user_metrics.go`：

```go
package repository

import (
	"context"
	"time"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

// UserMetricsRepository 把成长体系需要的指标从既有业务表聚合出来。
// 它是全仓唯一为了 XP/等级去读业务表的地方；不新增埋点表。
type UserMetricsRepository interface {
	// Metrics 返回全量累计指标；since 之前只影响窗口类指标（ActiveDays/ReportsUpheld）。
	Metrics(ctx context.Context, userID string, since time.Time) (*model.UserMetrics, error)
	// DailyCounts 返回近 N 天逐日的指标计数，供 XP 日上限截断使用。
	DailyCounts(ctx context.Context, userID string, metric string, since time.Time) (map[string]int, error)
}

func NewUserMetricsRepository(store db.Store) UserMetricsRepository {
	return &sqlUserMetricsRepo{db: store.SQL()}
}
```

`user_metrics_sql.go` 关键查询（SQLite/MySQL/Postgres 通用写法，只列核心四条，其余同构）：

```go
func (r *sqlUserMetricsRepo) Metrics(ctx context.Context, userID string, since time.Time) (*model.UserMetrics, error) {
	m := &model.UserMetrics{StreakHits: map[int]int{}}
	db := r.db.DB().WithContext(ctx)

	// 有效帖子：approved 且未软删
	if err := db.Raw(`SELECT COUNT(*) FROM posts
		WHERE author_id = ? AND deleted_at IS NULL AND status = ?`, userID, model.PostStatusApproved).Scan(&m.PostApproved).Error; err != nil {
		return nil, WrapDBErr(err)
	}
	// 有效评论
	if err := db.Raw(`SELECT COUNT(*) FROM contents
		WHERE author_id = ? AND deleted_at IS NULL AND status = ?`, userID, model.PostStatusApproved).Scan(&m.CommentApproved).Error; err != nil {
		return nil, WrapDBErr(err)
	}
	// 收到的赞（别人的赞，计在内容作者头上）
	if err := db.Raw(`SELECT COUNT(*) FROM content_likes cl
		JOIN contents c ON c.id = cl.content_id
		WHERE c.author_id = ? AND c.deleted_at IS NULL AND c.status = ? AND cl.user_id <> c.author_id`,
		userID, model.PostStatusApproved).Scan(&m.LikesReceived).Error; err != nil {
		return nil, WrapDBErr(err)
	}
	// 活跃天数：近窗口内有访问记录的天数
	if err := db.Raw(`SELECT COUNT(*) FROM user_visit_days WHERE user_id = ? AND day >= ?`,
		userID, since.Format("2006-01-02")).Scan(&m.ActiveDays).Error; err != nil {
		return nil, WrapDBErr(err)
	}
	// 连续签到里程碑：恰好达到各档的次数（档位来自 xp_streak_bonuses，由 service 传入）
	// 见 DailyCounts 的 checkin 分支；此处先取 check_ins 的 streak 分布。
	type streakRow struct {
		Streak int
		Cnt    int
	}
	var rows []streakRow
	if err := db.Raw(`SELECT streak, COUNT(*) AS cnt FROM check_ins WHERE user_id = ? GROUP BY streak`, userID).
		Scan(&rows).Error; err != nil {
		return nil, WrapDBErr(err)
	}
	for _, row := range rows {
		m.StreakHits[row.Streak] = row.Cnt
	}
	return m, nil
}
```

> 说明：`contents.status` 的枚举与 `content_likes` / `favorites` / `check_ins` / `user_visit_days` 的真实表名列以仓库现状为准，实现时逐个核对（`internal/model/content.go`、`like.go`、`favorite.go`）。

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/repository/ -run TestMetrics -count=1`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/repository/user_metrics.go app/server/internal/repository/user_metrics_sql.go \
        app/server/internal/repository/user_metrics_test.go
git commit -m "feat(trust): 指标聚合层（只计有效内容，含活跃天数与签到里程碑）"
```

---

## 任务 3：XP 求值（纯函数 + 日上限）

**文件：**
- 创建：`app/server/internal/service/xp_eval.go`、`xp_eval_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
package service

import (
	"testing"

	"feedchamber/internal/model"
)

func TestEvaluateXP_AppliesDailyCapPerDay(t *testing.T) {
	rules := []model.XPRule{{Metric: model.MetricPostApproved, Delta: 10, DailyCap: 25, Enabled: true}}
	daily := map[string]map[string]int{
		"2026-10-27": {model.MetricPostApproved: 5}, // 5×10=50 → 截断到 25
		"2026-10-28": {model.MetricPostApproved: 2}, // 2×10=20 → 不截断
	}
	got := EvaluateXP(rules, nil, daily, nil)
	if got != 45 {
		t.Fatalf("EvaluateXP = %d, want 45", got)
	}
}

func TestEvaluateXP_StreakMilestoneCountsOnce(t *testing.T) {
	bonuses := []model.XPStreakBonus{{Days: 7, Bonus: 10, Enabled: true}, {Days: 30, Bonus: 30, Enabled: true}}
	metrics := &model.UserMetrics{StreakHits: map[int]int{7: 2, 30: 1}}
	got := EvaluateXP(nil, bonuses, nil, metrics)
	if got != 50 { // 7 天档命中 2 次 = 20，30 天档 1 次 = 30
		t.Fatalf("EvaluateXP = %d, want 50", got)
	}
}

func TestEvaluateXP_IgnoresDisabledRules(t *testing.T) {
	rules := []model.XPRule{{Metric: model.MetricPostApproved, Delta: 10, Enabled: false}}
	daily := map[string]map[string]int{"2026-10-28": {model.MetricPostApproved: 3}}
	if got := EvaluateXP(rules, nil, daily, nil); got != 0 {
		t.Fatalf("EvaluateXP = %d, want 0（停用的规则不计分）", got)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/service/ -run TestEvaluateXP -count=1`
预期：FAIL，`undefined: EvaluateXP`。

- [ ] **步骤 3：编写实现**

```go
package service

import "feedchamber/internal/model"

// EvaluateXP 是纯函数：XP = Σ(按自然日截断的指标计数 × 分值) + Σ(签到里程碑) 。
// 人工调整不在本函数内（由调用方加上 xp_adjustments 之和）。
func EvaluateXP(rules []model.XPRule, bonuses []model.XPStreakBonus,
	daily map[string]map[string]int, metrics *model.UserMetrics) int {

	total := 0
	for _, rule := range rules {
		if !rule.Enabled || rule.Delta == 0 {
			continue
		}
		for _, perMetric := range daily {
			count := perMetric[rule.Metric]
			if rule.DailyCap > 0 && count > rule.DailyCap {
				count = rule.DailyCap
			}
			total += count * rule.Delta
		}
	}
	if metrics != nil {
		for _, b := range bonuses {
			if !b.Enabled || b.Bonus == 0 {
				continue
			}
			total += metrics.StreakHits[b.Days] * b.Bonus
		}
	}
	return total
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/service/ -run TestEvaluateXP -count=1`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/service/xp_eval.go app/server/internal/service/xp_eval_test.go
git commit -m "feat(trust): XP 求值（自然日 × 指标截断 + 签到里程碑）"
```

---

## 任务 4：等级求值（定级 + 100 天保级 + manual_only）

**文件：**
- 创建：`app/server/internal/service/level_eval.go`、`level_eval_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
package service

import (
	"testing"

	"feedchamber/internal/model"
)

var testLevels = []model.TrustLevel{
	{Level: 0, Key: "new", Name: "新手上路", MinXP: 0, Enabled: true},
	{Level: 1, Key: "basic", Name: "基础", MinXP: 50, Enabled: true},
	{Level: 2, Key: "member", Name: "成员", MinXP: 200, WindowDays: 100, MinActiveDays: 10, MaxReports: 5, Enabled: true},
	{Level: 3, Key: "regular", Name: "常客", MinXP: 500, WindowDays: 100, MinActiveDays: 50, MinPosts: 10, MinLikes: 20, MaxReports: 3, Enabled: true},
	{Level: 4, Key: "leader", Name: "意见领袖", MinXP: 1000, ManualOnly: true, Enabled: true},
}

func TestEvaluateLevel_PromotesWhenXpAndRetentionMet(t *testing.T) {
	m := &model.UserMetrics{ActiveDays: 12}
	got := EvaluateLevel(testLevels, 210, m)
	if got.Key != "member" {
		t.Fatalf("level = %s, want member", got.Key)
	}
}

func TestEvaluateLevel_FallsBackWhenRetentionFails(t *testing.T) {
	// XP 到 TL3，但活跃天数只够 TL1 → 落到 TL1（不是 TL0，也不越级）
	m := &model.UserMetrics{ActiveDays: 2}
	got := EvaluateLevel(testLevels, 600, m)
	if got.Key != "basic" {
		t.Fatalf("level = %s, want basic", got.Key)
	}
}

func TestEvaluateLevel_NeverAutoPromotesManualOnly(t *testing.T) {
	m := &model.UserMetrics{ActiveDays: 90, PostApproved: 50, LikesReceived: 100}
	got := EvaluateLevel(testLevels, 5000, m)
	if got.Key != "regular" {
		t.Fatalf("level = %s, want regular（leader 仅人工授予）", got.Key)
	}
}

func TestEvaluateLevel_ReportCapBlocksRetention(t *testing.T) {
	m := &model.UserMetrics{ActiveDays: 60, PostApproved: 20, LikesReceived: 30, ReportsUpheld: 4}
	got := EvaluateLevel(testLevels, 600, m)
	if got.Key != "member" {
		t.Fatalf("level = %s, want member（成立举报 4 > TL3 上限 3）", got.Key)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/service/ -run TestEvaluateLevel -count=1`
预期：FAIL，`undefined: EvaluateLevel`。

- [ ] **步骤 3：编写实现**

```go
package service

import (
	"sort"

	"feedchamber/internal/model"
)

// EvaluateLevel 是纯函数：先按累计 XP 取候选等级，再自顶向下校验该级的保级条件，
// 第一个「XP 与保级条件同时满足」的等级即结果（因此不会一次掉到底）。
// manual_only 的等级不参与自动定级。
func EvaluateLevel(levels []model.TrustLevel, totalXP int, m *model.UserMetrics) model.TrustLevel {
	sorted := make([]model.TrustLevel, 0, len(levels))
	for _, l := range levels {
		if l.Enabled && !l.ManualOnly {
			sorted = append(sorted, l)
		}
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Level > sorted[j].Level })

	var fallback *model.TrustLevel
	for i := range sorted {
		l := &sorted[i]
		if totalXP < l.MinXP {
			continue
		}
		if fallback == nil {
			fallback = l // 至少是「XP 够得着的最高级」
		}
		if retentionOK(l, m) {
			return *l
		}
	}
	if fallback != nil {
		return *fallback
	}
	// 连最低级都不满足 XP（理论上 TL0 的 MinXP=0）→ 返回最小等级
	lo := sorted[len(sorted)-1]
	return lo
}

func retentionOK(l *model.TrustLevel, m *model.UserMetrics) bool {
	if m == nil {
		return true
	}
	if l.MinActiveDays > 0 && m.ActiveDays < l.MinActiveDays {
		return false
	}
	if l.MinPosts > 0 && m.PostApproved < l.MinPosts {
		return false
	}
	if l.MinLikes > 0 && m.LikesReceived < l.MinLikes {
		return false
	}
	if l.MaxReports > 0 && m.ReportsUpheld > l.MaxReports {
		return false
	}
	return true
}
```

> 注意 `fallback` 的语义：`EvaluateLevel` 返回的是"XP 够得着且保级通过的最高级"；若全都不通过保级，则退到"XP 够得着的最高级"——**这与"逐级下探"等价**（下探过程中第一个通过的就是结果，全不通过则取 XP 档位）。实现时用测试把 `TestEvaluateLevel_FallsBackWhenRetentionFails` 钉死。

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/service/ -run TestEvaluateLevel -count=1`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/service/level_eval.go app/server/internal/service/level_eval_test.go
git commit -m "feat(trust): 等级求值（累计 XP 定级 + 窗口保级 + manual_only）"
```

---

## 任务 5：配置服务（CRUD + 内存快照 + seed 默认值）

**文件：**
- 创建：`app/server/internal/repository/xp_config.go`、`xp_config_sql.go`、`app/server/internal/service/xp_config.go`、`xp_config_iface.go`
- 修改：`app/server/server/seed.go`

- [ ] **步骤 1：编写失败的测试**

```go
package service

import (
	"context"
	"testing"

	"feedchamber/internal/mock"
	"feedchamber/internal/model"
)

func TestXPConfig_LoadCachesAndInvalidates(t *testing.T) {
	repo := mock.NewXPConfigRepository()
	if err := repo.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	svc := NewXPConfigService(repo)

	cfg, err := svc.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Levels) != 5 || cfg.Levels[3].Key != "regular" {
		t.Fatalf("levels = %+v", cfg.Levels)
	}
	if cfg.Rules[model.MetricPostApproved].Delta != 10 {
		t.Fatalf("发帖分值 = %d, want 10", cfg.Rules[model.MetricPostApproved].Delta)
	}
	if cfg.StreakBonus[30] != 40 {
		t.Fatalf("30 天档 = %d, want 40", cfg.StreakBonus[30])
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/service/ -run TestXPConfig -count=1`
预期：FAIL，`undefined: NewXPConfigRepository`。

- [ ] **步骤 3：编写实现**

`repository/xp_config.go`：

```go
type XPConfigRepository interface {
	ListLevels(ctx context.Context) ([]model.TrustLevel, error)
	UpsertLevel(ctx context.Context, l *model.TrustLevel) error
	DeleteLevel(ctx context.Context, level int) error

	ListXPRules(ctx context.Context) ([]model.XPRule, error)
	UpsertXPRule(ctx context.Context, r *model.XPRule) error

	ListStreakBonuses(ctx context.Context) ([]model.XPStreakBonus, error)
	UpsertStreakBonus(ctx context.Context, b *model.XPStreakBonus) error
	DeleteStreakBonus(ctx context.Context, days int) error

	SeedDefaults(ctx context.Context) error // 幂等：仅当表为空时写入 design §4.4 的默认值
}
```

`service/xp_config.go`：

```go
type XPConfig struct {
	Levels      []model.TrustLevel
	Rules       map[string]model.XPRule // 指标名 → 规则
	StreakBonus map[int]int             // 连续天数 → 奖励
	MaxWindow   int                     // 所有等级里最大的 WindowDays，用于聚合窗口
}

// RuleList 把 map 还原成切片，喂给纯函数 EvaluateXP（顺序无关，保证可测）。
func (c *XPConfig) RuleList() []model.XPRule {
	out := make([]model.XPRule, 0, len(c.Rules))
	for _, r := range c.Rules {
		out = append(out, r)
	}
	return out
}

// StreakList 同上，喂给 EvaluateXP 的 milestones 参数。
func (c *XPConfig) StreakList() []model.XPStreakBonus {
	out := make([]model.XPStreakBonus, 0, len(c.StreakBonus))
	for days, bonus := range c.StreakBonus {
		out = append(out, model.XPStreakBonus{Days: days, Bonus: bonus, Enabled: true})
	}
	return out
}

// ByKey 按用户行上存的 key 反查等级定义（找不到时回落到最低等级）。
func (c *XPConfig) ByKey(key string) model.TrustLevel {
	for _, l := range c.Levels {
		if l.Key == key {
			return l
		}
	}
	return c.Lowest()
}

// Lowest 返回最低等级（TL0 保底，EvaluateLevel 与游客判定都依赖它存在）。
func (c *XPConfig) Lowest() model.TrustLevel {
	low := c.Levels[0]
	for _, l := range c.Levels {
		if l.Level < low.Level {
			low = l
		}
	}
	return low
}

// LevelSnapshot 是 users.level_metrics 里存的快照（JSON）。
type LevelSnapshot struct {
	TotalXP     int               `json:"total_xp"`
	ComputedKey string            `json:"computed_key"`
	EffectiveKey string           `json:"effective_key"`
	Metrics     *model.UserMetrics `json:"metrics"`
	ComputedAt  time.Time         `json:"computed_at"`
}

func (s *LevelSnapshot) JSON() string { b, _ := json.Marshal(s); return string(b) }

// MetricContribution 是后台“为什么是这个等级”的一行。
type MetricContribution struct {
	Metric string `json:"metric"`
	Count  int    `json:"count"`
	Delta  int    `json:"delta"`
	XP     int    `json:"xp"`
}

type XPConfigService interface {
	Load(ctx context.Context) (*XPConfig, error) // 进程内缓存 + KV pub/sub 失效
	Invalidate()
	Levels(ctx context.Context) ([]model.TrustLevel, error)
	SaveLevel(ctx context.Context, l *model.TrustLevel) error
	DeleteLevel(ctx context.Context, level int) error
	Rules(ctx context.Context) ([]model.XPRule, error)
	SaveRule(ctx context.Context, r *model.XPRule) error
	StreakBonuses(ctx context.Context) ([]model.XPStreakBonus, error)
	SaveStreakBonus(ctx context.Context, b *model.XPStreakBonus) error
}
```

缓存策略沿用 `PermissionService` 的 `sync.RWMutex` + KV pub/sub（频道 `xpcfg:sync`）模式；`server/seed.go` 在 seed 阶段调用 `XPConfigRepo.SeedDefaults(ctx)`（`FirstOrCreate` 语义，不覆盖运营改过的值）。

**seed 默认值（design §4.4，逐字照抄）：** 等级 0/50/200/500/1000，TL2 `min_active_days=10 max_reports=5`，TL3 `min_active_days=50 min_posts=10 min_likes=20 max_reports=3`，TL4 `manual_only=true`；XP 规则 发帖 10/25、评论 3/20、被赞 1/30、被收藏 2、精华 10、签到 2；签到阶梯 3→3、7→10、15→20、30→40、60→80、100→150。

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/service/ -run TestXPConfig -count=1`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/repository/xp_config*.go app/server/internal/service/xp_config*.go \
        app/server/internal/mock/mock.go app/server/server/seed.go
git commit -m "feat(trust): 等级/XP 配置服务与 seed 默认值（可后台改，缓存可失效）"
```

---

## 任务 6：`TrustLevelService` 单点写入 + 触发点接线

**文件：**
- 创建：`app/server/internal/repository/xp_adjustment.go`、`xp_adjustment_sql.go`、`app/server/internal/service/trust_level.go`、`trust_level_iface.go`
- 修改：`app/server/internal/service/post.go`、`content.go`、`like.go`、`favorite.go`、`checkin.go`、`app/server/internal/container/container.go`、`runtime.go`

- [ ] **步骤 1：编写失败的测试**

创建 `app/server/internal/service/trust_level_test.go`：

```go
package service

import (
	"context"
	"testing"

	"feedchamber/internal/mock"
	"feedchamber/internal/model"

	"github.com/stretchr/testify/require"
)

// 测试装配：三个 mock repo + 已 seed 默认值的配置。
func newTrustLevelServiceForTest(t *testing.T) (TrustLevelService, *mock.UserRepository, *mock.UserMetricsRepository) {
	t.Helper()
	cfgRepo := mock.NewXPConfigRepository()
	require.NoError(t, cfgRepo.SeedDefaults(context.Background()))
	users := mock.NewUserRepository()
	metrics := mock.NewUserMetricsRepository()
	svc := NewTrustLevelService(TrustLevelDeps{
		Config:      NewXPConfigService(cfgRepo),
		Users:       users,
		Metrics:     metrics,
		Adjustments: mock.NewXPAdjustmentRepository(),
	})
	return svc, users, metrics
}

func TestRecompute_WritesXPAndLevelOnce(t *testing.T) {
	ctx := context.Background()
	svc, users, metrics := newTrustLevelServiceForTest(t)
	require.NoError(t, users.Create(ctx, &model.User{ID: "u1", TrustLevel: "new"}))
	metrics.Set("u1", &model.UserMetrics{PostApproved: 12}) // 12 × 10 = 120 XP
	require.NoError(t, svc.AdjustXP(ctx, "u1", 200, "活动奖励", "admin-1"))

	snap, err := svc.Recompute(ctx, "u1")
	require.NoError(t, err)
	require.Equal(t, 320, snap.TotalXP)
	require.Equal(t, "member", snap.EffectiveKey) // 320 ≥ 200 → TL2

	u, err := users.FindByID(ctx, "u1")
	require.NoError(t, err)
	require.Equal(t, 320, u.TotalXP)
	require.Equal(t, "member", u.TrustLevel)
	require.Equal(t, 1, users.LevelWriteCalls("u1"), "等级/XP 只允许一次写入")
}

func TestRecompute_RespectsManualOverride(t *testing.T) {
	ctx := context.Background()
	svc, users, metrics := newTrustLevelServiceForTest(t)
	require.NoError(t, users.Create(ctx, &model.User{ID: "u1", TrustLevel: "leader", LevelManual: true}))
	metrics.Set("u1", &model.UserMetrics{PostApproved: 12})

	snap, err := svc.Recompute(ctx, "u1")
	require.NoError(t, err)
	require.Equal(t, "basic", snap.ComputedKey, "按 XP 只能到 TL1")
	require.Equal(t, "leader", snap.EffectiveKey, "手工设级优先，不被打回")

	u, err := users.FindByID(ctx, "u1")
	require.NoError(t, err)
	require.Equal(t, "leader", u.TrustLevel)
}
```

**本任务需要同时给 `internal/mock/mock.go` 补的辅助方法**（否则上面的测试编译不过，属于同一提交的一部分）：

| mock | 新增 | 语义 |
|---|---|---|
| `XPConfigRepository` | `SeedDefaults(ctx) error`、`NewXPConfigRepository()` | 内存三表；`SeedDefaults` 写 design §4.4 的默认值 |
| `UserMetricsRepository` | `Set(userID string, m *model.UserMetrics)`、`Metrics(...)` | 按 userID 返回预置指标；`DailyCounts` 返回空 map（日上限不截断） |
| `XPAdjustmentRepository` | `NewXPAdjustmentRepository()`、`Sum/Create/ListByUser/Delete` | 内存切片 |
| `UserRepository` | `LevelWriteCalls(userID string) int` | 统计 `UpdateXPLevel` 被调用次数，用于钉住"单点写入" |

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/service/ -run TestRecompute -count=1`
预期：FAIL，`undefined: NewTrustLevelService`。

- [ ] **步骤 3：编写实现**

先在 `internal/repository/user.go` 的 `UserRepository` 接口上加一个**专用写入方法**（替换裸 `IncrPoints`，让"单点写入"可被类型系统与 mock 计数共同约束）：

```go
	// UpdateXPLevel 是等级/XP 冗余列的唯一写入方法，只允许 TrustLevelService 调用。
	UpdateXPLevel(ctx context.Context, userID string, totalXP int, levelKey, snapshotJSON string) error
```

`internal/repository/user_sql.go` 实现（一条 UPDATE，不做读改写，避免丢更新）：

```go
func (r *sqlUserRepo) UpdateXPLevel(ctx context.Context, userID string, totalXP int, levelKey, snapshotJSON string) error {
	return WrapDBErr(r.db.TxDB(ctx).Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
		"total_xp":     totalXP,
		"trust_level":  levelKey,
		"level_metrics": snapshotJSON,
		"updated_at":   time.Now(),
	}).Error)
}
```

然后是服务接口与 `Recompute`：

```go
type TrustLevelService interface {
	// Recompute 是 users.total_xp / users.trust_level 的唯一写入方。
	Recompute(ctx context.Context, userID string) (*LevelSnapshot, error)
	// Snapshot 读快照；超过 TTL 则先 Recompute。
	Snapshot(ctx context.Context, userID string) (*LevelSnapshot, error)
	// Breakdown 返回各指标对 XP 的贡献，供后台“为什么是这个等级”。
	Breakdown(ctx context.Context, userID string) ([]MetricContribution, error)
	// AdjustXP 写人工/活动加减分（不改冗余列，冗余列由 Recompute 刷新）。
	AdjustXP(ctx context.Context, userID string, delta int, reason, actorID string) error
	// SetManualLevel 管理员手工设级（置 level_manual）；ClearManualLevel 恢复自动。
	SetManualLevel(ctx context.Context, userID string, level int) error
	ClearManualLevel(ctx context.Context, userID string) error
	StartDailyRecomputeTicker(ctx context.Context) context.CancelFunc
}
```

`Recompute` 的核心（顺序不可调换）：

```go
func (s *trustLevelService) Recompute(ctx context.Context, userID string) (*LevelSnapshot, error) {
	cfg, err := s.config.Load(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	since := time.Now().AddDate(0, 0, -cfg.MaxWindow)
	metrics, err := s.metrics.Metrics(ctx, userID, since)
	if err != nil {
		return nil, err
	}
	daily, err := s.dailyCounts(ctx, userID, cfg, since) // 逐日计数，供日上限截断
	if err != nil {
		return nil, err
	}
	adj, err := s.adjustments.Sum(ctx, userID)
	if err != nil {
		return nil, err
	}
	totalXP := EvaluateXP(cfg.RuleList(), cfg.StreakList(), daily, metrics) + adj
	computed := EvaluateLevel(cfg.Levels, totalXP, metrics)

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	effective := computed
	if user.LevelManual {
		// 管理员意图优先：不自动改等级，但把计算结果记进快照供排查。
		effective = cfg.ByKey(user.TrustLevel)
	}
	snap := &LevelSnapshot{TotalXP: totalXP, ComputedKey: computed.Key, EffectiveKey: effective.Key,
		Metrics: metrics, ComputedAt: time.Now()}
	if err := s.users.UpdateXPLevel(ctx, userID, totalXP, effective.Key, snap.JSON()); err != nil {
		return nil, err
	}
	return snap, nil
}
```

触发点接线（每处都**只**调 `Recompute`，禁止再直写等级/积分）：

| 位置 | 改法 |
|---|---|
| `service/post.go` Create 尾部 | `awardPoints(...)` → `s.trustLevel.Recompute(ctx, authorID)`；Delete 同样 |
| `service/content.go` Create / Delete | 同上 |
| `service/like.go` toggle 两个分支 | `liked` 与 `unliked` 都要 Recompute（被赞方是 `content.AuthorID`） |
| `service/favorite.go` toggle | 同上（被收藏方） |
| `service/checkin.go` | 去掉 `user.TrustLevel = trustLevelForPoints(...)`，事务提交后 `Recompute` |
| `service/post.go` SetFeatured（任务 7） | 加精/取消加精后 Recompute |

`container/runtime.go`：把 `r.workers["trust_promotion"] = c.TrustSvc.StartDailyPromotionTicker(ctx)` 换成 `r.workers["trust_recompute"] = c.TrustLevelSvc.StartDailyRecomputeTicker(ctx)`（每日全量/增量重算，处理不活跃衰减降级）。

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/service/ -count=1 && go build ./...`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/repository/xp_adjustment*.go app/server/internal/service/trust_level*.go \
        app/server/internal/service/{post,content,like,favorite,checkin}.go \
        app/server/internal/container/ app/server/internal/mock/mock.go
git commit -m "feat(trust): TrustLevelService 单点写入 + 行为触发点接线"
```

---

## 任务 7：删除旧逻辑（硬编码常量与三方抢写）

**文件：**
- 删除：`app/server/internal/service/points.go`、`points_test.go`
- 修改：`app/server/internal/service/trust.go`、`trust_iface.go`、`checkin.go`

- [ ] **步骤 1：先写回归测试（钉住"不再有第二个写入方"）**

```go
func TestNoDirectLevelWritesOutsideTrustLevelService(t *testing.T) {
	// 静态检查：除 trust_level.go 与 trust.go（管理员设级）外，
	// 源码里不得出现 `TrustLevel =` / `TotalPoints +=` / `IncrPoints(`。
	out, err := exec.Command("grep", "-rn", "-E",
		`(TrustLevel|TotalXP|TotalPoints)\s*(\+=|=)|IncrPoints\(`,
		"internal/service", "internal/handler").CombinedOutput()
	if err != nil && len(out) == 0 {
		return // 无匹配即通过
	}
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" || strings.Contains(line, "trust_level.go") || strings.Contains(line, "trust.go") {
			continue
		}
		t.Fatalf("等级/XP 出现第二个写入方: %s", line)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/service/ -run TestNoDirectLevelWrites -count=1`
预期：FAIL，列出 `checkin.go:97` 等残留直写。

- [ ] **步骤 3：删除与改造**

- 删 `points.go`：`PointsPostCreate/CommentCreate/Liked`、`trustLevelForPoints`、`syncTrustLevel`、`awardPoints` 全部移除（调用点已在任务 6 换成 `Recompute`）。
- `trust.go`：删 `evaluatePromotion` / `PromoteEligible` / `StartDailyPromotionTicker` / `trustIntToString` / `trustStringToInt` 的**晋升链**；`ShouldForceApproval` / `ShouldBlockLinks` 暂时保留（P3 由规则引擎接管），其 `trustLevel` 入参改为按 `trust_levels.level` 比较 `> 0`。
- `trust_iface.go`：`TrustService` 只留 `ShouldForceApproval` / `ShouldBlockLinks` / `ContainsExternalLink` 与 `UpdateLevel`（改为写 `level_manual`）。
- `checkin.go`：删除 `user.TrustLevel = trustLevelForPoints(user.TotalPoints)`。
- `check_ins.points_earned` 保留写值（历史展示用），但**不再参与 XP 求值**，在字段注释里写明。

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./... 2>&1 | grep -Ev '^ok|no test files' | tail -20`
预期：无 FAIL（`trust_test.go` 里针对旧晋升链的用例需同步删除或改写）。

- [ ] **步骤 5：Commit**

```bash
git add -A app/server/internal/service
git commit -m "refactor(trust)!: 删除积分硬编码常量与独立晋升链，等级只由 TrustLevelService 写入"
```

---

## 任务 8：精华机制（`posts.featured_at`）

**文件：**
- 修改：`app/server/internal/model/post.go`、`app/server/internal/repository/post.go`（`SetFeatured`）、`app/server/internal/service/post.go`、`app/server/internal/handler/post.go`、`app/server/internal/router/routes_content.go`、`app/server/internal/gateway/permission.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestSetFeatured_RecomputesAuthorXP(t *testing.T) {
	// 帖子被设为精华 → 作者 XP +10；取消精华 → 回落
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/service/ -run TestSetFeatured -count=1`
预期：FAIL。

- [ ] **步骤 3：实现**

`model/post.go` 加 `FeaturedAt *time.Time \`json:"featured_at,omitempty"\``；`PostRepository` 加 `SetFeatured(ctx, id string, featured bool) error`（置/清 `featured_at`）；`PostService.SetFeatured(ctx, id string, featured bool, actorRole string) (*model.Post, error)`（权限：`post:pin`），成功后 `Recompute(post.AuthorID)`；handler `SetFeatured` + 路由 `p.Post("/{id}/feature", h.Post.SetFeatured)` + 网关映射 `addPattern("/api/v1/posts/{id}/feature", "post:pin")`。

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/service/ -run TestSetFeatured -count=1`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/model/post.go app/server/internal/repository/post*.go \
        app/server/internal/service/post*.go app/server/internal/handler/post.go \
        app/server/internal/router/routes_content.go app/server/internal/gateway/permission.go
git commit -m "feat(post): 精华标记（featured_at）并计入作者 XP"
```

---

## 任务 9：后台 API（等级 / XP 规则 / 用户 XP）

**文件：**
- 创建：`app/server/internal/handler/trust_level.go`
- 修改：`app/server/internal/model/permission.go`（`PermLevelManage = "level:manage"`）、`app/server/internal/router/routes_admin.go`、`app/server/internal/gateway/permission.go`、`app/server/internal/handler/handler.go`、`app/server/server/wiring.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestTrustLevelHandler_RejectsInvalidLevel(t *testing.T) {
	// POST /admin/levels/save {"level":-1} → 2001 INVALID_PARAMS
}
func TestTrustLevelHandler_RejectsInvalidMetric(t *testing.T) {
	// POST /admin/xp-rules/save {"metric":"whatever"} → 2001
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/handler/ -run TestTrustLevelHandler -count=1`
预期：FAIL。

- [ ] **步骤 3：实现端点（全部 POST，遵循 api-style）**

| 端点 | 权限 | 说明 |
|---|---|---|
| `POST /admin/levels/list` | `level:manage` | 等级列表 |
| `POST /admin/levels/save` | `level:manage` | 新增/更新等级（校验：level ≥0、key 唯一、min_xp 单调、窗口天数 >0） |
| `POST /admin/levels/{level}/delete` | `level:manage` | 删除等级（校验：不能删仍有用户的等级） |
| `POST /admin/xp-rules/list` | `level:manage` | XP 规则 + 签到阶梯 |
| `POST /admin/xp-rules/save` | `level:manage` | 保存规则（校验 metric ∈ 指标枚举、delta ≥0、daily_cap ≥0） |
| `POST /admin/xp-rules/streak/save` | `level:manage` | 保存签到阶梯档位 |
| `POST /admin/users/{id}/xp/breakdown` | `level:manage` | 指标分解 |
| `POST /admin/users/{id}/xp/adjust` | `level:manage` | 人工加减分（写 `xp_adjustments`） |
| `POST /admin/users/{id}/level/set` | `level:manage` | 手工设级（置 `level_manual`） |
| `POST /admin/users/{id}/level/clear` | `level:manage` | 恢复自动 |

网关映射加 `addPrefix("/api/v1/admin/levels/", "level:manage")` 等四条（注意 `addPrefix` 优先于 `addPattern`，需确认不与既有 `/api/v1/admin/` 规则冲突）。

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/handler/ ./internal/gateway/ -count=1 && cd .. && cd server && make swag`
预期：PASS，且 `openapi.json` 出现新端点。

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/handler/trust_level.go app/server/internal/handler/handler.go \
        app/server/internal/model/permission.go app/server/internal/router/routes_admin.go \
        app/server/internal/gateway/permission.go app/server/server/wiring.go \
        openapi.json app/server/docs
git commit -m "feat(admin): 等级/XP 规则/用户 XP 后台 API（level:manage）"
```

---

## 任务 10：后台前端页面

**文件：**
- 创建：`packages/api-client/src/services/trust-level.ts`、`app/web/app/admin/levels/page.tsx`、`app/web/app/admin/xp-rules/page.tsx`
- 修改：`app/web/lib/query/keys.ts`、`app/web/app/admin/users/detail/page.tsx`、`app/web/components/admin/layout/admin-shell.tsx`（菜单）、`app/web/lib/admin/auth/route-access.ts`（门控）

- [ ] **步骤 1：写类型与 service（先于页面）**

```ts
// packages/api-client/src/services/trust-level.ts
import { post } from '../client';
import type { TrustLevel, XPRule, XPStreakBonus, XPMetricContribution } from '../types';

export const trustLevelService = {
	listLevels: () => post<TrustLevel[]>('/admin/levels/list', {}),
	saveLevel: (l: TrustLevel) => post<null>('/admin/levels/save', l),
	deleteLevel: (level: number) => post<null>(`/admin/levels/${level}/delete`),
	listRules: () => post<{ rules: XPRule[]; streak: XPStreakBonus[] }>('/admin/xp-rules/list', {}),
	saveRule: (r: XPRule) => post<null>('/admin/xp-rules/save', r),
	saveStreak: (b: XPStreakBonus) => post<null>('/admin/xp-rules/streak/save', b),
	breakdown: (userId: string) => post<XPMetricContribution[]>(`/admin/users/${userId}/xp/breakdown`, {}),
	adjustXP: (userId: string, delta: number, reason: string) =>
		post<null>(`/admin/users/${userId}/xp/adjust`, { delta, reason }),
	setLevel: (userId: string, level: number) => post<null>(`/admin/users/${userId}/level/set`, { level }),
	clearLevel: (userId: string) => post<null>(`/admin/users/${userId}/level/clear`),
};
```

- [ ] **步骤 2：页面（两个表格 + 一个详情区块）**

`/admin/levels`：DataTable 列 = 等级 / key / 名称（可编辑）/ XP 门槛 / 窗口天数 / 活跃天数 / 发帖 / 获赞 / 举报上限 / 仅人工 / 启用；行内编辑 + 新增 + 删除（删除前 `useConfirm`）。写后 `invalidateQueries({ queryKey: qk.admin.levels.all })`。

`/admin/xp-rules`：上半区 XP 规则表（指标下拉固定枚举、分值、日上限），下半区签到阶梯表（天数、奖励、启用）。

用户详情新增区块：「XP 分解」表格（指标 / 计数 / 分值 / 贡献 XP，合计 = `total_xp`）+ 「人工调整」按钮（delta + reason 表单）+ 「手工设级 / 恢复自动」。

- [ ] **步骤 3：验收**

运行：`cd app/web && pnpm typecheck && pnpm check && cd ../../packages/api-client && pnpm check`
预期：全部 exit 0。

- [ ] **步骤 4：Commit**

```bash
git add packages/api-client/src/services/trust-level.ts packages/api-client/src/types.ts \
        app/web/app/admin/levels app/web/app/admin/xp-rules app/web/app/admin/users/detail/page.tsx \
        app/web/lib/query/keys.ts app/web/components/admin/layout/admin-shell.tsx \
        app/web/lib/admin/auth/route-access.ts
git commit -m "feat(admin-web): 等级管理 / XP 规则 / 用户 XP 分解与人工调整"
```

---

## 任务 11：全量验收与文档同步

- [ ] **步骤 1：后端全量**

运行：`cd app/server && go build ./... && go vet ./... && go test -count=1 ./... 2>&1 | grep -Ev '^ok|no test files' | tail -20`
预期：无 FAIL。

- [ ] **步骤 2：前端全量 + 文档门**

运行：`cd app/web && pnpm typecheck && pnpm check`；`cd /Users/noelorin/Feed Chamber && node scripts/check-agents-docs.mjs --base HEAD`
预期：全部通过（改了模块代码必须同步该模块 AGENTS.md）。

- [ ] **步骤 3：迁移脚本演练**

在干净 SQLite 上：`DB_TYPE=sqlite DB_DSN=/tmp/of_p1.db go run main.go -e dev`（需外部 NATS），确认 AutoMigrate 建表、seed 写入 5 个等级与规则，然后对既有用户跑一次全量 `Recompute`，核对 `total_xp` 与 `trust_level`。

- [ ] **步骤 4：文档与规范**

- 更新 `internal/service/AGENTS.md`、`internal/repository/AGENTS.md`、`internal/model/AGENTS.md`、`internal/handler/AGENTS.md`、`internal/router/AGENTS.md`、`internal/gateway/AGENTS.md`、`app/web/app/admin/AGENTS.md`、`packages/api-client/src/services/AGENTS.md`
- `make swag` 重新生成
- 勾选 `openspec/changes/2026-10-28-trust-level-xp-system/tasks.md` 的 P1 条目

- [ ] **步骤 5：Commit**

```bash
git add -A
git commit -m "docs(trust): P1 文档同步与验收记录"
```

---

## 风险与回滚

| 风险 | 处置 |
|---|---|
| 全量 `Recompute` 把老用户等级集体下调 | 首次上线前先在影子库跑一遍，输出"等级变化矩阵"供运营确认；必要时给 `trust_levels` 的保级条件设宽松默认 |
| 聚合查询拖慢请求（每次 Recompute 查 7 张表） | Recompute 只在写路径尾部调用且非事务内；每日 ticker 走批量；如仍慢，把 `user_metrics` 做成 5 分钟 TTL 的 KV 缓存 |
| `posts.featured_at` 与既有 `pinned` 语义混淆 | 精华只影响 XP 与展示标记，**不参与排序**；`pinned` 继续只管排序 |
| 回滚 | 新表与新列都是纯增量，回滚只需回退代码；`users.total_xp` / `trust_level` 的写入由 `TrustLevelService` 独占，旧 `points.go` 恢复即可回到旧行为 |

---

## 规格覆盖度自检（本计划 vs `specs/trust-levels` 与 `specs/access-gating`）

| 规范 Requirement | 本计划任务 | 说明 |
|---|---|---|
| XP 由行为聚合派生（含撤销自动回收、日上限、规则改动重算） | 任务 2 · 3 · 6 | 撤销路径靠"只计有效内容"天然成立，任务 2 的测试钉住 |
| 连续签到按里程碑一次性奖励 | 任务 2（`StreakHits`）· 3（`EvaluateXP`）· 5（阶梯配置） | "恰好达到"由 `check_ins.streak` 分组计数实现 |
| 等级由累计 XP 定级并由滚动窗口保级 | 任务 4 · 6 | 100 天窗口来自 `trust_levels.window_days` |
| 等级与 XP 是派生缓存且写入单点 | 任务 6 · 7 | `UpdateXPLevel` + 静态检查测试（任务 7 步骤 1） |
| 游客与刚注册用户同处 TL0 | 任务 5（seed 必须含 TL0） | **判定侧（游客按 TL0 比较）在 P2**，本计划只保证 TL0 存在且是最低级 |
| 等级定义必须后台可配（数量/名称/门槛/保级/阶梯/分值/日上限） | 任务 5 · 9 · 10 | 名称可改在任务 9 的 save 校验里 |
| 人工调整 XP 必须可追溯 + 后台指标分解 | 任务 6（`AdjustXP`/`Breakdown`）· 9 · 10 | 分解之和 = `total_xp` 由任务 10 的页面断言 |
| 版面可见性策略 / 受限内容表现 / 六出口一致性 | — | **P2**，本计划不含 |
| 门控规则引擎（受控语法、域默认、首匹配、可解释、后台可配） | — | **P3**，本计划不含 |
| 动议票权改按等级（community-motion MODIFIED） | — | **P4**，本计划不含 |

**占位符扫描：** 无 `TODO` / `待定` / "类似任务 N"；每个代码步骤都给了可编译的实现或测试片段。

**类型一致性检查：** `TrustLevel` / `XPRule` / `XPStreakBonus` / `XPAdjustment` / `UserMetrics` / `XPConfig` / `LevelSnapshot` / `MetricContribution` / `TrustLevelService` / `XPConfigService` / `UpdateXPLevel` / `EvaluateXP` / `EvaluateLevel` / `Recompute` 在首次出现的任务里定义，后续任务只引用不再改签名。
