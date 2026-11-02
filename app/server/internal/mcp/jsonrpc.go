// Package mcp 实现站点对外暴露的 MCP Server（P2）。
//
// 挂载位置是应用根 `/mcp`，不在 /api/v1 之下——MCP 客户端按约定路径发现服务。
// 代价是它拿不到 gateway.Preflight / Enforce 的任何保护，因此 Enforcer 不是
// 可选项：维护模式、ban、权限、限流四项检查必须在这里补齐（见 enforcer.go）。
package mcp

import (
	"encoding/json"
	"fmt"
)

// JSON-RPC 2.0 错误码（MCP 复用标准码，另加少量工具级语义）。
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
	// CodeToolDisabled 表示工具存在但被站长停用。用自定义码而非
	// MethodNotFound，便于客户端区分「不认识」与「关掉了」。
	CodeToolDisabled = -32010
	// CodeForbidden 表示权限 / 封禁 / 维护模式拒绝。
	CodeForbidden = -32011
	// CodeRateLimited 表示超出限流预算。
	CodeRateLimited = -32012
)

// Request 是 JSON-RPC 2.0 请求。
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response 是 JSON-RPC 2.0 响应。
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error 是 JSON-RPC 2.0 错误对象。
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *Error) Error() string { return fmt.Sprintf("JSON-RPC %d: %s", e.Code, e.Message) }

// newError 构造错误对象。
func newError(code int, msg string) *Error { return &Error{Code: code, Message: msg} }

// successResponse 构造成功响应。ID 原样回传，缺失时用 null。
func successResponse(id json.RawMessage, result any) Response {
	if id == nil {
		id = json.RawMessage("null")
	}
	return Response{JSONRPC: "2.0", ID: id, Result: result}
}

// errorResponse 构造错误响应。
func errorResponse(id json.RawMessage, e *Error) Response {
	if id == nil {
		id = json.RawMessage("null")
	}
	return Response{JSONRPC: "2.0", ID: id, Error: e}
}

// AuditActionPrefix 是 MCP 工具调用审计的动作前缀。
//
// 集中成常量：审计页按前缀筛选「MCP 调用」，散落的字面量会让筛选漏项。
const AuditActionPrefix = "mcp.tool."
