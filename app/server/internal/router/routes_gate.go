package router

import (
	"orinforum/internal/middleware"
	"orinforum/internal/model"

	"github.com/kataras/iris/v12"
)

// gateRoutes 是门控规则引擎的后台配置面，整段挂 gate:manage。
func (r *reg) gateRoutes() {
	protected, h, deps := r.protected, r.h, r.deps

	protected.PartyFunc("/admin/gates", func(a iris.Party) {
		a.Use(middleware.RequirePermission(deps, model.PermGateManage))
		a.Post("/meta", h.Gate.Meta)
		a.Post("/list", h.Gate.ListRules)
		a.Post("/save", h.Gate.SaveRule)
		a.Post("/{id}/delete", h.Gate.DeleteRule)
		a.Post("/defaults/list", h.Gate.ListDefaults)
		a.Post("/defaults/save", h.Gate.SaveDefault)
		a.Post("/simulate", h.Gate.Simulate)
	})
}
