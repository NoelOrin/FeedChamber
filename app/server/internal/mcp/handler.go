package mcp

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/kataras/iris/v12"
)

// Handler 是 /mcp 的 HTTP 入口。
//
// 它挂应用根而非 /api/v1，因此**不使用** pkg.Success 信封：MCP 客户端按
// JSON-RPC 2.0 解析（api-style 规范里登记为协议端点例外，与 OAuth/OIDC 同构）。
type Handler struct {
	server *Server
}

func NewHandler(server *Server) *Handler {
	return &Handler{server: server}
}

// Serve 处理一次 MCP 请求。
func (h *Handler) Serve(ctx iris.Context) {
	// MCP 的请求体可能较大（工具参数里带正文），但不应无限制。
	// 独立读而不是复用 v1 的 PathBodyLimit：那个中间件只挂在 /api/v1。
	body, err := io.ReadAll(io.LimitReader(ctx.Request().Body, 1<<20))
	if err != nil {
		ctx.StatusCode(http.StatusBadRequest)
		writeJSON(ctx, errorResponse(nil, newError(CodeParseError, "读取请求体失败")))
		return
	}

	reqCtx, cancel := context.WithTimeout(ctx.Request().Context(), 30*time.Second)
	defer cancel()

	resp := h.server.Handle(reqCtx, ctx.Request().Header, body)
	// JSON-RPC 层面的错误仍然回 200：协议要求错误走响应体，
	// 只有「连 JSON-RPC 都算不上」才用 HTTP 状态码区分。
	ctx.StatusCode(http.StatusOK)
	ctx.ContentType("application/json")
	writeJSON(ctx, resp)
}

// writeJSON 直接写 JSON。
//
// 这里**故意**绕过 pkg.Success：MCP 是协议端点，套论坛信封会破坏互操作。
// 与 OAuth/OIDC 端点的既有例外同构（见 api-style 规范）。
func writeJSON(ctx iris.Context, v any) {
	_ = ctx.JSON(v)
}

// ToolStates 返回全部工具及其启停状态，供后台管理页展示。
func (s *Server) ToolStates() []ToolState { return s.registry.All() }

// SetToolEnabled 启停单个工具并返回是否找到该工具。
func (s *Server) SetToolEnabled(name string, enabled bool) bool {
	return s.registry.SetEnabled(name, enabled)
}

// Registry 暴露注册表，供后台构建持久化名单。
func (s *Server) Registry() *Registry { return s.registry }

// ToolStates 代理到 Server，供后台管理页读取工具启停状态。
func (h *Handler) ToolStates() []ToolState { return h.server.ToolStates() }

// SetToolEnabled 代理到 Server，供后台启停单个工具。
func (h *Handler) SetToolEnabled(name string, enabled bool) bool {
	return h.server.SetToolEnabled(name, enabled)
}

// Registry 暴露工具注册表，供后台持久化停用名单。
func (h *Handler) Registry() *Registry { return h.server.Registry() }
