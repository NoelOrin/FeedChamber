package router

import (
	"github.com/kataras/iris/v12"
)

// historyRoutes 我的浏览记录：列表、删单条、清空。
//
// 新域（2026-11-09，openspec change 2026-11-09-browse-history-read-position）。
// 三条都是 protected，网关按 path→权限码映射放行（`favorite:self` 一族，与
// watches/favorites 同档「自助数据」）；访问者身份取自会话，路径与 body 都
// 不接受别人的 user id。
//
// 顺序是硬约束：固定段 `/list`、`/clear` 必须排在参数段 `/{postId}/delete` 之前。
func (r *reg) historyRoutes() {
	protected, h := r.protected, r.h

	protected.PartyFunc("/history", func(p iris.Party) {
		p.Post("/list", h.History.List)
		p.Post("/clear", h.History.Clear)
		p.Post("/{postId}/delete", h.History.Delete)
	})
}
