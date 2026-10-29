package router

import (
	"time"

	"orinforum/internal/middleware"

	"github.com/kataras/iris/v12"
)

// motionRoutes 社区自治动议。
//
// 与 reportRoutes 同构并存但完全独立：举报=违规上行→管理员；动议=灰区→社区集体裁判。
// 权限门在网关 path→permission 映射（motion:create / motion:manage），
// 撤回的发起者校验在 service 层（router 只给 motion:manage，避免越权枚举）。
func (r *reg) motionRoutes() {
	protected, h, deps := r.protected, r.h, r.deps

	// ── Motion ────────────────────────────────────────────────
	protected.PartyFunc("/motions", func(m iris.Party) {
		m.Post("/create", middleware.RateLimit(deps.Limiter, "motion_create", 10, time.Minute), h.Motion.Create)
		m.Post("/list", h.Motion.List)
		m.Get("/{id}", h.Motion.Get)
		m.Post("/{id}/vote", middleware.RateLimit(deps.Limiter, "motion_vote", 30, time.Minute), h.Motion.Vote)
		m.Post("/{id}/resolve", h.Motion.Resolve)
		m.Post("/{id}/status", h.Motion.UpdateStatus)
		m.Post("/{id}/delete", h.Motion.Delete)
	})
}
