package pkg

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
)

const (
	AuthFlagCookie = "master_auth"
	// SessionCookie carries the opaque user-session credential (64-hex sid).
	// Since the session-auth migration it is the only user credential the
	// server accepts from cookies; the legacy access/refresh-token cookies
	// (master_access_token, master_refresh_token) are gone.
	SessionCookie = "master_session"
)

func CookieSecure() bool {
	v := strings.ToLower(os.Getenv("MASTER_COOKIE_SECURE"))
	return v == "true" || v == "1"
}

// ── Session cookie helpers ──────────────────────────────────
//
// The session cookie holds the opaque sid minted by SessionStore — it is the
// user's only credential. Conventions: Path=/, HttpOnly, SameSite=Lax, Secure
// from CookieSecure(), plus the JS-readable master_auth flag the frontend
// guards rely on.

// SetSessionCookie writes the HttpOnly master_session cookie and refreshes
// the master_auth flag with the same Max-Age as the session's remaining life.
func SetSessionCookie(ctx iris.Context, sid string, ttl time.Duration) {
	maxAge := int(ttl.Seconds())
	if maxAge <= 0 {
		maxAge = 604800 // 7d safety net
	}
	secure := CookieSecure()
	ctx.SetCookie(&http.Cookie{
		Name:     SessionCookie,
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
	ctx.SetCookie(&http.Cookie{
		Name:     AuthFlagCookie,
		Value:    "1",
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

// ClearSessionCookie expires both the session credential and the auth flag.
func ClearSessionCookie(ctx iris.Context) {
	secure := CookieSecure()
	ctx.SetCookie(&http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	ctx.SetCookie(&http.Cookie{
		Name:     AuthFlagCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// ReadSessionCookie returns the opaque session id from the HttpOnly cookie,
// or empty string if absent. Used by the auth middleware as the fallback when
// the Authorization header is not provided.
func ReadSessionCookie(ctx iris.Context) string {
	return ctx.GetCookie(SessionCookie)
}
