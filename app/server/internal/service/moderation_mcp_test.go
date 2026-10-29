package service

import (
	"context"
	"encoding/json"
	"testing"

	"feedchamber/internal/model"
	"feedchamber/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── P3：外部审核优先走 MCP 的分支 ─────────────────────────────
//
// 这条分支的行为契约见 mcp-client 规范：
//   1. 配了承担审核职责的 MCP Server → 走 MCP tools/call；
//   2. 没配 → 静默回退既有 HTTP 契约（不是错误）；
//   3. 配了但调用失败 → 打警告后回退，绝不静默放行。

// stubMCPServerService 是 MCPServerService 的最小替身。
type stubMCPServerService struct {
	servers []model.MCPServer
	// result 是 tools/call 的返回值；err 非空时模拟调用失败。
	result json.RawMessage
	err    error
	// called 记录是否真的被调用过。
	called bool
	tool   string
}

func (s *stubMCPServerService) List(ctx context.Context) ([]MCPServerView, error) {
	return nil, nil
}
func (s *stubMCPServerService) Create(ctx context.Context, in MCPServerInput) (*MCPServerView, error) {
	return nil, nil
}
func (s *stubMCPServerService) Update(ctx context.Context, id string, in MCPServerInput) (*MCPServerView, error) {
	return nil, nil
}
func (s *stubMCPServerService) Delete(ctx context.Context, id string) error { return nil }
func (s *stubMCPServerService) TestConnection(ctx context.Context, id string) (*MCPTestResult, error) {
	return &MCPTestResult{OK: true}, nil
}
func (s *stubMCPServerService) ModerationServers(ctx context.Context) ([]model.MCPServer, error) {
	return s.servers, nil
}
func (s *stubMCPServerService) CallTool(ctx context.Context, srv *model.MCPServer, tool string, args map[string]any) (json.RawMessage, error) {
	s.called = true
	s.tool = tool
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

// stubModerationRepo 是 ModerationRepository 的最小替身：
// 只实现回退路径会用到的那一个方法（ListConfigs），其余返回零值。
//
// 只测 MCP 分支时不需要真的审核规则表；补齐接口是为了让
// 「未配置 → 回退」这条路径能跑到 GetConfigs 而不是 panic。
type stubModerationRepo struct{}

func (stubModerationRepo) CreateModeration(context.Context, *model.ContentModeration) error {
	return nil
}
func (stubModerationRepo) GetModeration(context.Context, string) (*model.ContentModeration, error) {
	return nil, repository.ErrNotFound
}
func (stubModerationRepo) ListModerations(context.Context, model.ModerationStatus, int, int) ([]model.ContentModeration, int64, error) {
	return nil, 0, nil
}
func (stubModerationRepo) UpdateModerationStatus(context.Context, string, model.ModerationStatus, string) error {
	return nil
}
func (stubModerationRepo) CreateRule(context.Context, *model.ModerationRule) error { return nil }
func (stubModerationRepo) GetRule(context.Context, string) (*model.ModerationRule, error) {
	return nil, repository.ErrNotFound
}
func (stubModerationRepo) ListRules(context.Context, int, int) ([]model.ModerationRule, int64, error) {
	return nil, 0, nil
}
func (stubModerationRepo) ListEnabledRules(context.Context, model.ModerationScene) ([]model.ModerationRule, error) {
	return nil, nil
}
func (stubModerationRepo) UpdateRule(context.Context, *model.ModerationRule) error { return nil }
func (stubModerationRepo) DeleteRule(context.Context, string) error                { return nil }
func (stubModerationRepo) GetConfig(context.Context, string) (*model.ModerationConfig, error) {
	return nil, repository.ErrNotFound
}
func (stubModerationRepo) SetConfig(context.Context, string, string) error { return nil }
func (stubModerationRepo) ListConfigs(context.Context) ([]model.ModerationConfig, error) {
	return nil, nil // 没有配置 → endpoint 为空 → 回退后报不可用
}

// newMCPModerationFixture 构造一个只测 MCP 分支的 moderationService。
//
// settingSvc 为 nil 会让 GetConfigs 走不通，因此走 MCP 分支时必须在
// 调用 MCP 之前返回，这正好验证「优先 MCP」的语义。
func newMCPModerationFixture(mcp *stubMCPServerService) *moderationService {
	return &moderationService{mcpSvc: mcp, repo: stubModerationRepo{}}
}

// TestCheckExternal_UsesMCPWhenConfigured 配了审核用途的服务时应走 MCP。
func TestCheckExternal_UsesMCPWhenConfigured(t *testing.T) {
	mcp := &stubMCPServerService{
		servers: []model.MCPServer{{
			ID: "srv1", Name: "审核", Purpose: model.MCPServerPurposeModeration,
			Enabled: true, ModerationTool: "check_content",
		}},
		// MCP 的 content 文本里带结论
		result: json.RawMessage(`{"content":[{"type":"text","text":"allow"}]}`),
	}
	svc := newMCPModerationFixture(mcp)

	action, err := svc.CheckExternal(context.Background(), "post", "p1", "标题", "正文", model.ModerationScenePost)
	require.NoError(t, err, "MCP 返回 allow 时不应回退")
	assert.Equal(t, model.ModerationActionAllow, action)
	assert.True(t, mcp.called, "应真的调用了 MCP")
	assert.Equal(t, "check_content", mcp.tool, "应使用登记的工具名")
}

// TestCheckExternal_MCPActionField 支持 {"action":"block"} 形态。
func TestCheckExternal_MCPActionField(t *testing.T) {
	mcp := &stubMCPServerService{
		servers: []model.MCPServer{{
			ID: "srv1", Purpose: model.MCPServerPurposeModeration, Enabled: true,
		}},
		result: json.RawMessage(`{"action":"block","reason":"违规"}`),
	}
	svc := newMCPModerationFixture(mcp)

	action, err := svc.CheckExternal(context.Background(), "post", "p1", "t", "b", model.ModerationScenePost)
	require.NoError(t, err)
	assert.Equal(t, model.ModerationActionBlock, action)
	// 未指定工具名时用默认。
	assert.Equal(t, model.DefaultModerationTool, mcp.tool)
}

// TestCheckExternal_NotConfiguredFallsBack 未配置时应回退，且不算错误。
//
// 关键断言是 mcp.called == false：没配就不该去调。
func TestCheckExternal_NotConfiguredFallsBack(t *testing.T) {
	mcp := &stubMCPServerService{} // 没有任何服务
	svc := newMCPModerationFixture(mcp)

	// 没有 MCP 服务、也没有 HTTP 端点 → 回退后得到「不可用」。
	_, err := svc.CheckExternal(context.Background(), "post", "p1", "t", "b", model.ModerationScenePost)
	assert.ErrorIs(t, err, errExternalModerationUnavailable,
		"未配置任何外部审核时应回退到 HTTP 契约并报告不可用")
	assert.False(t, mcp.called, "没配 MCP 服务时不应发起调用")
}

// TestCheckExternal_MCPFailureFallsBack 配了但调用失败时必须回退。
//
// 这是最容易被写错的一条：失败后**不能**当成 allow 放行。
func TestCheckExternal_MCPFailureFallsBack(t *testing.T) {
	mcp := &stubMCPServerService{
		servers: []model.MCPServer{{
			ID: "srv1", Purpose: model.MCPServerPurposeModeration, Enabled: true,
		}},
		err: assert.AnError, // 模拟超时 / 网络失败
	}
	svc := newMCPModerationFixture(mcp)

	_, err := svc.CheckExternal(context.Background(), "post", "p1", "t", "b", model.ModerationScenePost)
	assert.ErrorIs(t, err, errExternalModerationUnavailable,
		"MCP 调用失败后必须回退，不得静默放行")
	assert.True(t, mcp.called, "确实尝试过 MCP")
}

// TestCheckExternal_MCPUnknownVerdictFallsBack 未知结论也必须回退。
func TestCheckExternal_MCPUnknownVerdictFallsBack(t *testing.T) {
	mcp := &stubMCPServerService{
		servers: []model.MCPServer{{
			ID: "srv1", Purpose: model.MCPServerPurposeModeration, Enabled: true,
		}},
		result: json.RawMessage(`{"content":[{"type":"text","text":"whatever"}]}`),
	}
	svc := newMCPModerationFixture(mcp)

	_, err := svc.CheckExternal(context.Background(), "post", "p1", "t", "b", model.ModerationScenePost)
	assert.ErrorIs(t, err, errExternalModerationUnavailable,
		"无法理解的结论应按不可用处理，交给异步规则兜底")
}
