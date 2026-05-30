package router

import (
	"orinforum/internal/handler"
	"orinforum/internal/middleware"
	"orinforum/internal/model"

	"github.com/kataras/iris/v12"
)

func Register(app *iris.Application, h *handler.Handler) {
	v1 := app.Party("/api/v1")

	// ── Public ────────────────────────────────────────────────
	v1.Get("/health", handler.Health)

	v1.PartyFunc("/auth", func(auth iris.Party) {
		auth.Post("/register", h.Auth.Register)
		auth.Post("/login", h.Auth.Login)
		auth.Post("/refresh_token", h.Auth.GetRefreshToken)
		auth.Post("/reset_password", h.Auth.ResetPassword)
	})

	// ── Protected (require JWT) ───────────────────────────────
	protected := v1.Party("")
	protected.Use(middleware.JWTAuth(), middleware.BanCheck())

	protected.PartyFunc("/auth", func(auth iris.Party) {
		auth.Post("/logout", h.Auth.Logout)
		auth.Post("/refresh", h.Auth.RefreshToken)
		auth.Post("/change_password", h.Auth.ChangePassword)
		auth.Post("/first_change_password", h.Auth.FirstChangePassword)
	})

	protected.PartyFunc("/user", func(u iris.Party) {
		u.Get("/profile", h.User.GetProfile)
		u.Put("/profile", h.User.UpdateProfile)
		u.Get("/search", middleware.RequirePermission(model.PermUserRead), h.User.Search)
		u.Get("/list", middleware.RequirePermission(model.PermUserRead), h.User.List)
		u.Get("/{id}", middleware.RequirePermission(model.PermUserRead), h.User.GetByID)
		u.Delete("/{id}", middleware.RequirePermission(model.PermUserDelete), h.User.Delete)
		u.Put("/{id}/role", middleware.RequirePermission(model.PermUserUpdate), h.User.UpdateRole)
	})

	protected.PartyFunc("/role", func(r iris.Party) {
		r.Get("/list", middleware.RequirePermission(model.PermRoleRead), h.Role.List)
		r.Post("/create", middleware.RequirePermission(model.PermRoleManage), h.Role.Create)
		r.Delete("/{id}", middleware.RequirePermission(model.PermRoleManage), h.Role.Delete)
	})

	protected.PartyFunc("/permission", func(p iris.Party) {
		p.Get("/list", middleware.RequirePermission(model.PermRoleRead), h.Permission.ListPermissions)
		p.Get("/role", middleware.RequirePermission(model.PermRoleRead), h.Permission.GetRolePermissions)
		p.Post("/sync", middleware.RequirePermission(model.PermRoleManage), h.Permission.SyncRolePermissions)
	})

	protected.PartyFunc("/categories", func(cat iris.Party) {
		cat.Get("/", h.Category.List)
		cat.Post("/", middleware.RequirePermission(model.PermCategoryCreate), h.Category.Create)
		cat.Put("/{id}", middleware.RequirePermission(model.PermCategoryCreate), h.Category.Update)
		cat.Delete("/{id}", middleware.RequirePermission(model.PermCategoryDelete), h.Category.Delete)
	})

	protected.PartyFunc("/tags", func(t iris.Party) {
		t.Get("/", h.Tag.List)
		t.Post("/", middleware.RequirePermission(model.PermPostCreate), h.Tag.Create)
		t.Put("/{id}", middleware.RequirePermission(model.PermPostCreate), h.Tag.Update)
		t.Delete("/{id}", middleware.RequirePermission(model.PermPostDelete), h.Tag.Delete)
	})

	protected.PartyFunc("/posts", func(p iris.Party) {
		p.Get("/", h.Post.List)
		p.Get("/search", h.Post.Search)
		p.Get("/{id}", h.Post.GetByID)
		p.Post("/", middleware.RequirePermission(model.PermPostCreate), h.Post.Create)
		p.Put("/{id}", middleware.RequirePermission(model.PermPostUpdate), h.Post.Update)
		p.Delete("/{id}", middleware.RequirePermission(model.PermPostDelete), h.Post.Delete)

		p.Put("/{id}/pin", middleware.RequirePermission(model.PermPostPin), h.Post.TogglePin)
		p.Put("/{id}/lock", middleware.RequirePermission(model.PermPostLock), h.Post.ToggleLock)

		p.Get("/{postId}/tags", h.Tag.GetPostTags)
		p.Post("/{postId}/tags", middleware.RequirePermission(model.PermPostCreate), h.Tag.SetPostTags)

		p.PartyFunc("/{postId}/contents", func(c iris.Party) {
			c.Get("/", h.Content.ListByPost)
			c.Post("/", middleware.RequirePermission(model.PermCommentCreate), h.Content.Create)
			c.Put("/{id}", middleware.RequirePermission(model.PermCommentCreate), h.Content.Update)
			c.Delete("/{id}", middleware.RequirePermission(model.PermCommentDelete), h.Content.Delete)
		})

		p.Post("/{postId}/favorite", h.Favorite.Toggle)
		p.Get("/{postId}/favorite", h.Favorite.GetStatus)
	})

	protected.PartyFunc("/favorites", func(f iris.Party) {
		f.Get("/", h.Favorite.ListByUser)
	})

	protected.PartyFunc("/contents/{contentId}", func(c iris.Party) {
		c.Post("/like", h.Like.ToggleContentLike)
		c.Get("/like", h.Like.GetContentLikeStatus)
		c.Get("/replies", h.Content.ListReplies)
	})

	protected.PartyFunc("/users/{userId}", func(u iris.Party) {
		u.Post("/follow", h.Follow.Follow)
		u.Delete("/follow", h.Follow.Unfollow)
		u.Get("/follow", h.Follow.GetStatus)
		u.Get("/followers", h.Follow.GetFollowers)
		u.Get("/following", h.Follow.GetFollowing)
		u.Get("/counts", h.Follow.GetCounts)
	})

	protected.PartyFunc("/upload", func(u iris.Party) {
		u.Post("/", h.Upload.Upload)
		u.Get("/", h.Upload.List)
		u.Get("/{id}", h.Upload.GetFile)
		u.Delete("/{id}", h.Upload.Delete)
	})

	protected.PartyFunc("/notifications", func(n iris.Party) {
		n.Get("/", h.Notification.List)
		n.Get("/unread_count", h.Notification.CountUnread)
		n.Put("/{id}/read", h.Notification.MarkRead)
		n.Put("/read_all", h.Notification.MarkAllRead)
		n.Delete("/{id}", h.Notification.Delete)
	})

	// ── Reports ────────────────────────────────────────────────
	protected.PartyFunc("/reports", func(r iris.Party) {
		r.Post("/", middleware.RequirePermission(model.PermReportCreate), h.Report.Create)
		r.Get("/", middleware.RequirePermission(model.PermReportManage), h.Report.List)
		r.Put("/{id}/status", middleware.RequirePermission(model.PermReportManage), h.Report.UpdateStatus)
		r.Delete("/{id}", middleware.RequirePermission(model.PermReportManage), h.Report.Delete)
	})

	// ── Admin ───────────────────────────────────────────────────
	protected.PartyFunc("/admin", func(a iris.Party) {
		a.Use(middleware.RequirePermission(model.PermRoleManage))
		a.Get("/stats", h.Admin.GetStats)
		a.Get("/settings", h.Setting.GetAll)
		a.Get("/settings/{key}", h.Setting.Get)
		a.Put("/settings/{key}", h.Setting.Set)
		a.Delete("/settings/{key}", h.Setting.Delete)
	})
}
