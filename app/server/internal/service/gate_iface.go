package service

import (
	"context"

	"feedchamber/internal/model"
)

// GateDecision 是一次门控求值的结果，必须可解释：要么命中某条规则，要么走域默认。
type GateDecision struct {
	Domain    string         `json:"domain"`
	Action    string         `json:"action"`
	RuleID    string         `json:"rule_id,omitempty"`
	Priority  int            `json:"priority,omitempty"`
	Note      string         `json:"note,omitempty"`
	IsDefault bool           `json:"is_default"`
	Params    map[string]any `json:"action_params,omitempty"`
}

// GateInput 是求值所需的访问者与内容属性（不含自由文本）。
type GateInput struct {
	UserID               string
	Role                 string
	Level                int
	XP                   int
	IsAnonymous          bool
	EmailVerified        bool
	Banned               bool
	Exiled               bool
	IsAuthor             bool
	AuthorLevel          int
	SpaceID              string
	Tags                 []string
	ContainsExternalLink bool
}

// GateSimulateInput 是后台「以用户试算」的输入。
type GateSimulateInput struct {
	UserID               string   `json:"user_id"`
	Domain               string   `json:"domain"`
	SpaceID              string   `json:"space_id"`
	Tags                 []string `json:"tags"`
	IsAuthor             bool     `json:"is_author"`
	AuthorLevel          int      `json:"author_level"`
	Content              string   `json:"content"`
	ContainsExternalLink *bool    `json:"contains_external_link"`
}

// GateService 是门控规则引擎：固定谓词集 + 固定动作集 + 首匹配 + 域默认。
type GateService interface {
	// Evaluate 按优先级降序取首个命中；无命中执行该能力域的显式默认动作。
	Evaluate(ctx context.Context, domain string, in GateInput) (GateDecision, error)
	// EnforceRateLimit 执行 rate_limit 动作（键含用户 / 等级维度）。
	EnforceRateLimit(ctx context.Context, decision GateDecision, in GateInput) error

	ListRules(ctx context.Context) ([]model.GateRule, error)
	SaveRule(ctx context.Context, rule *model.GateRule, actorID string) error
	DeleteRule(ctx context.Context, id string) error
	ListDomainDefaults(ctx context.Context) ([]model.GateDomainDefault, error)
	SaveDomainDefault(ctx context.Context, domain, action string) error
	Simulate(ctx context.Context, in GateSimulateInput) (GateDecision, error)

	// StartSyncSubscriber 订阅 gate:sync，多实例在同步延迟内一致。
	StartSyncSubscriber(ctx context.Context) error
}
