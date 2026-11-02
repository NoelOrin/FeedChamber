package handler

import (
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"

	"github.com/kataras/iris/v12"
)

// UserSession represents the current authenticated user's session data
// extracted from context values set by the gateway auth middleware (session or opaque OAuth token).
// ID is the canonical user identifier (ULID).
type UserSession struct {
	ID       string
	Username string
	Role     string
}

// currentSession returns the server-side session the gateway auth middleware
// resolved for this request (opaque-credential path), or nil for API-key /
// OAuth-client callers which carry no session row.
func currentSession(ctx iris.Context) *model.UserSession {
	sess, _ := ctx.Values().Get(pkg.CtxSession).(*model.UserSession)
	return sess
}

// currentUser extracts the authenticated user's session info from the request context.
func currentUser(ctx iris.Context) UserSession {
	return UserSession{
		ID:       ctx.Values().GetString(pkg.CtxUserID),
		Username: ctx.Values().GetString(pkg.CtxUsername),
		Role:     ctx.Values().GetString(pkg.CtxRole),
	}
}

// currentUserID returns the current user's ID from context.
//
// The context value is User.ID (a ULID), not a UUID. This helper was named
// currentUserID historically; it was renamed on 2026-10-28 because the old
// name made every caller look like it was reading a UUID column.
func currentUserID(ctx iris.Context) string {
	return ctx.Values().GetString(pkg.CtxUserID)
}

// currentRole returns the current user's role from context.
func currentRole(ctx iris.Context) string {
	return ctx.Values().GetString(pkg.CtxRole)
}

// requireUser extracts the current user from context.
// Returns the user and true if authenticated; otherwise writes a 401 response and returns zero value + false.
func requireUser(ctx iris.Context) (UserSession, bool) {
	u := currentUser(ctx)
	if u.ID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return UserSession{}, false
	}
	return u, true
}
