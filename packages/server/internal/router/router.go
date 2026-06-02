package router

import (
	"orinforum/internal/handler"
	"orinforum/internal/middleware"
	"orinforum/internal/model"

	"github.com/kataras/iris/v12"
)

func Register(app *iris.Application, h *handler.Handler) {
	v1 := app.Party("/api/v1")

	// ── Public (no JWT required) ──────────────────────────────
	v1.Get("/health", handler.Health)
	v1.Post("/settings/public", h.Setting.GetPublic)

	v1.PartyFunc("/auth", func(auth iris.Party) {
		auth.Post("/register", h.Auth.Register)
		auth.Post("/login", h.Auth.Login)
		auth.Post("/refresh_token", h.Auth.GetRefreshToken)
		auth.Post("/reset_password", h.Auth.ResetPassword)
	})

	// Public category and tag lists
	v1.Post("/categories/list", h.Category.List)
	v1.Post("/tags/list", h.Tag.List)

	// ── Protected (require JWT) ───────────────────────────────
	protected := v1.Party("")
	protected.Use(middleware.JWTAuth(), middleware.BanCheck())

	protected.PartyFunc("/auth", func(auth iris.Party) {
		auth.Post("/logout", h.Auth.Logout)
		auth.Post("/refresh", h.Auth.RefreshToken)
		auth.Post("/change_password", h.Auth.ChangePassword)
		auth.Post("/first_change_password", h.Auth.FirstChangePassword)
	})

	// ── User ──────────────────────────────────────────────────
	protected.PartyFunc("/user", func(u iris.Party) {
		u.Get("/profile", h.User.GetProfile)
		u.Post("/profile/update", h.User.UpdateProfile)
		u.Post("/search", middleware.RequirePermission(model.PermUserRead), h.User.Search)
		u.Post("/list", middleware.RequirePermission(model.PermUserRead), h.User.List)
		u.Get("/{id}", middleware.RequirePermission(model.PermUserRead), h.User.GetByID)
		u.Post("/{id}/delete", middleware.RequirePermission(model.PermUserDelete), h.User.Delete)
		u.Post("/{id}/role/update", middleware.RequirePermission(model.PermUserUpdate), h.User.UpdateRole)
	})

	// ── Role ──────────────────────────────────────────────────
	protected.PartyFunc("/role", func(r iris.Party) {
		r.Post("/list", middleware.RequirePermission(model.PermRoleRead), h.Role.List)
		r.Post("/create", middleware.RequirePermission(model.PermRoleManage), h.Role.Create)
		r.Post("/{id}/delete", middleware.RequirePermission(model.PermRoleManage), h.Role.Delete)
	})

	// ── Permission ────────────────────────────────────────────
	protected.PartyFunc("/permission", func(p iris.Party) {
		p.Post("/list", middleware.RequirePermission(model.PermRoleRead), h.Permission.ListPermissions)
		p.Post("/role", middleware.RequirePermission(model.PermRoleRead), h.Permission.GetRolePermissions)
		p.Post("/sync", middleware.RequirePermission(model.PermRoleManage), h.Permission.SyncRolePermissions)
	})

	// ── Category ──────────────────────────────────────────────
	protected.PartyFunc("/categories", func(cat iris.Party) {
		cat.Post("/create", middleware.RequirePermission(model.PermCategoryCreate), h.Category.Create)
		cat.Post("/{id}/update", middleware.RequirePermission(model.PermCategoryCreate), h.Category.Update)
		cat.Post("/{id}/delete", middleware.RequirePermission(model.PermCategoryDelete), h.Category.Delete)
	})

	// ── Tag ───────────────────────────────────────────────────
	protected.PartyFunc("/tags", func(t iris.Party) {
		t.Post("/create", middleware.RequirePermission(model.PermPostCreate), h.Tag.Create)
		t.Post("/{id}/update", middleware.RequirePermission(model.PermPostCreate), h.Tag.Update)
		t.Post("/{id}/delete", middleware.RequirePermission(model.PermPostDelete), h.Tag.Delete)
	})

	// ── Post ──────────────────────────────────────────────────
	// list/search/get are public by default; gates enforce auth when setting is enabled
	v1.PartyFunc("/posts", func(p iris.Party) {
		p.Use(middleware.OptionalJWTAuth())
		p.Post("/list", middleware.PostAuthGate(), h.Post.List)
		p.Post("/search", middleware.PostAuthGate(), h.Post.Search)
		p.Get("/{id}", middleware.PostAuthGate(), h.Post.GetByID)

		p.Post("/{postId}/contents/list", middleware.CommentAuthGate(), h.Content.ListByPost)
		p.Post("/{postId}/tags", h.Tag.GetPostTags)
	})

	protected.PartyFunc("/posts", func(p iris.Party) {
		p.Post("/create", middleware.RequirePermission(model.PermPostCreate), h.Post.Create)
		p.Post("/{id}/update", middleware.RequirePermission(model.PermPostUpdate), h.Post.Update)
		p.Post("/{id}/delete", middleware.RequirePermission(model.PermPostDelete), h.Post.Delete)
		p.Post("/{id}/pin", middleware.RequirePermission(model.PermPostPin), h.Post.TogglePin)
		p.Post("/{id}/lock", middleware.RequirePermission(model.PermPostLock), h.Post.ToggleLock)

		p.Post("/{postId}/tags/set", middleware.RequirePermission(model.PermPostCreate), h.Tag.SetPostTags)

		p.Post("/{postId}/contents/create", middleware.RequirePermission(model.PermCommentCreate), h.Content.Create)
		p.Post("/{postId}/contents/{id}/update", middleware.RequirePermission(model.PermCommentCreate), h.Content.Update)
		p.Post("/{postId}/contents/{id}/delete", middleware.RequirePermission(model.PermCommentDelete), h.Content.Delete)

		p.Post("/{postId}/favorite", h.Favorite.Toggle)
		p.Post("/{postId}/favorite/status", h.Favorite.GetStatus)
	})

	// ── Favorites list ────────────────────────────────────────
	protected.PartyFunc("/favorites", func(f iris.Party) {
		f.Post("/list", h.Favorite.ListByUser)
	})

	// ── Content standalone ────────────────────────────────────
	protected.PartyFunc("/contents/{contentId}", func(c iris.Party) {
		c.Post("/like", h.Like.ToggleContentLike)
		c.Post("/like/status", h.Like.GetContentLikeStatus)
		c.Post("/replies", h.Content.ListReplies)
	})

	// ── Follow ────────────────────────────────────────────────
	protected.PartyFunc("/users/{userId}", func(u iris.Party) {
		u.Post("/follow", h.Follow.Follow)
		u.Post("/unfollow", h.Follow.Unfollow)
		u.Post("/follow/status", h.Follow.GetStatus)
		u.Post("/followers", h.Follow.GetFollowers)
		u.Post("/following", h.Follow.GetFollowing)
		u.Post("/counts", h.Follow.GetCounts)
	})

	// ── Upload ────────────────────────────────────────────────
	protected.PartyFunc("/upload", func(u iris.Party) {
		u.Post("/", h.Upload.Upload)
		u.Post("/presign", h.Upload.Presign)
		u.Post("/confirm", h.Upload.Confirm)
		u.Post("/list", h.Upload.List)
		u.Get("/{id}", h.Upload.GetFile)
		u.Post("/{id}/delete", h.Upload.Delete)
	})

	// ── Notification ──────────────────────────────────────────
	protected.PartyFunc("/notifications", func(n iris.Party) {
		n.Post("/list", h.Notification.List)
		n.Post("/unread_count", h.Notification.CountUnread)
		n.Post("/{id}/read", h.Notification.MarkRead)
		n.Post("/read_all", h.Notification.MarkAllRead)
		n.Post("/{id}/delete", h.Notification.Delete)
	})

	// ── Report ────────────────────────────────────────────────
	protected.PartyFunc("/reports", func(r iris.Party) {
		r.Post("/create", middleware.RequirePermission(model.PermReportCreate), h.Report.Create)
		r.Post("/list", middleware.RequirePermission(model.PermReportManage), h.Report.List)
		r.Post("/{id}/status", middleware.RequirePermission(model.PermReportManage), h.Report.UpdateStatus)
		r.Post("/{id}/delete", middleware.RequirePermission(model.PermReportManage), h.Report.Delete)
	})

	// ── Admin ─────────────────────────────────────────────────
	protected.PartyFunc("/admin", func(a iris.Party) {
		a.Use(middleware.RequirePermission(model.PermRoleManage))
		a.Post("/stats", h.Admin.GetStats)
		a.Post("/settings", h.Setting.GetAll)
		a.Post("/settings/batch", h.Setting.BatchSet)
		a.Post("/settings/{key}", h.Setting.Get)
		a.Post("/settings/{key}/update", h.Setting.Set)
		a.Post("/settings/{key}/delete", h.Setting.Delete)
	})
}
