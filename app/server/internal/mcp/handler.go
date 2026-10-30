package mcp

import (
	"context"
	"errors"
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
	// store 是工具启停的持久化出口。为 nil 时只改内存（单实例、无设置服务）。
	store ToolStateStore
}

// ToolStateStore 持久化工具启停覆盖。
//
// 只收一个字符串而不是「名字 + 开关」：编码格式（负号表示显式启用写工具）
// 属于 Registry，Store 只是一个不会解析它的信封。
type ToolStateStore interface {
	SaveToolState(ctx context.Context, raw string) error
}

// ErrToolNotFound 表示工具名不存在——与「保存失败」必须可区分。
var ErrToolNotFound = errors.New("mcp: tool not found")

func NewHandler(server *Server) *Handler {
	return &Handler{server: server}
}

// SetToolStateStore 注入持久化出口。容器装配时调用一次。
func (h *Handler) SetToolStateStore(store ToolStateStore) { h.store = store }

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

// SetToolEnabled 启停单个工具并**持久化**。
//
// 顺序刻意是「先落库、后改内存」的反向：先算出目标状态、写库、再应用到内存，
// 保存失败时内存保持不变——后台不会出现「界面显示已开、重启又关」的假成功。
func (h *Handler) SetToolEnabled(ctx context.Context, name string, enabled bool) error {
	reg := h.server.Registry()
	if !reg.Exists(name) {
		return ErrToolNotFound
	}
	if h.store == nil {
		reg.SetEnabled(name, enabled)
		return nil
	}

	previous := reg.Enabled(name)
	reg.SetEnabled(name, enabled)
	if err := h.store.SaveToolState(ctx, reg.DisabledList()); err != nil {
		reg.SetEnabled(name, previous) // 回滚，保持一致
		return err
	}
	return nil
}

// Registry 暴露工具注册表，供后台持久化停用名单。
func (h *Handler) Registry() *Registry { return h.server.Registry() }
