package service

import (
	"context"
	"encoding/json"

	"feedchamber/internal/model"
)

// MCPServerService 管理「外部 MCP Server」登记，并提供连通性测试。
//
// 这是 P3 的能力源方向：站点作为 MCP 客户端去调用外部服务（审核、摘要等），
// 与 P2 的 MCP Server（对外暴露）方向相反。
type MCPServerService interface {
	List(ctx context.Context) ([]MCPServerView, error)
	Create(ctx context.Context, in MCPServerInput) (*MCPServerView, error)
	Update(ctx context.Context, id string, in MCPServerInput) (*MCPServerView, error)
	Delete(ctx context.Context, id string) error
	// TestConnection 对指定服务执行 initialize + tools/list，返回工具清单或失败原因。
	// 测试失败 MUST NOT 修改已保存的配置。
	TestConnection(ctx context.Context, id string) (*MCPTestResult, error)
	// ModerationServers 返回承担审核职责且已启用的服务，供审核链路使用。
	ModerationServers(ctx context.Context) ([]model.MCPServer, error)
	// CallTool 对指定外部服务调用一个工具，返回原始 JSON-RPC result。
	// 审核链路（moderation.go 的 CheckExternal）用它取外部审核结论。
	CallTool(ctx context.Context, srv *model.MCPServer, tool string, args map[string]any) (json.RawMessage, error)
}

// MCPServerView 是登记的对外视图。Headers 只返回键名，不返回值——
// 值可能是 Bearer token 之类的凭据（见 mcp-client 规范「凭据不以明文返回」）。
type MCPServerView struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Endpoint       string   `json:"endpoint"`
	Transport      string   `json:"transport"`
	HeaderKeys     []string `json:"header_keys,omitempty"`
	Purpose        string   `json:"purpose"`
	ModerationTool string   `json:"moderation_tool,omitempty"`
	Enabled        bool     `json:"enabled"`
	UpdatedBy      string   `json:"updated_by,omitempty"`
	CreatedAt      string   `json:"created_at,omitempty"`
	UpdatedAt      string   `json:"updated_at,omitempty"`
}

// MCPServerInput 是登记写入参数。Headers 为 JSON 对象字符串；
// 更新时留空表示「保持原样」（避免用脱敏值冲掉真实凭据）。
type MCPServerInput struct {
	Name           string `json:"name"`
	Endpoint       string `json:"endpoint"`
	Transport      string `json:"transport"`
	Headers        string `json:"headers"`
	Purpose        string `json:"purpose"`
	ModerationTool string `json:"moderation_tool"`
	Enabled        bool   `json:"enabled"`
}

// MCPTestResult 是连通性测试结果。
type MCPTestResult struct {
	OK    bool     `json:"ok"`
	Tools []string `json:"tools,omitempty"`
	Error string   `json:"error,omitempty"`
}
