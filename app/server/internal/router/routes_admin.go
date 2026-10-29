package router

import (
	"feedchamber/internal/middleware"
	"feedchamber/internal/model"

	"github.com/kataras/iris/v12"
)

// spaceManageRoutes 版面的写操作（版面树的公开读在 publicOptionalRoutes 里）。
func (r *reg) spaceManageRoutes() {
	protected, h := r.protected, r.h

	// ── Space ─────────────────────────────────────────────────
	protected.PartyFunc("/spaces", func(s iris.Party) {
		s.Post("/create", h.Space.Create)
		s.Post("/reorder", h.Space.Reorder)
		s.Post("/{space_id}/update", h.Space.Update)
		s.Post("/{space_id}/delete", h.Space.Delete)
		s.Post("/{space_id}/status", h.Space.ToggleActive)
		s.Post("/{space_id}/move", h.Space.Move)
	})
}

// adminRoutes 管理后台：邀请、注册审批、公告、设置、禁言列表、备份、插件、回收站、审核、webhook、统计与检索、批量操作、审计日志、
// 计数器对账。
func (r *reg) adminRoutes() {
	protected, h, deps := r.protected, r.h, r.deps

	// ── Admin ─────────────────────────────────────────────────
	protected.PartyFunc("/admin", func(a iris.Party) {
		// ── Invites ────────────────────────────────────────────
		a.PartyFunc("/invites", func(ai iris.Party) {
			ai.Post("/list", h.Invite.List)
			ai.Post("/create", h.Invite.Create)
			ai.Post("/{id}/delete", h.Invite.Delete)
		})

		// ── Registration approval ─────────────────────────────
		a.Post("/users/{id}/approve_registration", h.Auth.ApproveRegistration)

		// ── Announcements ──────────────────────────────────────
		a.PartyFunc("/announcements", func(aa iris.Party) {
			aa.Post("/list", h.Announcement.List)
			aa.Post("/create", h.Announcement.Create)
			aa.Post("/{id}/update", h.Announcement.Update)
			aa.Post("/{id}/delete", h.Announcement.Delete)
			aa.Post("/{id}/publish", h.Announcement.Publish)
			aa.Post("/{id}/unpublish", h.Announcement.Unpublish)
		})

		// ── Settings ──────────────────────────────────────────
		a.PartyFunc("/settings", func(as iris.Party) {
			as.Post("", h.Setting.GetAll)
			as.Post("/batch", h.Setting.BatchSet)
			as.Post("/{key}", h.Setting.Get)
			as.Post("/{key}/update", h.Setting.Set)
			as.Post("/{key}/delete", h.Setting.Delete)
		})

		// ── Banned ────────────────────────────────────────────
		a.Post("/banned/list", h.Banned.List)

		// ── Backups ────────────────────────────────────────────
		a.PartyFunc("/backups", func(b iris.Party) {
			b.Use(middleware.RequirePermission(deps, model.PermBackupManage))
			b.Post("/create", h.Backup.Create)
			b.Post("/list", h.Backup.List)
			b.Post("/cleanup", h.Backup.Cleanup)
			b.Post("/{id}/download", h.Backup.Download)
			b.Post("/{id}/restore", h.Backup.Restore)
			b.Post("/{id}/delete", h.Backup.Delete)
		})

		// ── Plugins (declarative metadata only) ─────────────────
		a.PartyFunc("/plugins", func(p iris.Party) {
			p.Post("/list", h.Plugin.List)
			p.Post("/install", h.Plugin.Install)
			p.Post("/{id}/update", h.Plugin.Update)
			p.Post("/{id}/status", h.Plugin.SetEnabled)
			p.Post("/{id}/delete", h.Plugin.Delete)
		})

		// ── Trash / Restore ────────────────────────────────────
		a.PartyFunc("/trash", func(t iris.Party) {
			t.Post("/posts/list", h.Trash.ListPosts)
			t.Post("/posts/{id}/restore", h.Trash.RestorePost)
			t.Post("/comments/list", h.Trash.ListComments)
			t.Post("/comments/{id}/restore", h.Trash.RestoreComment)
		})

		// ── Moderation ────────────────────────────────────────
		a.PartyFunc("/moderation", func(am iris.Party) {
			am.Post("/pending", h.Moderation.ListPending)
			am.Post("/{id}/approve", h.Moderation.Approve)
			am.Post("/{id}/reject", h.Moderation.Reject)
			am.Post("/rules/create", h.Moderation.CreateRule)
			am.Post("/rules/update", h.Moderation.UpdateRule)
			am.Post("/rules/delete", h.Moderation.DeleteRule)
			am.Post("/rules/list", h.Moderation.ListRules)
			am.Post("/configs", h.Moderation.GetConfigs)
			am.Post("/configs/set", h.Moderation.SetConfig)
		})

		// ── Webhooks ──────────────────────────────────────────
		a.PartyFunc("/webhooks", func(w iris.Party) {
			w.Use(middleware.RequirePermission(deps, model.PermRoleManage))
			w.Post("/create", h.Webhook.Create)
			w.Post("/list", h.Webhook.List)
			w.Post("/{id}/update", h.Webhook.Update)
			w.Post("/{id}/delete", h.Webhook.Delete)
			w.Post("/{id}/deliveries/list", h.Webhook.ListDeliveries)
			w.Post("/{id}/ping", h.Webhook.Ping)
			w.Post("/{id}/deliveries/{deliveryId}/retry", h.Webhook.RetryDelivery)
		})

		// ── Site Customization（站点自定义层，P1） ─────────────
		// 读用 customize:read，写用 customize:write。发布用 customize:publish。
		a.PartyFunc("/customization", func(ac iris.Party) {
			// 片段
			ac.Post("/snippets/list", h.Customize.ListSnippets)
			ac.Post("/snippets/create", h.Customize.CreateSnippet)
			ac.Post("/snippets/{id}/update", h.Customize.UpdateSnippet)
			ac.Post("/snippets/{id}/delete", h.Customize.DeleteSnippet)
			ac.Post("/snippets/{id}/toggle", h.Customize.ToggleSnippet)
			// 主题 token
			ac.Post("/tokens/list", h.Customize.ListThemeTokens)
			ac.Post("/tokens/set", h.Customize.SetThemeTokens)
			ac.Post("/tokens/clear", h.Customize.ClearThemeTokens)
			ac.Post("/tokens/{token}/delete", h.Customize.DeleteThemeToken)
			// 导航
			ac.Post("/nav/list", h.Customize.ListNavItems)
			ac.Post("/nav/create", h.Customize.CreateNavItem)
			ac.Post("/nav/{id}/update", h.Customize.UpdateNavItem)
			ac.Post("/nav/{id}/delete", h.Customize.DeleteNavItem)
			// 首页模块
			ac.Post("/modules/list", h.Customize.ListHomeModules)
			ac.Post("/modules/create", h.Customize.CreateHomeModule)
			ac.Post("/modules/{id}/update", h.Customize.UpdateHomeModule)
			ac.Post("/modules/{id}/delete", h.Customize.DeleteHomeModule)
			// 自定义页
			ac.Post("/pages/list", h.Customize.ListCustomPages)
			ac.Post("/pages/create", h.Customize.CreateCustomPage)
			ac.Post("/pages/{id}/update", h.Customize.UpdateCustomPage)
			ac.Post("/pages/{id}/delete", h.Customize.DeleteCustomPage)
			ac.Post("/pages/{id}/publish", h.Customize.PublishCustomPage)
		})

		// ── 本站 MCP Server 工具管理（P2） ─────────────────────
		a.PartyFunc("/mcp", func(am iris.Party) {
			am.Use(middleware.RequirePermission(deps, model.PermSettingManage))
			am.Post("/tools", h.MCPServer.ListTools)
			am.Post("/tools/status", h.MCPServer.SetToolEnabled)
		})

		// ── External MCP Servers（P3） ─────────────────────────
		a.PartyFunc("/mcp-servers", func(am iris.Party) {
			am.Use(middleware.RequirePermission(deps, model.PermSettingManage))
			am.Post("/list", h.MCPServer.List)
			am.Post("/create", h.MCPServer.Create)
			am.Post("/{id}/update", h.MCPServer.Update)
			am.Post("/{id}/delete", h.MCPServer.Delete)
			am.Post("/{id}/test", h.MCPServer.Test)
		})

		// ── Stats / Dashboard ─────────────────────────────────
		a.Post("/stats", h.Admin.GetStats)
		a.Post("/dashboard", h.Admin.Dashboard)

		// ── Search & User Activities ──────────────────────────
		a.Post("/search", h.Admin.Search)
		a.Post("/user/{id}/activities", h.Admin.GetUserActivities)

		// ── Batch Operations ──────────────────────────────────
		a.PartyFunc("/batch", func(ab iris.Party) {
			ab.Post("/users", h.Admin.BatchUsers)
			ab.Post("/posts", h.Admin.BatchPosts)
			ab.Post("/comments", h.Admin.BatchComments)
		})

		// ── Audit Logs ────────────────────────────────────────
		a.Post("/audit_logs/list", h.Admin.ListAuditLogs)

		// ── Reconcile ─────────────────────────────────────────
		a.Post("/reconcile/counters", h.Admin.ReconcileCounters)
	})
}
