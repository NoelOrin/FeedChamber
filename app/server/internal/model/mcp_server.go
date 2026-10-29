package model

import "time"

// 外部 MCP Server 的用途。moderation 表示该服务承担内容审核职责，
// 会在 moderation_mode=external 链路上被优先调用（见 mcp-client 规范）。
const (
	MCPServerPurposeGeneral    = "general"
	MCPServerPurposeModeration = "moderation"
)

// MCPServer 是一条外部 MCP Server 登记。
//
// Headers 存 JSON 对象（鉴权头等），在接口响应里 MUST 脱敏（见 mcp-client 规范）。
type MCPServer struct {
	ID        string `gorm:"primaryKey;size:36" json:"id"`
	Name      string `gorm:"size:128;not null" json:"name"`
	Endpoint  string `gorm:"size:512;not null" json:"endpoint"`
	Transport string `gorm:"size:32;not null;default:'streamable_http'" json:"transport"`
	Headers   string `gorm:"type:text" json:"-"`
	Purpose   string `gorm:"size:32;not null;default:'general';index" json:"purpose"`
	Enabled   bool   `gorm:"not null;default:false;index" json:"enabled"`
	// ModerationTool 指定审核用途时要调用的工具名（如 "check_content"）。
	// 为空时用默认工具名。
	ModerationTool string    `gorm:"size:128" json:"moderation_tool"`
	UpdatedBy      string    `gorm:"size:36" json:"updated_by"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (MCPServer) TableName() string { return "mcp_servers" }

const (
	MCPTransportStreamableHTTP = "streamable_http"
	// DefaultModerationTool 是未显式指定审核工具时使用的名字。
	DefaultModerationTool = "check_content"
)

// ValidMCPTransport 校验传输方式。当前只支持 Streamable HTTP：
// stdio 需要子进程管理，不属于服务端形态。
func ValidMCPTransport(t string) bool {
	return t == MCPTransportStreamableHTTP
}

// ValidMCPServerPurpose 校验用途取值。
func ValidMCPServerPurpose(p string) bool {
	return p == MCPServerPurposeGeneral || p == MCPServerPurposeModeration
}
