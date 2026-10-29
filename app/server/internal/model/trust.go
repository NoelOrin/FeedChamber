package model

import (
	"encoding/json"
	"time"

	"feedchamber/internal/pkg"
)

// 指标注册表（XP 规则与保级条件共用同一批固定枚举）。
//
// 指标必须是可用 SQL 聚合的固定枚举：新增指标要同时加这里的常量、仓库层的
// 聚合 SQL 与后台 XP 规则页的标签，否则规则存得进去但算不出数。
const (
	MetricPostApproved    = "post_approved_count"     // 已通过审核的帖子数
	MetricCommentApproved = "comment_approved_count"  // 已通过审核的评论数
	MetricLikesReceived   = "likes_received_count"    // 收到的赞数
	MetricFavoritesRecv   = "favorite_received_count" // 被收藏数
	MetricFeaturedPost    = "featured_post_count"     // 精华帖数
	MetricCheckin         = "checkin_count"           // 签到天数
	MetricActiveDays      = "active_day_count"        // 活跃天数（保级用）
	MetricReportUpheld    = "report_upheld_count"     // 已受理成立的被举报数（保级用）
)

// XPMetricLabels 是指标的中文标签，仅用于后台展示，不落库。
var XPMetricLabels = map[string]string{
	MetricPostApproved:    "发帖（已通过）",
	MetricCommentApproved: "评论（已通过）",
	MetricLikesReceived:   "被点赞",
	MetricFavoritesRecv:   "被收藏",
	MetricFeaturedPost:    "精华帖",
	MetricCheckin:         "签到",
	MetricActiveDays:      "活跃天数（仅保级）",
	MetricReportUpheld:    "举报成立（仅保级）",
}

// XP 规则：指标 → 经验值。DailyCap 为 0 表示不做自然日截断。
type XPRule struct {
	Metric    string    `gorm:"primaryKey;size:64" json:"metric"`
	Delta     int       `gorm:"default:0" json:"delta"`
	DailyCap  int       `gorm:"default:0" json:"daily_cap"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`

	Label string `gorm:"-" json:"label,omitempty"`
}

func (XPRule) TableName() string { return "xp_rules" }

// XPStreakBonus 连续签到里程碑阶梯：连续天数恰好达到 Days 时一次性给 Bonus。
type XPStreakBonus struct {
	Days      int       `gorm:"primaryKey;autoIncrement:false" json:"days"`
	Bonus     int       `gorm:"default:0" json:"bonus"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (XPStreakBonus) TableName() string { return "xp_streak_bonuses" }

// TrustLevel 等级定义。数量、名称、门槛与保级条件全部可配。
//
// WindowDays = 0 表示该等级没有保级条件（TL0/TL1）。MaxReports 为 nil 表示
// 不限制，0 表示「一次成立举报都不允许」——两者必须区分，否则无法表达 TL3
// 的「≤3」与「不允许」。
type TrustLevel struct {
	Level         int       `gorm:"primaryKey;autoIncrement:false" json:"level"`
	Key           string    `gorm:"size:32" json:"key"`
	Name          string    `gorm:"size:64" json:"name"`
	MinXP         int       `gorm:"default:0" json:"min_xp"`
	WindowDays    int       `gorm:"default:0" json:"window_days"`
	MinActiveDays int       `gorm:"default:0" json:"min_active_days"`
	MinPosts      int       `gorm:"default:0" json:"min_posts"`
	MinLikes      int       `gorm:"default:0" json:"min_likes"`
	MaxReports    *int      `json:"max_reports"`
	ManualOnly    bool      `gorm:"default:false" json:"manual_only"`
	Enabled       bool      `gorm:"default:true" json:"enabled"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (TrustLevel) TableName() string { return "trust_levels" }

// XPAdjustment 人工/活动加减分记录（唯一有账本的 XP 来源）。
type XPAdjustment struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    string    `gorm:"index;not null" json:"user_id"`
	Delta     int       `json:"delta"`
	Reason    string    `gorm:"size:500" json:"reason"`
	ActorID   string    `gorm:"size:64" json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (XPAdjustment) TableName() string { return "xp_adjustments" }

// LevelMetrics 是保级条件依赖的窗口指标快照，序列化后存 users.level_metrics。
type LevelMetrics struct {
	ActiveDays    int `json:"active_days"`
	Posts         int `json:"posts"`
	Likes         int `json:"likes"`
	UpheldReports int `json:"upheld_reports"`
}

// DefaultXPRules 是首启 seed 的 XP 分值（design §4.4 已定稿）。
var DefaultXPRules = []XPRule{
	{Metric: MetricPostApproved, Delta: 10, DailyCap: 25, Enabled: true},
	{Metric: MetricCommentApproved, Delta: 3, DailyCap: 20, Enabled: true},
	{Metric: MetricLikesReceived, Delta: 1, DailyCap: 30, Enabled: true},
	{Metric: MetricFavoritesRecv, Delta: 2, Enabled: true},
	{Metric: MetricFeaturedPost, Delta: 10, Enabled: true},
	{Metric: MetricCheckin, Delta: 2, Enabled: true},
}

// DefaultXPStreakBonuses 连续签到阶梯：恰好达到计一次。
var DefaultXPStreakBonuses = []XPStreakBonus{
	{Days: 3, Bonus: 3, Enabled: true},
	{Days: 7, Bonus: 10, Enabled: true},
	{Days: 15, Bonus: 20, Enabled: true},
	{Days: 30, Bonus: 40, Enabled: true},
	{Days: 60, Bonus: 80, Enabled: true},
	{Days: 100, Bonus: 150, Enabled: true},
}

func intPtr(v int) *int { return &v }

// DefaultTrustLevels 是首启 seed 的等级定义（design §4.4 已定稿）。
var DefaultTrustLevels = []TrustLevel{
	{Level: 0, Key: "new", Name: "新手上路", MinXP: 0, Enabled: true},
	{Level: 1, Key: "basic", Name: "基础", MinXP: 50, WindowDays: 100, Enabled: true},
	{Level: 2, Key: "member", Name: "成员", MinXP: 200, WindowDays: 100, MinActiveDays: 10, MaxReports: intPtr(5), Enabled: true},
	{Level: 3, Key: "regular", Name: "常客", MinXP: 500, WindowDays: 100, MinActiveDays: 50, MinPosts: 10, MinLikes: 20, MaxReports: intPtr(3), Enabled: true},
	{Level: 4, Key: "leader", Name: "意见领袖", MinXP: 1000, ManualOnly: true, Enabled: true},
}

// NewXPAdjustment 构造一条人工加分记录。
func NewXPAdjustment(userID, actorID string, delta int, reason string) *XPAdjustment {
	return &XPAdjustment{
		ID:        pkg.NewID(),
		UserID:    userID,
		ActorID:   actorID,
		Delta:     delta,
		Reason:    reason,
		CreatedAt: time.Now(),
	}
}

// MetricsJSON 把窗口指标编码成 users.level_metrics 的快照。
func (m LevelMetrics) MetricsJSON() string {
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}
