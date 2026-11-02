package router

import (
	"time"

	"feedchamber/internal/middleware"

	"github.com/kataras/iris/v12"
)

// messageRoutes 站内消息：私信会话与消息、通知列表与已读、通用 SSE 实时流。
func (r *reg) messageRoutes() {
	protected, h, deps := r.protected, r.h, r.deps

	// ── Notification ──────────────────────────────────────────

	// ── Private messages ─────────────────────────────────────
	protected.PartyFunc("/messages", func(m iris.Party) {
		m.Post("/conversations/list", h.PM.ListConversations)
		m.Post("/conversations/create", middleware.RateLimit(deps.Limiter, "pm_conversation_create", 20, time.Minute), h.PM.CreateConversation)
		m.Post("/conversations/{id}/messages/list", h.PM.ListMessages)
		m.Post("/conversations/{id}/messages/create", middleware.RateLimit(deps.Limiter, "pm_message_create", 60, time.Minute), h.PM.SendMessage)
		m.Post("/conversations/{id}/read", h.PM.MarkRead)
	})

	protected.PartyFunc("/notifications", func(n iris.Party) {
		n.Post("/list", h.Notification.List)
		n.Post("/unread_count", h.Notification.CountUnread)
		n.Post("/{id}/read", h.Notification.MarkRead)
		n.Post("/read_all", h.Notification.MarkAllRead)
		n.Post("/{id}/delete", h.Notification.Delete)
		n.Post("/stream", h.Notification.Stream)
	})

	// ── Realtime generic SSE ─────────────────────────────────
	protected.Post("/realtime/stream", h.Realtime.Stream)
}

// reportRoutes 举报。
func (r *reg) reportRoutes() {
	protected, h, deps := r.protected, r.h, r.deps

	// ── Report ────────────────────────────────────────────────
	protected.PartyFunc("/reports", func(r iris.Party) {
		r.Post("/create", middleware.RateLimit(deps.Limiter, "report_create", 10, time.Minute), h.Report.Create)
		r.Post("/list", h.Report.List)
		r.Get("/{id}", h.Report.Get)
		r.Post("/{id}/status", h.Report.UpdateStatus)
		r.Post("/{id}/delete", h.Report.Delete)
	})
}
