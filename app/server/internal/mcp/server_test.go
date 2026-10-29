package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"feedchamber/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer 构造一个最小可用的 Server：读工具启用、写工具默认停用。
func newTestServer(t *testing.T) (*Server, *Registry) {
	t.Helper()
	enf := testEnforcer(enforcerOpts{
		role: "admin",
		granted: map[string]bool{
			"admin|" + model.PermPostRead:       true,
			"admin|" + model.PermCustomizeRead:  true,
			"admin|" + model.PermCustomizeWrite: true,
			"admin|" + model.PermPostCreate:     true,
		},
	})
	reg := NewRegistry()
	reg.Register(Tool{
		Name: "read_tool", Description: "读工具", Perm: model.PermPostRead, Write: false,
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Handler: func(ctx context.Context, in ToolInput) (any, error) {
			return map[string]any{"ok": true}, nil
		},
	})
	reg.Register(Tool{
		Name: "write_tool", Description: "写工具", Perm: model.PermPostCreate, Write: true,
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Handler: func(ctx context.Context, in ToolInput) (any, error) {
			return map[string]any{"written": true}, nil
		},
	})
	reg.ApplyDisabledList("")
	return NewServer(enf, reg, nil), reg
}

func callBody(t *testing.T, method string, params any) []byte {
	t.Helper()
	req := map[string]any{"jsonrpc": "2.0", "id": 1, "method": method}
	if params != nil {
		req["params"] = params
	}
	raw, err := json.Marshal(req)
	require.NoError(t, err)
	return raw
}

func keyHeader() http.Header {
	h := http.Header{}
	h.Set("X-API-Key", "ak_key1")
	return h
}

// TestServer_InitializeShape 握手必须回 JSON-RPC 形状（不能套论坛信封）。
func TestServer_InitializeShape(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := srv.Handle(context.Background(), keyHeader(), callBody(t, "initialize", nil))
	require.Nil(t, resp.Error)
	assert.Equal(t, "2.0", resp.JSONRPC)

	// 结果里要有协议版本与 serverInfo。
	raw, err := json.Marshal(resp.Result)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "protocolVersion")
	assert.Contains(t, string(raw), "feedchamber")
}

// TestServer_ToolsListHidesDisabled 停用的写工具不得出现在清单里。
func TestServer_ToolsListHidesDisabled(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := srv.Handle(context.Background(), keyHeader(), callBody(t, "tools/list", nil))
	require.Nil(t, resp.Error)

	raw, _ := json.Marshal(resp.Result)
	assert.Contains(t, string(raw), "read_tool")
	assert.NotContains(t, string(raw), "write_tool", "默认停用的写工具不应出现在 tools/list")
}

// TestServer_DisabledToolDistinguishableFromUnknown
// 「停用」与「不存在」必须可区分：客户端据此提示站长开启，而不是怀疑工具名。
func TestServer_DisabledToolDistinguishableFromUnknown(t *testing.T) {
	srv, _ := newTestServer(t)

	disabled := srv.Handle(context.Background(), keyHeader(),
		callBody(t, "tools/call", map[string]any{"name": "write_tool", "arguments": map[string]any{}}))
	require.NotNil(t, disabled.Error)
	assert.Equal(t, CodeToolDisabled, disabled.Error.Code)

	unknown := srv.Handle(context.Background(), keyHeader(),
		callBody(t, "tools/call", map[string]any{"name": "nope", "arguments": map[string]any{}}))
	require.NotNil(t, unknown.Error)
	assert.Equal(t, CodeMethodNotFound, unknown.Error.Code)
}

// TestServer_UnknownMethod 未知方法回 MethodNotFound。
func TestServer_UnknownMethod(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := srv.Handle(context.Background(), keyHeader(), callBody(t, "does/not/exist", nil))
	require.NotNil(t, resp.Error)
	assert.Equal(t, CodeMethodNotFound, resp.Error.Code)
}

// TestServer_MalformedJSON 非法 JSON 回 ParseError。
func TestServer_MalformedJSON(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := srv.Handle(context.Background(), keyHeader(), []byte("{not json"))
	require.NotNil(t, resp.Error)
	assert.Equal(t, CodeParseError, resp.Error.Code)
}

// TestServer_RequiresCredential 无 Key 一律拒绝（MCP 不是公开信息面）。
func TestServer_RequiresCredential(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := srv.Handle(context.Background(), http.Header{}, callBody(t, "initialize", nil))
	require.NotNil(t, resp.Error)
	assert.Equal(t, CodeForbidden, resp.Error.Code)
}

// TestServer_EnabledWriteToolCallable 站长显式开启后写工具可调用。
func TestServer_EnabledWriteToolCallable(t *testing.T) {
	srv, reg := newTestServer(t)
	require.True(t, reg.SetEnabled("write_tool", true))

	resp := srv.Handle(context.Background(), keyHeader(),
		callBody(t, "tools/call", map[string]any{"name": "write_tool", "arguments": map[string]any{}}))
	require.Nil(t, resp.Error, "启用后应可调用")
	raw, _ := json.Marshal(resp.Result)
	assert.Contains(t, string(raw), "written")
}

// TestRegistry_DefaultDisablesWrites 写工具默认停用、读工具默认启用。
func TestRegistry_DefaultDisablesWrites(t *testing.T) {
	_, reg := newTestServer(t)
	assert.True(t, reg.Enabled("read_tool"), "读工具默认启用")
	assert.False(t, reg.Enabled("write_tool"), "写工具默认停用")
}

// TestRegistry_DisabledListRoundTrip 停用名单的序列化与回读必须一致。
//
// 这里钉住「负号表示显式启用写工具」的编码：清空名单只会回到全关，
// 不会意外打开所有写工具。
func TestRegistry_DisabledListRoundTrip(t *testing.T) {
	_, reg := newTestServer(t)

	// 默认状态：写工具关。
	raw := reg.DisabledList()
	assert.Empty(t, raw, "默认状态不应产生名单条目")

	// 显式启用写工具 → 名单里出现负号条目。
	require.True(t, reg.SetEnabled("write_tool", true))
	raw = reg.DisabledList()
	assert.Contains(t, raw, "-write_tool")

	// 回读后仍启用。
	fresh := NewRegistry()
	fresh.Register(Tool{Name: "read_tool", Write: false})
	fresh.Register(Tool{Name: "write_tool", Write: true})
	fresh.ApplyDisabledList(raw)
	assert.True(t, fresh.Enabled("write_tool"), "负号条目应把写工具重新打开")
	assert.True(t, fresh.Enabled("read_tool"))

	// 空名单 → 回到安全默认（写工具关）。
	fresh.ApplyDisabledList("")
	assert.False(t, fresh.Enabled("write_tool"), "空名单必须回到写工具关闭的安全默认")
}
