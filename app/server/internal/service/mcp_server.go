package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/repository"

	"github.com/sirupsen/logrus"
)

// mcpClientTimeout 是单次外部 MCP 调用的超时。
//
// 3s 与既有外部审核的 HTTP 契约保持同一量级：审核链路在发帖主链路上，
// 拖长超时等于拖慢发帖。规范要求「超时不会挂起发布」。
const mcpClientTimeout = 3 * time.Second

type mcpServerService struct {
	repo  repository.MCPServerRepository
	audit AuditLogService
	http  *http.Client
}

func NewMCPServerService(repo repository.MCPServerRepository, audit AuditLogService) MCPServerService {
	return &mcpServerService{
		repo:  repo,
		audit: audit,
		http:  &http.Client{Timeout: mcpClientTimeout},
	}
}

// ── CRUD ──────────────────────────────────────────────────────

func (s *mcpServerService) List(ctx context.Context) ([]MCPServerView, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	out := make([]MCPServerView, 0, len(items))
	for _, it := range items {
		out = append(out, toMCPServerView(it))
	}
	return out, nil
}

func validateMCPServerInput(in *MCPServerInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "name 不能为空")
	}
	if strings.TrimSpace(in.Endpoint) == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "endpoint 不能为空")
	}
	if !strings.HasPrefix(in.Endpoint, "http://") && !strings.HasPrefix(in.Endpoint, "https://") {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "endpoint 必须是 http(s) 地址")
	}
	if in.Transport == "" {
		in.Transport = model.MCPTransportStreamableHTTP
	}
	if !model.ValidMCPTransport(in.Transport) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "transport 目前只支持 streamable_http")
	}
	if in.Purpose == "" {
		in.Purpose = model.MCPServerPurposeGeneral
	}
	if !model.ValidMCPServerPurpose(in.Purpose) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "purpose 必须是 general / moderation")
	}
	if in.Headers != "" && !json.Valid([]byte(in.Headers)) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "headers 必须是合法 JSON 对象")
	}
	return nil
}

func (s *mcpServerService) Create(ctx context.Context, in MCPServerInput) (*MCPServerView, error) {
	if err := validateMCPServerInput(&in); err != nil {
		return nil, err
	}
	now := time.Now()
	srv := &model.MCPServer{
		ID:             pkg.NewID(),
		Name:           in.Name,
		Endpoint:       in.Endpoint,
		Transport:      in.Transport,
		Headers:        in.Headers,
		Purpose:        in.Purpose,
		ModerationTool: in.ModerationTool,
		Enabled:        in.Enabled,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.repo.Create(ctx, srv); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.record(ctx, "mcp_server.create", srv.ID, fmt.Sprintf("name=%s purpose=%s", srv.Name, srv.Purpose))
	view := toMCPServerView(*srv)
	return &view, nil
}

func (s *mcpServerService) Update(ctx context.Context, id string, in MCPServerInput) (*MCPServerView, error) {
	if err := validateMCPServerInput(&in); err != nil {
		return nil, err
	}
	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "外部 MCP 服务不存在")
	}
	existing.Name = in.Name
	existing.Endpoint = in.Endpoint
	existing.Transport = in.Transport
	existing.Purpose = in.Purpose
	existing.ModerationTool = in.ModerationTool
	existing.Enabled = in.Enabled
	// Headers 留空表示保持原样：接口返回的是脱敏值，若原样回传会把真实凭据冲掉。
	if in.Headers != "" {
		existing.Headers = in.Headers
	}
	existing.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, mapRepoErr(err, "外部 MCP 服务不存在")
	}
	s.record(ctx, "mcp_server.update", id, fmt.Sprintf("name=%s enabled=%v", existing.Name, existing.Enabled))
	view := toMCPServerView(*existing)
	return &view, nil
}

func (s *mcpServerService) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return mapRepoErr(err, "外部 MCP 服务不存在")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return mapRepoErr(err, "外部 MCP 服务不存在")
	}
	s.record(ctx, "mcp_server.delete", id, "")
	return nil
}

func (s *mcpServerService) ModerationServers(ctx context.Context) ([]model.MCPServer, error) {
	items, err := s.repo.ListEnabledByPurpose(ctx, model.MCPServerPurposeModeration)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s *mcpServerService) TestConnection(ctx context.Context, id string) (*MCPTestResult, error) {
	srv, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "外部 MCP 服务不存在")
	}
	tools, err := s.listTools(ctx, srv)
	if err != nil {
		// 测试失败只是结果，不是接口错误：把原因回给调用方即可，
		// 已保存的配置不受影响。
		return &MCPTestResult{OK: false, Error: err.Error()}, nil
	}
	return &MCPTestResult{OK: true, Tools: tools}, nil
}

func (s *mcpServerService) record(ctx context.Context, action, target, detail string) {
	if s.audit == nil {
		return
	}
	_ = s.audit.Record(ctx, "system", action, target, detail)
}

// ── MCP 客户端（最小实现） ────────────────────────────────────

// jsonrpcRequest / jsonrpcResponse 是最小 JSON-RPC 2.0 形状。
type jsonrpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *jsonrpcError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// call 发一次 JSON-RPC 调用。外部服务可能要求先 initialize，
// 因此由调用方决定调用序列。
func (s *mcpServerService) call(ctx context.Context, srv *model.MCPServer, method string, params any) (json.RawMessage, error) {
	body, err := json.Marshal(jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return nil, err
	}

	// 用 background + 自己的超时，而不是继承请求 ctx：外部服务响应慢
	// 不应该因为上游 HTTP 请求被客户端取消而变成「配置测试失败」。
	reqCtx, cancel := context.WithTimeout(context.Background(), mcpClientTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, srv.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	// 外部服务的鉴权头存在 Headers JSON 里。
	for k, v := range parseHeaders(srv.Headers) {
		req.Header.Set(k, v)
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}

	// Streamable HTTP 可能返回 text/event-stream，取出 data: 负载行。
	payload := raw
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		payload = extractSSEData(raw)
	}

	var out jsonrpcResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, fmt.Errorf("响应不是合法 JSON-RPC: %w", err)
	}
	if out.Error != nil {
		return nil, out.Error
	}
	return out.Result, nil
}

// extractSSEData 从 SSE 流里取出最后一条 data: 负载。
//
// 只取最后一条：MCP 的 Streamable HTTP 在单次响应里可能推送通知再推结果，
// 最终结果总在最后。
func extractSSEData(raw []byte) []byte {
	var last []byte
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "data:") {
			last = []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if last == nil {
		return raw
	}
	return last
}

func parseHeaders(raw string) map[string]string {
	out := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// initialize 完成 MCP 握手。失败不阻断后续调用：部分实现允许直接 tools/list，
// 但规范建议先 initialize，这里按规范做，失败时把错误交给调用方。
func (s *mcpServerService) initialize(ctx context.Context, srv *model.MCPServer) error {
	_, err := s.call(ctx, srv, "initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "feedchamber",
			"version": "1.0.0",
		},
	})
	return err
}

func (s *mcpServerService) listTools(ctx context.Context, srv *model.MCPServer) ([]string, error) {
	if err := s.initialize(ctx, srv); err != nil {
		return nil, err
	}
	result, err := s.call(ctx, srv, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, fmt.Errorf("tools/list 结果无法解析: %w", err)
	}
	names := make([]string, 0, len(parsed.Tools))
	for _, t := range parsed.Tools {
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return names, nil
}

// CallTool 供审核链路调用外部工具。导出是因为 moderation.go 需要它。
func (s *mcpServerService) CallTool(ctx context.Context, srv *model.MCPServer, tool string, args map[string]any) (json.RawMessage, error) {
	if err := s.initialize(ctx, srv); err != nil {
		logrus.Warnf("mcp: initialize %s failed: %v", srv.Name, err)
		return nil, err
	}
	return s.call(ctx, srv, "tools/call", map[string]any{
		"name":      tool,
		"arguments": args,
	})
}

// ── 视图转换 ──────────────────────────────────────────────────

func toMCPServerView(m model.MCPServer) MCPServerView {
	keys := make([]string, 0)
	for k := range parseHeaders(m.Headers) {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return MCPServerView{
		ID:             m.ID,
		Name:           m.Name,
		Endpoint:       m.Endpoint,
		Transport:      m.Transport,
		HeaderKeys:     keys,
		Purpose:        m.Purpose,
		ModerationTool: m.ModerationTool,
		Enabled:        m.Enabled,
		UpdatedBy:      m.UpdatedBy,
		CreatedAt:      m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      m.UpdatedAt.Format(time.RFC3339),
	}
}
