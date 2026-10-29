package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"feedchamber/internal/pkg"

	"github.com/sirupsen/logrus"
)

// Server 是 MCP 的协议入口：处理 initialize / tools/list / tools/call。
type Server struct {
	enforcer *Enforcer
	registry *Registry
	audit    AuditRecorder
	// maxResultBytes 是单次工具结果上限，超出时截断正文（见 design Open Questions）。
	maxResultBytes int
}

// AuditRecorder 记录一次工具调用。抽成接缝便于测试与解耦 service 包。
type AuditRecorder interface {
	RecordToolCall(ctx context.Context, actorID, tool string, ok bool, detail string) error
}

// NewServer 构造 MCP Server。
func NewServer(enforcer *Enforcer, registry *Registry, audit AuditRecorder) *Server {
	return &Server{
		enforcer:       enforcer,
		registry:       registry,
		audit:          audit,
		maxResultBytes: 8192,
	}
}

// SetMaxResultBytes 覆盖结果体积上限。
func (s *Server) SetMaxResultBytes(n int) {
	if n > 0 {
		s.maxResultBytes = n
	}
}

// protocolVersion 是协商时回报的协议版本。
const protocolVersion = "2025-06-18"

// Handle 是一次 HTTP 请求的完整处理入口。
//
// 它自己完成鉴权与授权（不经过网关），因为 /mcp 挂在应用根。
func (s *Server) Handle(ctx context.Context, header http.Header, body []byte) Response {
	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		return errorResponse(nil, newError(CodeParseError, "请求不是合法 JSON"))
	}
	if req.JSONRPC != "2.0" || strings.TrimSpace(req.Method) == "" {
		return errorResponse(req.ID, newError(CodeInvalidRequest, "不是合法的 JSON-RPC 2.0 请求"))
	}

	// initialize 也要求凭证：MCP 服务本身不是公开信息面。
	ident, authErr := s.enforcer.Authenticate(ctx, header)
	if authErr != nil {
		return errorResponse(req.ID, authErr)
	}
	ident.CanPublish = s.enforcer.CanPublishCustomize(ident)

	switch req.Method {
	case "initialize":
		return successResponse(req.ID, s.initializeResult())
	case "tools/list":
		return successResponse(req.ID, s.listResult(ident))
	case "tools/call":
		return s.handleToolCall(ctx, req, ident)
	case "notifications/initialized", "ping":
		// MCP 客户端握手后会发 initialized 通知，ping 用于保活。
		// 两者都不需要结果体，但协议上仍回成功。
		return successResponse(req.ID, map[string]any{})
	default:
		return errorResponse(req.ID, newError(CodeMethodNotFound, "未知方法: "+req.Method))
	}
}

func (s *Server) initializeResult() map[string]any {
	return map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "feedchamber",
			"version": "1.0.0",
		},
	}
}

// listResult 只列对当前身份可见且已启用的工具。
//
// 权限不足的工具不出现在清单里：这既减少噪声，也避免把
// 「存在但你不能用」暴露成攻击面地图。
func (s *Server) listResult(ident *Identity) map[string]any {
	tools := s.registry.List(func(t Tool) bool {
		return s.enforcer.Authorize(context.Background(), ident, t) == nil
	})
	out := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		out = append(out, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": json.RawMessage(orDefaultSchema(t.InputSchema)),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i]["name"].(string) < out[j]["name"].(string)
	})
	return map[string]any{"tools": out}
}

func orDefaultSchema(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte(`{"type":"object","properties":{}}`)
	}
	return raw
}

// handleToolCall 执行一次工具调用。
func (s *Server) handleToolCall(ctx context.Context, req Request, ident *Identity) Response {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return errorResponse(req.ID, newError(CodeInvalidParams, "params 不是合法对象"))
		}
	}
	if params.Name == "" {
		return errorResponse(req.ID, newError(CodeInvalidParams, "缺少工具名"))
	}

	tool, ok := s.registry.Get(params.Name)
	if !ok {
		return errorResponse(req.ID, newError(CodeMethodNotFound, "未知工具: "+params.Name))
	}
	if !s.registry.Enabled(params.Name) {
		// 停用与不存在要能区分：客户端据此提示站长开启，而不是怀疑工具名。
		return errorResponse(req.ID, newError(CodeToolDisabled, "工具已被停用: "+params.Name))
	}
	if authErr := s.enforcer.Authorize(ctx, ident, *tool); authErr != nil {
		s.recordAudit(ctx, ident, tool.Name, false, authErr.Message)
		return errorResponse(req.ID, authErr)
	}

	result, err := tool.Handler(ctx, ToolInput{
		UserID:     ident.UserID,
		Role:       ident.Role,
		Scopes:     ident.Scopes,
		CanPublish: ident.CanPublish,
		Args:       params.Arguments,
	})
	if err != nil {
		// 业务失败表达为工具级错误结果，不是 JSON-RPC error。
		s.recordAudit(ctx, ident, tool.Name, false, err.Error())
		return successResponse(req.ID, errorResult(errMessage(err)))
	}

	res, err := textResult(result)
	if err != nil {
		s.recordAudit(ctx, ident, tool.Name, false, "结果序列化失败")
		return successResponse(req.ID, errorResult("结果序列化失败"))
	}
	s.truncate(res)
	s.recordAudit(ctx, ident, tool.Name, true, summarize(result))
	return successResponse(req.ID, res)
}

// truncate 按上限截断内容，并在结尾标注被截断。
func (s *Server) truncate(res *ToolResult) {
	for i := range res.Content {
		if len(res.Content[i].Text) > s.maxResultBytes {
			res.Content[i].Text = res.Content[i].Text[:s.maxResultBytes] + "\n…（结果已截断）"
		}
	}
}

func (s *Server) recordAudit(ctx context.Context, ident *Identity, tool string, ok bool, detail string) {
	if s.audit == nil {
		return
	}
	if err := s.audit.RecordToolCall(ctx, ident.UserID, tool, ok, detail); err != nil {
		// 审计失败不阻断工具执行，但必须留痕到日志——否则「没有审计记录」
		// 会被误读为「没有调用」。
		logrus.Warnf("mcp: audit record for tool %s failed: %v", tool, err)
	}
}

// errMessage 提取业务错误的可读文案。
//
// service 层返回 *pkg.AppError，它的 Error() 带错误码前缀；直接给 AI 客户端
// 一个干净的中文原因是更好的体验。
func errMessage(err error) string {
	if appErr, ok := err.(*pkg.AppError); ok {
		return appErr.Message
	}
	return err.Error()
}

// summarize 生成审计用的结果摘要，避免把整篇帖子写进 audit_logs。
func summarize(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	s := string(raw)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
