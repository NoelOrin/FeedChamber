package router

import (
	"time"

	"orinforum/internal/middleware"

	"github.com/kataras/iris/v12"
)

// dashboardRoutes protected 面上的第一批路由：首页聚合与用户徽章列表。
func (r *reg) dashboardRoutes() {
	protected, h := r.protected, r.h

	// 指标已迁到 METRICS_PORT 独立内部端口（server/iris.go），
	// 不再作为业务路由暴露，避免抓取端需要 admin 令牌。

	// ── Dashboard (首页聚合) ──────────────────────────────────
	protected.Post("/dashboard", h.Home.Dashboard)

	protected.Post("/user/{id}/badges/list", h.Badge.ListByUser)
}

// protectedAuthRoutes 登录态下的身份与会话端点：登出、改密、邮箱验证、MFA，
// OAuth 账号绑定与 provider 客户端管理，SSO 授权页。
func (r *reg) protectedAuthRoutes() {
	protected, h, deps, gw := r.protected, r.h, r.deps, r.gw

	protected.PartyFunc("/auth", func(auth iris.Party) {
		auth.Post("/logout", gw.Session.LogoutHandler)
		auth.Post("/refresh", h.Auth.RefreshToken)
		auth.Post("/send_verify_email",
			middleware.RateLimit(deps.Limiter, "send_verify_email", 3, time.Minute),
			h.Auth.SendVerifyEmail)
		auth.Post("/verify_email",
			middleware.RateLimit(deps.Limiter, "verify_email", 10, time.Minute),
			h.Auth.VerifyEmail)
		auth.Post("/change_password", h.Auth.ChangePassword)
		auth.Post("/first_change_password", h.Auth.FirstChangePassword)
		auth.Post("/reset_password", h.Auth.ResetPassword)
		auth.Post("/mfa/totp/setup", h.MFA.SetupTOTP)
		auth.Post("/mfa/totp/enable", h.MFA.EnableTOTP)
		auth.Post("/mfa/totp/disable", h.MFA.DisableTOTP)
		auth.Post("/mfa/backup/regenerate", h.MFA.RegenerateBackup)
	})

	// ── OAuth (protected, account management + provider) ──────
	protected.PartyFunc("/oauth", func(oauth iris.Party) {
		// Consumer OAuth: linked account management
		oauth.Get("/accounts", h.OAuth.GetLinkedAccounts)
		oauth.Post("/{provider}/link", h.OAuth.LinkAccount)
		oauth.Post("/{provider}/unlink", h.OAuth.UnlinkAccount)

		// OAuth provider: client registration (admin) + authorize
		oauth.Post("/clients", gw.Provider.RegisterClient)
		oauth.Post("/clients/list", gw.Provider.ListClients)
		oauth.Post("/clients/{id}/update", gw.Provider.UpdateClient)
		oauth.Post("/clients/{id}/reset-secret", gw.Provider.ResetClientSecret)
		oauth.Post("/clients/{id}/delete", gw.Provider.DeleteClient)
		oauth.Get("/authorize", gw.Provider.Authorize)
		oauth.Post("/authorize", gw.Provider.Authorize)
	})

	// -- SSO OIDC (protected, requires auth) --------------------
	protected.PartyFunc("/sso", func(sso iris.Party) {
		sso.Get("/authorize", gw.SSO.Authorize)
	})
}

// userRoutes 用户：公开资料页走 v1 + 可选登录，其余（资料修改、检索、角色、信任等级、全局与版面禁言、活跃度）走 protected。
func (r *reg) userRoutes() {
	v1, protected, h := r.v1, r.protected, r.h

	// -- User -------------------------------------------------
	// Public profile (optional auth for is_following check)
	v1.PartyFunc("/user", func(u iris.Party) {
		u.Get("/{id}/profile", h.User.GetPublicProfile)
	})

	protected.PartyFunc("/user", func(u iris.Party) {
		u.Get("/profile", h.User.GetProfile)
		u.Post("/profile/update", h.User.UpdateProfile)
		u.Post("/search", h.User.Search)
		u.Post("/list", h.User.List)
		u.Get("/{id}", h.User.GetByID)
		u.Post("/{id}/delete", h.User.Delete)
		u.Post("/{id}/role/update", h.User.UpdateRole)
		u.Post("/{id}/trust_level/update", h.User.UpdateTrustLevel)

		// 全局禁言
		u.Post("/{id}/banned", h.Banned.Ban)
		u.Post("/{id}/unbanned", h.Banned.Unban)
		u.Post("/{id}/banned/status", h.Banned.Get)
		u.Post("/{id}/banned/overview", h.Banned.ListByUserID)

		// 活跃度
		u.Post("/{id}/activity", h.User.GetActivity)

		// 版面禁言
		u.Post("/{id}/space/{space_id}/banned", h.Banned.BanInSpace)
		u.Post("/{id}/space/{space_id}/unbanned", h.Banned.UnbanInSpace)
		u.Post("/{id}/space/{space_id}/banned/status", h.Banned.GetInSpace)
	})
}
