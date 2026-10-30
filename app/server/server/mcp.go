package server

import (
	"context"
	"errors"

	"feedchamber/internal/container"
	"feedchamber/internal/mcp"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/ratelimit"
	"feedchamber/internal/service"

	"github.com/sirupsen/logrus"
)

// mcpAuditAdapter 把 MCP 的工具调用写进 audit_logs。
//
// action 前缀 mcp.tool. 让审计页可以按「MCP 调用」一栏筛出来；detail 里带
// 成功/失败与结果摘要（见 mcp-server 规范「MCP 调用必须留审计」）。
type mcpAuditAdapter struct{ svc service.AuditLogService }

func (a mcpAuditAdapter) RecordToolCall(ctx context.Context, actorID, tool string, ok bool, detail string) error {
	status := "ok"
	if !ok {
		status = "failed"
	}
	return a.svc.Record(ctx, actorID, mcp.AuditActionPrefix+tool, tool, status+": "+detail)
}

// buildMCP 装配站点对外暴露的 MCP Server。
//
// 放在 composition root 而不是 container 里，是因为 Enforcer 需要的
// SettingReader / PermissionChecker / BannedChecker / 限流引擎都是
// 「网关与中间件共用同一份实例」的组件，它们在这里才齐全。
func buildMCP(c *container.Container, settings service.SettingService, perms pkg.PermissionChecker,
	banned pkg.BannedChecker, limiter *ratelimit.Limiter) container.MCPComponents {

	var lookup mcp.APIKeyLookup
	if sqlStore := c.Store.SQL(); sqlStore != nil {
		lookup = mcp.NewGORMLookup(sqlStore.DB())
	}

	enforcer := mcp.NewEnforcer(mcp.EnforcerDeps{
		Settings: settings,
		Perms:    perms,
		Banned:   banned,
		Limiter:  limiter,
		Lookup:   lookup,
	})

	registry := mcp.NewRegistry()
	mcp.RegisterAll(registry, mcp.ToolsDeps{
		Posts:     c.PostSvc,
		Contents:  c.ContentSvc,
		Likes:     c.LikeSvc,
		Favorites: c.FavoriteSvc,
		Spaces:    c.SpaceSvc,
		Tags:      c.TagSvc,
		Users:     c.UserSvc,
		Customize: c.CustomizeSvc,
	})

	// 工具启停从设置同步：mcp_disabled_tools 存的是「相对默认的覆盖」
	// （写工具默认关，负号前缀表示显式开启）。
	if settings != nil {
		if raw, err := settings.Get(context.Background(), model.SettingMCPDisabledTools); err == nil {
			registry.ApplyDisabledList(raw)
		} else {
			// 读不到设置也要落到安全默认：写工具全关。
			registry.ApplyDisabledList("")
		}
	} else {
		registry.ApplyDisabledList("")
	}

	audit := mcpAuditAdapter{svc: c.AuditLogSvc}
	srv := mcp.NewServer(enforcer, registry, audit)
	if settings != nil {
		if raw, err := settings.Get(context.Background(), model.SettingMCPMaxResultBytes); err == nil {
			srv.SetMaxResultBytes(atoiSafe(raw))
		}
	}

	logrus.WithField("tools", len(registry.All())).Info("mcp: server assembled")
	h := mcp.NewHandler(srv)
	// 工具启停落库到设置 mcp_disabled_tools：与启动时的读取形成闭环，
	// 否则后台开关只在当前进程有效（重启回默认、多实例各一份）。
	h.SetToolStateStore(mcpToolStateStore{settings: settings})
	return container.MCPComponents{
		Handler:  h,
		Registry: registry,
	}
}

// atoiSafe 解析整数设置，失败返回 0（调用方按「不覆盖默认」处理）。
// mcpToolStateStore 把工具启停名单写进系统设置。
//
// 编码（负号 = 显式启用写工具）由 mcp.Registry 决定，这一层只负责搬运。
type mcpToolStateStore struct {
	settings service.SettingService
}

func (s mcpToolStateStore) SaveToolState(ctx context.Context, raw string) error {
	if s.settings == nil {
		return errors.New("setting service unavailable")
	}
	return s.settings.Set(ctx, model.SettingMCPDisabledTools, raw)
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
		if n > 1<<20 {
			return 0
		}
	}
	return n
}
