package model

import (
	"encoding/json"
	"time"

	"feedchamber/internal/pkg"
)

// 能力域：每个域有且只有一个显式默认动作（无规则命中时执行）。
const (
	GateDomainContentView   = "content_view"
	GateDomainPostCreate    = "post_create"
	GateDomainCommentCreate = "comment_create"
	GateDomainExternalLink  = "external_link"
	GateDomainRateLimit     = "rate_limit"
)

// GateDomainLabels 是能力域的中文名（后台展示）。
var GateDomainLabels = map[string]string{
	GateDomainContentView:   "读内容",
	GateDomainPostCreate:    "发帖",
	GateDomainCommentCreate: "评论",
	GateDomainExternalLink:  "外链",
	GateDomainRateLimit:     "限流",
}

// 动作集：固定枚举，互不重叠。内容可见性 MUST NOT 通过动作表达。
const (
	GateActionAllow         = "allow"
	GateActionDeny          = "deny"
	GateActionRequireReview = "require_review"
	GateActionRateLimit     = "rate_limit"
)

var GateActions = []string{GateActionAllow, GateActionDeny, GateActionRequireReview, GateActionRateLimit}

// 谓词字段：固定枚举，可在后台表单化配置，不提供自由表达式。
const (
	GateFieldLevel                = "level"
	GateFieldXP                   = "xp"
	GateFieldSpaceID              = "space_id"
	GateFieldTag                  = "tag"
	GateFieldAuthorLevel          = "author_level"
	GateFieldIsAuthor             = "is_author"
	GateFieldIsAnonymous          = "is_anonymous"
	GateFieldEmailVerified        = "email_verified"
	GateFieldBanned               = "banned"
	GateFieldExiled               = "exiled"
	GateFieldContainsExternalLink = "contains_external_link"
	GateFieldWindowMetric         = "window_metric"
)

// GateFieldLabels / GateFieldOps 供后台渲染条件表单与校验。
var GateFieldLabels = map[string]string{
	GateFieldLevel:                "用户等级",
	GateFieldXP:                   "用户 XP",
	GateFieldSpaceID:              "版面 ID",
	GateFieldTag:                  "标签",
	GateFieldAuthorLevel:          "作者等级",
	GateFieldIsAuthor:             "是否作者本人",
	GateFieldIsAnonymous:          "是否游客",
	GateFieldEmailVerified:        "邮箱已验证",
	GateFieldBanned:               "是否被封禁",
	GateFieldExiled:               "是否在暗区",
	GateFieldContainsExternalLink: "含外链",
	GateFieldWindowMetric:         "窗口指标",
}

var GateFieldOps = map[string][]string{
	GateFieldLevel:                {"eq", "ne", "gt", "gte", "lt", "lte"},
	GateFieldXP:                   {"eq", "ne", "gt", "gte", "lt", "lte"},
	GateFieldSpaceID:              {"eq", "ne", "in"},
	GateFieldTag:                  {"eq", "in"},
	GateFieldAuthorLevel:          {"eq", "ne", "gt", "gte", "lt", "lte"},
	GateFieldIsAuthor:             {"eq"},
	GateFieldIsAnonymous:          {"eq"},
	GateFieldEmailVerified:        {"eq"},
	GateFieldBanned:               {"eq"},
	GateFieldExiled:               {"eq"},
	GateFieldContainsExternalLink: {"eq"},
	GateFieldWindowMetric:         {"eq", "ne", "gt", "gte", "lt", "lte"},
}

// GateWindowMetrics 是 window_metric 谓词允许引用的指标。
var GateWindowMetrics = map[string]string{
	"active_days":    "活跃天数",
	"post_count":     "发帖数",
	"likes_received": "获赞数",
	"report_upheld":  "成立举报数",
}

// GateCondition 是一条谓词。Value 的形态由 Field/Op 决定（数字 / 布尔 / 字符串 / 数组）。
type GateCondition struct {
	Field      string `json:"field"`
	Op         string `json:"op"`
	Value      any    `json:"value"`
	WindowDays int    `json:"window_days,omitempty"`
}

// GateRule 是门控规则：优先级 + 能力域 + 条件 + 动作。
type GateRule struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	Priority     int       `gorm:"default:100;index" json:"priority"`
	Enabled      bool      `gorm:"default:true;index" json:"enabled"`
	Domain       string    `gorm:"size:32;index" json:"domain"`
	Conditions   string    `gorm:"type:text" json:"conditions"` // JSON []GateCondition
	Action       string    `gorm:"size:32" json:"action"`
	ActionParams string    `gorm:"type:text" json:"action_params"` // JSON object
	Note         string    `gorm:"size:255" json:"note"`
	UpdatedBy    string    `gorm:"size:64" json:"updated_by"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (GateRule) TableName() string { return "gate_rules" }

// ParsedConditions 解码条件；坏数据返回空集（规则因此永不命中，而不是 panic）。
func (r GateRule) ParsedConditions() []GateCondition {
	var out []GateCondition
	if r.Conditions == "" {
		return out
	}
	_ = json.Unmarshal([]byte(r.Conditions), &out)
	return out
}

// ParsedActionParams 解码动作参数。
func (r GateRule) ParsedActionParams() map[string]any {
	out := map[string]any{}
	if r.ActionParams == "" {
		return out
	}
	_ = json.Unmarshal([]byte(r.ActionParams), &out)
	return out
}

// GateDomainDefault 是能力域的显式默认动作。
type GateDomainDefault struct {
	Domain        string    `gorm:"primaryKey;size:32" json:"domain"`
	DefaultAction string    `gorm:"size:32" json:"default_action"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (GateDomainDefault) TableName() string { return "gate_domain_defaults" }

func NewGateRuleID() string { return pkg.NewID() }

// DefaultGateDomainDefaults 是首启 seed 的域默认动作（design §4.4）。
var DefaultGateDomainDefaults = []GateDomainDefault{
	{Domain: GateDomainContentView, DefaultAction: GateActionAllow},
	{Domain: GateDomainPostCreate, DefaultAction: GateActionAllow},
	{Domain: GateDomainCommentCreate, DefaultAction: GateActionAllow},
	{Domain: GateDomainExternalLink, DefaultAction: GateActionDeny},
	{Domain: GateDomainRateLimit, DefaultAction: GateActionAllow},
}

// DefaultGateRules 只 seed「外链」一条（不 seed 发帖强制待审）。
var DefaultGateRules = []GateRule{
	{
		ID:       "gate-external-link-tl0",
		Priority: 100,
		Enabled:  true,
		Domain:   GateDomainExternalLink,
		Conditions: `[{"field":"level","op":"eq","value":0},` +
			`{"field":"contains_external_link","op":"eq","value":true}]`,
		Action: GateActionDeny,
		Note:   "TL0 禁止外链（沿用旧 trust_tl0_block_links 的保守默认）",
	},
	{
		ID:         "gate-external-link-level1",
		Priority:   50,
		Enabled:    true,
		Domain:     GateDomainExternalLink,
		Conditions: `[{"field":"level","op":"gte","value":1}]`,
		Action:     GateActionAllow,
		Note:       "等级 ≥1 允许外链（域默认是 deny，需要显式放行）",
	},
}
