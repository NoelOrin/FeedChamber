package router

import (
	"github.com/kataras/iris/v12"
)

// accountRoutes 账号自用端点：签到、API Key、草稿、登录会话、主题偏好、安全风控。
func (r *reg) accountRoutes() {
	protected, h, gw := r.protected, r.h, r.gw

	// Check-in
	protected.PartyFunc("/checkin", func(c iris.Party) {
		c.Post("", h.CheckIn.CheckIn)
		c.Get("/status", h.CheckIn.GetStatus)
		c.Get("/history", h.CheckIn.GetHistory)
	})

	// -- API Keys (auth required) ---------------------------
	protected.PartyFunc("/api-keys", func(ak iris.Party) {
		ak.Post("", gw.APIKey.Create)
		ak.Post("/list", gw.APIKey.List)
		ak.Post("/{id}/revoke", gw.APIKey.Revoke)
	})

	// -- Drafts (auth required) ------------------------------
	protected.PartyFunc("/drafts", func(d iris.Party) {
		d.Post("/save", h.Draft.Save)
		d.Post("/list", h.Draft.List)
		d.Post("/{id}/delete", h.Draft.Delete)
	})

	// -- Sessions (auth required) --------------------------------
	protected.PartyFunc("/sessions", func(sess iris.Party) {
		sess.Get("", gw.Session.ListSessions)
		sess.Post("/{id}/revoke", gw.Session.RevokeSession)
		sess.Post("/logout-all", gw.Session.LogoutAll)
	})

	// -- Themes (auth required, self preference) -------------------------
	protected.PartyFunc("/themes", func(th iris.Party) {
		th.Post("/preference", h.Theme.GetPreference)
		th.Post("/preference/update", h.Theme.SetPreference)
	})

	// -- Security / Risk Control (auth required) ------------------
	protected.PartyFunc("/security", func(sec iris.Party) {
		sec.Get("/events", gw.Security.ListSecurityEvents)
		sec.Get("/login-history", gw.Security.ListLoginHistory)
		sec.Post("/report", gw.Security.ReportSuspicious)
	})
}
