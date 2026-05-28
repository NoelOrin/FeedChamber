package router

import (
	"orinforum/internal/handler"
	"orinforum/internal/middleware"

	"github.com/kataras/iris/v12"
)

func Register(app *iris.Application, h *handler.Handler) {
	v1 := app.Party("/api/v1")

	// ── Public ────────────────────────────────────────────────
	v1.Get("/health", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"status": "ok"})
	})

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
		u.Get("/list", h.User.List)
		u.Get("/{id}", h.User.GetByID)
		u.Delete("/{id}", h.User.Delete)
		u.Put("/{id}/role", middleware.RequireRole("admin"), h.User.UpdateRole)
	})

	protected.PartyFunc("/role", func(r iris.Party) {
		r.Get("/list", h.Role.List)
		r.Post("/create", middleware.RequireRole("admin"), h.Role.Create)
		r.Delete("/{id}", middleware.RequireRole("admin"), h.Role.Delete)
	})

	protected.PartyFunc("/categories", func(cat iris.Party) {
		cat.Get("/", h.Category.List)
		cat.Post("/", h.Category.Create)
		cat.Delete("/{id}", h.Category.Delete)
	})

	protected.PartyFunc("/posts", func(p iris.Party) {
		p.Get("/", h.Post.List)
		p.Get("/{id}", h.Post.GetByID)
		p.Post("/", h.Post.Create)
		p.Put("/{id}", h.Post.Update)
		p.Delete("/{id}", h.Post.Delete)

		p.PartyFunc("/{postId}/comments", func(c iris.Party) {
			c.Get("/", h.Comment.ListByPost)
			c.Post("/", h.Comment.Create)
			c.Delete("/{id}", h.Comment.Delete)
		})
	})
}