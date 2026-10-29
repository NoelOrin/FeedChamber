package model

import "time"

// 片段类型。
const (
	SnippetKindCSS  = "css"
	SnippetKindJS   = "js"
	SnippetKindHTML = "html"
)

// 片段注入位置。
const (
	SnippetPositionHead    = "head"
	SnippetPositionBodyEnd = "body_end"
)

// Snippet 是一段可独立启停的自定义代码片段。
//
// 之所以不用 settings 的 KV 存整包 JSON：那会同时失去三样能力——逐段启停
// （Enabled）、逐段审计（UpdatedBy/UpdatedAt）、以及「当前生效的 JS 有哪些」
// 这类按键索引的查询。而这三样正是「允许任意 JS」场景的硬需求。
type Snippet struct {
	ID   string `gorm:"primaryKey;size:36" json:"id"`
	Kind string `gorm:"size:16;not null;index" json:"kind"`
	// Position 只对 css / js 有意义；html 固定按 head 处理（见 EffectivePosition）。
	Position string `gorm:"size:16;not null" json:"position"`
	Content  string `gorm:"type:text;not null" json:"content"`
	// Enabled 是「是否对外生效」的唯一判据。机器凭证（MCP）写入 JS/HTML 时
	// 强制为 false，需人工在后台启用（见 site-customization 规范）。
	Enabled   bool      `gorm:"not null;default:false;index" json:"enabled"`
	Sort      int       `gorm:"not null;default:0" json:"sort"`
	UpdatedBy string    `gorm:"size:36" json:"updated_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Snippet) TableName() string { return "snippets" }

// EffectivePosition 返回实际注入位置。html 片段一律落 body_end：
// 放在 head 里的裸 HTML 会在 <body> 之前被解析，行为不可预期。
func (s Snippet) EffectivePosition() string {
	if s.Kind == SnippetKindHTML {
		return SnippetPositionBodyEnd
	}
	return s.Position
}

// IsHighRisk 判定该片段类型是否属于「写入即需人工确认」的高风险类。
// css 只影响样式，最坏是站点变丑（可撤销）；js/html 等于对全站访客执行内容。
func (s Snippet) IsHighRisk() bool {
	return s.Kind == SnippetKindJS || s.Kind == SnippetKindHTML
}
