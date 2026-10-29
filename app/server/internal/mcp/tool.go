package mcp

import (
	"context"
	"encoding/json"
)

// Tool 是一个 MCP 工具的定义与实现。
//
// 实现一律直调 service 层，不自调用 HTTP：自调用要多一次往返、要自己构造 Bearer，
// 还会把 service 层的所有权 / 门控 / 可见性校验跑两遍。
type Tool struct {
	Name        string
	Description string
	// InputSchema 是 JSON Schema 对象，原样回给客户端。
	InputSchema json.RawMessage
	// Perm 是调用该工具所需的权限码。空表示登录即可（仍受 ban / 限流约束）。
	Perm string
	// Write 标记这是否是写类工具。写工具默认停用，需站长显式开启。
	Write bool
	// CustomizeWrite 标记这是否会改站点自定义配置。用于审计归类与
	// 分级生效判定——这类工具一律以 WriteSourceMachine 调 service。
	CustomizeWrite bool
	// Handler 执行工具。返回值会被序列化为 MCP 的 content 结果。
	Handler func(ctx context.Context, in ToolInput) (any, error)
}

// ToolInput 是一次工具调用的上下文与参数。
type ToolInput struct {
	// UserID 是凭证所属用户（由 Enforcer 解开 API Key 后填入）。
	UserID string
	// Role 是凭证所属用户的角色。
	Role string
	// Scopes 是 API Key 的原始 scope 串。
	Scopes string
	// CanPublish 表示该凭证是否持 customize:publish（决定高风险内容能否直生效）。
	CanPublish bool
	// Args 是客户端传来的 arguments 对象（原始 JSON）。
	Args json.RawMessage
}

// DecodeArgs 把 Args 解到目标结构。允许空参数。
func (in ToolInput) DecodeArgs(dst any) error {
	if len(in.Args) == 0 {
		return nil
	}
	return json.Unmarshal(in.Args, dst)
}

// ToolResult 是 tools/call 的 result 形状（MCP 规范的 content 数组）。
type ToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ContentItem 是 MCP 内容项。当前只用 text；结构化数据以 JSON 文本承载，
// 这样任何客户端都能读，不依赖对结构化 content 的扩展支持。
type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// textResult 把任意值序列化为单条 text 内容。
func textResult(v any) (*ToolResult, error) {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return &ToolResult{Content: []ContentItem{{Type: "text", Text: string(raw)}}}, nil
}

// errorResult 把工具级失败表达为 MCP 结果（而非 JSON-RPC 错误）。
//
// 规范要求「业务失败以工具级错误表达而非 HTTP 层错误」：JSON-RPC 层的
// error 留给协议问题，业务拒绝走 isError=true 的结果，客户端才能把
// 「工具执行失败」和「通信失败」区分开。
func errorResult(msg string) *ToolResult {
	return &ToolResult{
		Content: []ContentItem{{Type: "text", Text: msg}},
		IsError: true,
	}
}
