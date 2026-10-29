package router

import (
	"time"

	"feedchamber/internal/middleware"
	"feedchamber/internal/model"

	"github.com/kataras/iris/v12"
)

// publicRoutes 无需登录的公开端点：健康检查以外的公共读接口，以及注册/登录、
// OAuth 消费者与自建 OAuth provider 的公开部分、SSO OIDC 的公开部分。
func (r *reg) publicRoutes() {
	v1, h, deps, gw := r.v1, r.h, r.deps, r.gw

	v1.Post("/settings/public", h.Setting.GetPublic)
	v1.Post("/themes/list", h.Theme.List)
	v1.Post("/leaderboard/list", h.Leaderboard.List)
	v1.Post("/badges/list", h.Badge.List)
	v1.Get("/upload/file/{key}", h.Upload.GetFileByKey)

	v1.PartyFunc("/auth", func(auth iris.Party) {
		auth.Post("/register",
			middleware.RateLimit(deps.Limiter, "register", 5, time.Minute),
			h.Auth.Register)
		auth.Post("/login",
			middleware.RateLimit(deps.Limiter, "login", 10, time.Minute),
			h.Auth.Login)
		auth.Post("/forgot_password",
			middleware.RateLimit(deps.Limiter, "forgot_password", 3, time.Minute),
			h.Auth.ForgotPassword)
		auth.Post("/reset_password_by_code",
			middleware.RateLimit(deps.Limiter, "reset_password_by_code", 5, time.Minute),
			h.Auth.ResetPasswordByCode)
		auth.Post("/mfa/verify",
			middleware.RateLimit(deps.Limiter, "mfa_verify", 20, time.Minute),
			h.MFA.VerifyLogin)
	})

	// ── OAuth (public, callback + provider endpoints) ─────────
	v1.PartyFunc("/oauth", func(oauth iris.Party) {
		// Consumer OAuth (external login flow)
		oauth.Get("/providers", h.OAuth.GetAllProviderConfigs)
		oauth.Get("/{provider}/auth", h.OAuth.GetAuthURL)
		oauth.Get("/{provider}/callback", h.OAuth.HandleCallback)
		oauth.Get("/{provider}/config", h.OAuth.GetProviderConfig)

		// OAuth provider (this server as OAuth 2.0 provider)
		// Token and revoke authenticate the client, not a user
		oauth.Post("/token", gw.Provider.Token)
		oauth.Post("/revoke", gw.Provider.Revoke)
	})

	// -- SSO OIDC (public endpoints) -----------------------------------------
	v1.PartyFunc("/sso", func(sso iris.Party) {
		sso.Post("/token", gw.SSO.Token)
		sso.Get("/userinfo", gw.SSO.UserInfo)
	})
}

// publicOptionalRoutes 公开但可选登录的读端点：可选会话解析已由 v1 层的 gw.Preflight()
// 覆盖，这里只用 AuthGate 按设置项决定是否要求登录。
func (r *reg) publicOptionalRoutes() {
	v1, h, deps := r.v1, r.h, r.deps

	v1.PartyFunc("/tags", func(t iris.Party) {
		t.Post("/list", middleware.AuthGate(deps, model.SettingTagListAuthRequired), h.Tag.List)
	})
	v1.PartyFunc("/spaces", func(s iris.Party) {
		s.Post("/tree", middleware.AuthGate(deps, model.SettingSpaceViewAuthRequired), h.Space.Tree)
		s.Post("/flat", middleware.AuthGate(deps, model.SettingSpaceViewAuthRequired), h.Space.Flat)
		s.Post("/roots", middleware.AuthGate(deps, model.SettingSpaceViewAuthRequired), h.Space.Roots)
		s.Post("/{space_id}/children", middleware.AuthGate(deps, model.SettingSpaceViewAuthRequired), h.Space.Children)
		s.Post("/{space_id}/ancestors", middleware.AuthGate(deps, model.SettingSpaceViewAuthRequired), h.Space.Ancestors)
		s.Post("/{space_id}/breadcrumb", middleware.AuthGate(deps, model.SettingSpaceViewAuthRequired), h.Space.Breadcrumb)
		s.Post("/{space_id}/descendants", middleware.AuthGate(deps, model.SettingSpaceViewAuthRequired), h.Space.Descendants)
		s.Post("/{space_id}/subtree", middleware.AuthGate(deps, model.SettingSpaceViewAuthRequired), h.Space.SubTree)
		s.Post("/{space_id}", middleware.AuthGate(deps, model.SettingSpaceViewAuthRequired), h.Space.Get)
	})
	// 等级名称是公开配置（前端展示用），不含任何管理字段。
	v1.Get("/levels", h.Trust.GetPublicLevels)
	v1.PartyFunc("/announcements", func(a iris.Party) {
		a.Post("/published", middleware.AuthGate(deps, model.SettingAnnouncementAuthRequired), h.Announcement.ListPublished)
		a.Get("/{id}", middleware.AuthGate(deps, model.SettingAnnouncementAuthRequired), h.Announcement.Get)
	})
}
