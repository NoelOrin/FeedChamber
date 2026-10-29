package gateway

import (
	"errors"
	"strings"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"

	"github.com/kataras/iris/v12"
)

// AuthChecker resolves user credentials against the server-side session store
// and sets user context values.
type AuthChecker struct {
	deps     Deps
	sessions *SessionStore
}

func NewAuthChecker(deps Deps) *AuthChecker {
	return &AuthChecker{
		deps:     deps,
		sessions: NewSessionStore(deps),
	}
}

// presentCredential extracts the user credential from the request.
// Priority: Authorization: Bearer <sid> > master_session HttpOnly cookie.
// A non-Bearer Authorization header is rejected outright (keeps the old error
// codes so clients keep seeing 1001 for malformed headers).
func presentCredential(ctx iris.Context) (cred string, badHeader bool) {
	authHeader := ctx.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer "), false
	}
	if strings.TrimSpace(authHeader) != "" {
		return "", true
	}
	return pkg.ReadSessionCookie(ctx), false
}

// Middleware resolves the presented credential to a live session.
// Returns true if the request should continue. Stops with the established
// codes: 1001 missing credential, 1002 invalid credential,
// 1003 expired (client token), 1014 revoked (client token).
func (a *AuthChecker) Middleware(ctx iris.Context) bool {
	cred, badHeader := presentCredential(ctx)
	if badHeader {
		pkg.Fail(ctx, pkg.TOKEN_WRONG)
		return false
	}
	if cred == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return false
	}

	// 1. User credential = opaque session id resolved against user_sessions.
	if a.sessions != nil {
		sess, err := a.sessions.Lookup(ctx.Request().Context(), cred)
		if err == nil && sess != nil {
			a.applySession(ctx, sess)
			return true
		}
		if err != nil && !errors.Is(err, ErrSessionNotFound) {
			pkg.Fail(ctx, pkg.TOKEN_WRONG, "令牌无效或已过期")
			return false
		}
	}

	// 2. OAuth2/OIDC access tokens are opaque rows in oauth_tokens (see token.go).
	row, err := lookupOAuthAccessToken(ctx.Request().Context(), a.deps.DB, cred)
	switch {
	case err == nil:
		if !a.applyOAuthToken(ctx, row) {
			pkg.Fail(ctx, pkg.TOKEN_WRONG, "令牌无效或已过期")
			return false
		}
		return true
	case errors.Is(err, errOAuthTokenExpired):
		pkg.Fail(ctx, pkg.TOKEN_EXPIRED)
		return false
	case errors.Is(err, errOAuthTokenRevoked):
		pkg.Fail(ctx, pkg.TOKEN_REVOKED)
		return false
	default:
		pkg.Fail(ctx, pkg.TOKEN_WRONG, "令牌无效或已过期")
		return false
	}
}

// OptionalMiddleware resolves the credential when present but never rejects.
// A missing, unknown or forged credential simply continues unauthenticated.
func (a *AuthChecker) OptionalMiddleware(ctx iris.Context) bool {
	cred, badHeader := presentCredential(ctx)
	if badHeader || cred == "" {
		return true
	}

	if a.sessions != nil {
		if sess, err := a.sessions.Lookup(ctx.Request().Context(), cred); err == nil && sess != nil {
			a.applySession(ctx, sess)
			return true
		}
	}

	if row, err := lookupOAuthAccessToken(ctx.Request().Context(), a.deps.DB, cred); err == nil {
		a.applyOAuthToken(ctx, row)
	}
	return true
}

// applyOAuthToken projects an opaque OAuth2/OIDC access token onto request
// context. For user-bound tokens the username and role are read from the user
// row on every request, so a role change applies immediately; client_credentials
// tokens carry no user and are projected as the oauth_client pseudo-role.
func (a *AuthChecker) applyOAuthToken(ctx iris.Context, row *model.OAuthToken) bool {
	username := row.ClientID
	role := oauthClientRole
	userID := ""
	if row.UserID != "" {
		var user model.User
		if err := a.deps.DB.WithContext(ctx.Request().Context()).
			Where("id = ?", row.UserID).First(&user).Error; err != nil {
			return false
		}
		username, role, userID = user.Username, user.Role, user.ID
	}
	ctx.Values().Set(pkg.CtxUsername, username)
	ctx.Values().Set(pkg.CtxUserID, userID)
	ctx.Values().Set(pkg.CtxRole, role)
	ctx.Values().Set(pkg.CtxAuthMethod, pkg.AuthMethodOAuthClient)
	ctx.Values().Set(pkg.CtxClientScopes, row.Scope)
	return true
}

// applySession projects the session row onto request context and applies the
// sliding-renewal policy: when the remaining lifetime has dropped below 50%
// of the horizon, active use extends it back and the cookie is re-issued so
// the browser's Max-Age matches the new DB expiry.
func (a *AuthChecker) applySession(ctx iris.Context, sess *model.UserSession) {
	ctx.Values().Set(pkg.CtxUsername, sess.Username)
	ctx.Values().Set(pkg.CtxUserID, sess.UserUUID)
	ctx.Values().Set(pkg.CtxRole, sess.Role)
	ctx.Values().Set(pkg.CtxSession, sess)

	if a.sessions == nil {
		return
	}
	reqCtx := ctx.Request().Context()
	if shouldRenew(sess, time.Now()) {
		expires := a.sessions.Extend(reqCtx, sess, model.SessionTTL)
		pkg.SetSessionCookie(ctx, sess.SessionID, time.Until(time.Unix(expires, 0)))
	}
}
