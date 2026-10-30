package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"feedchamber/internal/model"
	"feedchamber/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memMCPServerRepo 是 MCPServerRepository 的内存替身。
type memMCPServerRepo struct {
	items map[string]*model.MCPServer
}

func newMemMCPServerRepo() *memMCPServerRepo {
	return &memMCPServerRepo{items: map[string]*model.MCPServer{}}
}

func (r *memMCPServerRepo) List(_ context.Context) ([]model.MCPServer, error) {
	out := make([]model.MCPServer, 0, len(r.items))
	for _, it := range r.items {
		out = append(out, *it)
	}
	return out, nil
}
func (r *memMCPServerRepo) Get(_ context.Context, id string) (*model.MCPServer, error) {
	it, ok := r.items[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return it, nil
}
func (r *memMCPServerRepo) Create(_ context.Context, s *model.MCPServer) error {
	r.items[s.ID] = s
	return nil
}
func (r *memMCPServerRepo) Update(_ context.Context, s *model.MCPServer) error {
	r.items[s.ID] = s
	return nil
}
func (r *memMCPServerRepo) Delete(_ context.Context, id string) error {
	delete(r.items, id)
	return nil
}
func (r *memMCPServerRepo) ListEnabledByPurpose(_ context.Context, purpose string) ([]model.MCPServer, error) {
	out := make([]model.MCPServer, 0, len(r.items))
	for _, it := range r.items {
		if it.Enabled && it.Purpose == purpose {
			out = append(out, *it)
		}
	}
	return out, nil
}

// endpoint 是管理员填的 URL，服务端会去请求它 —— 典型的 SSRF 面。
// 只校验 http(s) 前缀挡不住内网探测，必须拒私网/回环/链路本地。
func TestValidateEndpointRejectsSSRFTargets(t *testing.T) {
	cases := map[string]bool{
		"https://mcp.example.com/tool":  true,
		"http://203.0.113.10:8080/x":    true,
		"http://127.0.0.1:8080/x":       false,
		"http://localhost:8080/x":       false,
		"http://0.0.0.0/x":              false,
		"http://[::1]/x":                false,
		"http://192.168.1.10/x":         false,
		"http://10.0.0.5/x":             false,
		"http://172.16.3.4/x":           false,
		"http://169.254.169.254/latest": false,
		"file:///etc/passwd":            false,
		"ftp://example.com/x":           false,
		"not-a-url":                     false,
	}

	for endpoint, valid := range cases {
		err := validateEndpoint(endpoint)
		if valid {
			assert.NoError(t, err, "endpoint=%q 应放行", endpoint)
			continue
		}
		require.Error(t, err, "endpoint=%q 应被拒", endpoint)
	}
}

// 旧实现只做 json.Valid，`[1,2]` / `{"X-Key":123}` 都能入库，
// 随后反序列化失败被静默吞掉 → 鉴权头全丢、外部 401、无任何线索。
func TestValidateHeaderFormatRequiresStringMap(t *testing.T) {
	assert.NoError(t, validateHeaderFormat(""))
	assert.NoError(t, validateHeaderFormat(`{"X-API-Key":"k"}`))
	assert.Error(t, validateHeaderFormat(`[1,2]`))
	assert.Error(t, validateHeaderFormat(`"str"`))
	assert.Error(t, validateHeaderFormat(`{"X-Key":123}`))
	assert.Error(t, validateHeaderFormat(`{`))
}

// 不跟随重定向：Go 只跨域剥离 Authorization/Cookie，
// 自定义鉴权头会被原样带到重定向目标。
func TestClientDoesNotFollowRedirects(t *testing.T) {
	var gotHeader string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirector.Close()

	svc := NewMCPServerService(newMemMCPServerRepo(), nil).(*mcpServerService)
	srv := &model.MCPServer{
		ID: "s1", Endpoint: redirector.URL, Headers: `{"X-API-Key":"secret"}`,
	}
	_, err := svc.call(context.Background(), srv, "initialize", nil)
	require.Error(t, err, "重定向应被当作失败而不是跟随")
	assert.Empty(t, gotHeader, "鉴权头不得被带到重定向目标")
}

// 响应超限必须报「响应过大」，而不是被截断后报「不是合法 JSON-RPC」。
func TestCallRejectsOversizedResponse(t *testing.T) {
	big := strings.Repeat("x", maxMCPResponseBytes+1024)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":"%s"}`, big)
	}))
	defer ts.Close()

	svc := NewMCPServerService(newMemMCPServerRepo(), nil).(*mcpServerService)
	_, err := svc.call(context.Background(), &model.MCPServer{ID: "s1", Endpoint: ts.URL}, "tools/list", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "响应过大")
}

// JSON-RPC 2.0 允许字符串 id；声明成 int 会让整条响应 Unmarshal 失败。
// 且响应 id 必须与请求 id 一致（SSE 里可能混进通知帧）。
func TestCallValidatesResponseID(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantErr string
	}{
		{"数字 id 正确", `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`, ""},
		{"字符串 id 同值", `{"jsonrpc":"2.0","id":"1","result":{"ok":true}}`, ""},
		{"id 不匹配", `{"jsonrpc":"2.0","id":2,"result":{"ok":true}}`, "不匹配"},
		{"通知帧无 result", `{"jsonrpc":"2.0","method":"notifications/progress"}`, "不匹配"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tc.payload)
			}))
			defer ts.Close()

			svc := NewMCPServerService(newMemMCPServerRepo(), nil).(*mcpServerService)
			_, err := svc.call(context.Background(), &model.MCPServer{ID: "s1", Endpoint: ts.URL}, "initialize", nil)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// 有状态实现要求回传 Mcp-Session-Id，否则第二个请求会被拒。
func TestCallReusesSessionID(t *testing.T) {
	var seen []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Mcp-Session-Id"))
		w.Header().Set("Content-Type", "application/json")
		if len(seen) == 1 {
			w.Header().Set("Mcp-Session-Id", "sess-abc")
		}
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
	}))
	defer ts.Close()

	svc := NewMCPServerService(newMemMCPServerRepo(), nil).(*mcpServerService)
	srv := &model.MCPServer{ID: "s1", Endpoint: ts.URL}
	_, err := svc.call(context.Background(), srv, "initialize", nil)
	require.NoError(t, err)
	_, err = svc.call(context.Background(), srv, "tools/list", nil)
	require.NoError(t, err)

	require.Len(t, seen, 2)
	assert.Empty(t, seen[0], "首次请求还没有 session")
	assert.Equal(t, "sess-abc", seen[1], "第二次必须回传服务端下发的 session")

	// 配置变更后旧 session 失效：下一次调用重新握手。
	svc.forgetSession("s1")
	assert.Empty(t, svc.sessionOf("s1"))
}

// 创建时就要拒绝非法 endpoint/headers，而不是等到调用外部服务才失败。
func TestCreateRejectsBadEndpointAndHeaders(t *testing.T) {
	svc := NewMCPServerService(newMemMCPServerRepo(), nil)
	ctx := context.Background()

	_, err := svc.Create(ctx, MCPServerInput{Name: "x", Endpoint: "http://127.0.0.1/mcp"})
	require.Error(t, err)

	_, err = svc.Create(ctx, MCPServerInput{
		Name: "x", Endpoint: "https://mcp.example.com/mcp", Headers: `{"X-Key":123}`,
	})
	require.Error(t, err)

	view, err := svc.Create(ctx, MCPServerInput{
		Name: "x", Endpoint: "https://mcp.example.com/mcp", Headers: `{"X-Key":"v"}`,
	})
	require.NoError(t, err)
	require.NotNil(t, view)
	// 视图只回键名，不回凭据值。
	assert.Equal(t, []string{"X-Key"}, view.HeaderKeys)
}

// 确保 JSON-RPC 请求体形状稳定（id 恒为 1，否则上面的 id 校验无从谈起）。
func TestRequestPayloadShape(t *testing.T) {
	raw, err := json.Marshal(jsonrpcRequest{JSONRPC: "2.0", ID: 1, Method: "initialize"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, string(raw))
}
