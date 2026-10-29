package router

import (
	"orinforum/internal/middleware"
	"orinforum/internal/model"

	"github.com/kataras/iris/v12"
)

// accessRoutes 权限面：角色、权限（含版面级权限）、用户组。
func (r *reg) accessRoutes() {
	protected, h, deps := r.protected, r.h, r.deps

	// ── Role ──────────────────────────────────────────────────
	protected.PartyFunc("/role", func(r iris.Party) {
		r.Post("/list", h.Role.List)
		r.Post("/create", h.Role.Create)
		r.Post("/{id}/update", h.Role.Update)
		r.Post("/{id}/delete", h.Role.Delete)
	})

	// ── Permission ────────────────────────────────────────────
	protected.PartyFunc("/permission", func(p iris.Party) {
		p.Post("/list", middleware.RequirePermission(deps, model.PermRoleRead), h.Permission.ListPermissions)
		p.Post("/role", middleware.RequirePermission(deps, model.PermRoleRead), h.Permission.GetRolePermissions)
		p.Post("/sync", middleware.RequirePermission(deps, model.PermRoleManage), h.Permission.SyncRolePermissions)
		p.Post("/space", middleware.RequirePermission(deps, model.PermRoleRead), h.Permission.GetSpaceRolePermissions)
		p.Post("/space/effective", middleware.RequirePermission(deps, model.PermRoleRead), h.Permission.GetEffectiveSpaceRolePermissions)
		p.Post("/space/tree", middleware.RequirePermission(deps, model.PermRoleRead), h.Permission.GetSpacePermissionsTree)
		p.Post("/space/sync", middleware.RequirePermission(deps, model.PermRoleManage), h.Permission.SyncSpaceRolePermissions)
		p.Post("/space/clear", middleware.RequirePermission(deps, model.PermRoleManage), h.Permission.ClearSpaceRolePermissions)
	})

	// ── Groups ────────────────────────────────────────────────
	protected.PartyFunc("/groups", func(g iris.Party) {
		g.Post("/list", h.Group.List)
		g.Post("/create", h.Group.Create)
		g.Post("/{id}/update", h.Group.Update)
		g.Post("/{id}/delete", h.Group.Delete)
		g.Post("/{id}/members/list", h.Group.ListMembers)
		g.Post("/{id}/members/add", h.Group.AddMember)
		g.Post("/{id}/members/remove", h.Group.RemoveMember)
		g.Post("/{id}/permissions", h.Group.GetPermissions)
		g.Post("/{id}/permissions/sync", h.Group.SyncPermissions)
	})
}
