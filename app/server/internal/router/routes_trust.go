package router

import (
	"feedchamber/internal/middleware"
	"feedchamber/internal/model"

	"github.com/kataras/iris/v12"
)

// trustRoutes 是 XP / 等级体系的后台配置面（等级、XP 规则、连续签到阶梯、
// 人工调整、全量重算）。全部挂 level:manage。
func (r *reg) trustRoutes() {
	protected, h, deps := r.protected, r.h, r.deps

	protected.PartyFunc("/admin", func(a iris.Party) {
		a.Use(middleware.RequirePermission(deps, model.PermLevelManage))

		// 等级定义
		a.Post("/levels/list", h.Trust.ListLevels)
		a.Post("/levels/save", h.Trust.SaveLevel)
		a.Post("/levels/{level}/delete", h.Trust.DeleteLevel)

		// XP 规则与连续签到阶梯
		a.Post("/xp-rules/list", h.Trust.ListXPRules)
		a.Post("/xp-rules/save", h.Trust.SaveXPRule)
		a.Post("/xp-rules/streak/list", h.Trust.ListStreakBonuses)
		a.Post("/xp-rules/streak/save", h.Trust.SaveStreakBonus)
		a.Post("/xp-rules/streak/{days}/delete", h.Trust.DeleteStreakBonus)

		// 全量重算（迁移/对账）
		a.Post("/trust/recompute", h.Trust.RecomputeAll)

		// 用户 XP 详情与人工调整
		a.Get("/users/{id}/xp", h.Trust.GetUserXP)
		a.Post("/users/{id}/xp/adjust", h.Trust.AdjustXP)
		a.Post("/users/{id}/xp/adjustments/list", h.Trust.ListAdjustments)
		a.Post("/xp-adjustments/{id}/delete", h.Trust.DeleteAdjustment)
	})
}
