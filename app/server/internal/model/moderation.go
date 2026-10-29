package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// ModerationStatus 审核状态
type ModerationStatus string

const (
	ModerationPending  ModerationStatus = "pending"   // 待审核
	ModerationApproved ModerationStatus = "approved"  // 通过
	ModerationRejected ModerationStatus = "rejected"  // 拒绝
	ModerationAutoPass ModerationStatus = "auto_pass" // 自动通过(白名单/高信用用户)
)

// ModerationAction 审核动作
type ModerationAction string

const (
	ModerationActionAllow   ModerationAction = "allow"   // 放行
	ModerationActionBlock   ModerationAction = "block"   // 拦截(不可见)
	ModerationActionReplace ModerationAction = "replace" // 替换敏感词
	ModerationActionFlag    ModerationAction = "flag"    // 标记人工复核
)

// ModerationScene 审核场景
type ModerationScene string

const (
	ModerationScenePost    ModerationScene = "post"    // 帖子
	ModerationSceneComment ModerationScene = "comment" // 评论/回复
	ModerationSceneEdit    ModerationScene = "edit"    // 编辑修改
)

// 文章审核模式（平台级四选一，存于系统设置表的 moderation_mode）。
//
//	off      不审核：发布即公开，不跑规则、不写审核记录
//	async    发布后异步审核：先公开，随后后台跑规则，命中转待审交人工判断
//	external 外部自动化审核：发布前同步调用外部服务，不可用则回退 async
//	manual   全部人工：发布即待审，必须人工处理
const (
	ModerationModeOff      = "off"
	ModerationModeAsync    = "async"
	ModerationModeExternal = "external"
	ModerationModeManual   = "manual"
)

// DefaultModerationMode 是未配置时的模式：发布后异步审核。
const DefaultModerationMode = ModerationModeAsync

// ParseModerationMode 规范化审核模式取值。未知值回落默认值——后台写错一个字
// 不应该把审核门静默关掉（那等于全站免审），也不应该把发帖卡成全员待审。
func ParseModerationMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case ModerationModeOff:
		return ModerationModeOff
	case ModerationModeAsync:
		return ModerationModeAsync
	case ModerationModeExternal:
		return ModerationModeExternal
	case ModerationModeManual:
		return ModerationModeManual
	default:
		return DefaultModerationMode
	}
}

// ValidModerationMode 报告取值是否为可写入的合法模式（写设置时的校验用，
// 与 ParseModerationMode 的「回落」不同：它拒绝未知值）。
func ValidModerationMode(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case ModerationModeOff, ModerationModeAsync, ModerationModeExternal, ModerationModeManual:
		return true
	default:
		return false
	}
}

// ContentModeration 内容审核记录
type ContentModeration struct {
	ID           string           `gorm:"primaryKey" json:"id"`
	TargetType   string           `gorm:"size:32;not null;index" json:"target_type"` // post / content
	TargetID     string           `gorm:"not null;index" json:"target_id"`
	Scene        ModerationScene  `gorm:"size:32;not null" json:"scene"`
	Status       ModerationStatus `gorm:"size:16;not null;default:pending;index" json:"status"`
	Action       ModerationAction `gorm:"size:16" json:"action,omitempty"`
	Strategy     string           `gorm:"size:64" json:"strategy,omitempty"` // keyword / ai / manual / whitelist
	Reason       string           `gorm:"type:text" json:"reason,omitempty"`
	OriginalBody string           `gorm:"type:text" json:"original_body,omitempty"`
	FilteredBody string           `gorm:"type:text" json:"filtered_body,omitempty"`
	MatchedRules string           `gorm:"type:text" json:"matched_rules,omitempty"` // JSON array
	ReviewerID   string           `gorm:"index" json:"reviewer_id,omitempty"`
	ReviewedAt   *time.Time       `json:"reviewed_at,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	DeletedAt    gorm.DeletedAt   `gorm:"index" json:"deleted_at,omitempty" swaggertype:"string"`
}

func (ContentModeration) TableName() string { return "content_moderations" }

// ModerationRule 审核规则配置
type ModerationRule struct {
	ID          string           `gorm:"primaryKey" json:"id"`
	Name        string           `gorm:"uniqueIndex;size:128;not null" json:"name"`
	Scene       ModerationScene  `gorm:"size:32;not null;index" json:"scene"`
	Type        string           `gorm:"size:32;not null" json:"type"`      // keyword / regex / ai
	Pattern     string           `gorm:"type:text;not null" json:"pattern"` // 关键词/正则/AI模型名
	Action      ModerationAction `gorm:"size:16;not null" json:"action"`
	Priority    int              `gorm:"default:0" json:"priority"` // 优先级，越大越先匹配
	Enabled     bool             `gorm:"default:true" json:"enabled"`
	Description string           `gorm:"type:text" json:"description,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	DeletedAt   gorm.DeletedAt   `gorm:"index" json:"deleted_at,omitempty" swaggertype:"string"`
}

func (ModerationRule) TableName() string { return "moderation_rules" }

// ModerationConfig 审核全局配置
type ModerationConfig struct {
	Key   string `gorm:"primaryKey;size:64" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}

func (ModerationConfig) TableName() string { return "moderation_configs" }

// 审核配置键名
const (
	ModerationConfigEnabled           = "moderation_enabled"             // 总开关
	ModerationConfigPostEnabled       = "moderation_post_enabled"        // 帖子审核开关
	ModerationConfigCommentEnabled    = "moderation_comment_enabled"     // 评论审核开关
	ModerationConfigEditEnabled       = "moderation_edit_enabled"        // 编辑审核开关
	ModerationConfigDefaultAction     = "moderation_default_action"      // 默认动作
	ModerationConfigWhitelistRoles    = "moderation_whitelist_roles"     // 白名单角色(JSON数组)
	ModerationConfigAutoPassThreshold = "moderation_auto_pass_threshold" // 自动通过信用分阈值
	ModerationConfigAIProvider        = "moderation_ai_provider"         // AI提供商: openai / local / none
	ModerationConfigAIEndpoint        = "moderation_ai_endpoint"         // AI服务地址
	ModerationConfigAIAPIKey          = "moderation_ai_api_key"          // AI API Key
)

// DefaultModerationConfigs 默认审核配置
var DefaultModerationConfigs = map[string]string{
	ModerationConfigEnabled:           "false",
	ModerationConfigPostEnabled:       "false",
	ModerationConfigCommentEnabled:    "false",
	ModerationConfigEditEnabled:       "false",
	ModerationConfigDefaultAction:     string(ModerationActionFlag),
	ModerationConfigWhitelistRoles:    `["admin","moderator"]`,
	ModerationConfigAutoPassThreshold: "100",
	ModerationConfigAIProvider:        "none",
	ModerationConfigAIEndpoint:        "",
	ModerationConfigAIAPIKey:          "",
}
