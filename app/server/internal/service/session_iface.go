package service

import (
	"context"

	"orinforum/internal/model"
)

// SessionStore is the server-side session authority (implemented by
// *gateway.SessionStore). Since the session-auth migration it replaces the
// old JWT + blacklist pair entirely: issuing a credential creates a row, revoking
// deletes it, and a credential that resolves to nothing is not authenticated.
//
// Kept as a consumer-side interface so the service layer never imports the
// gateway package.
type SessionStore interface {
	// Issue mints an opaque session id plus its row for the user.
	Issue(ctx context.Context, user *model.User, ip, userAgent string) (*model.UserSession, error)
	// Rotate swaps an existing session for a fresh credential and revokes the
	// old one (POST /auth/refresh).
	Rotate(ctx context.Context, old *model.UserSession) (*model.UserSession, error)
	// Revoke drops one credential immediately.
	Revoke(ctx context.Context, sessionID string) error
	// RevokeAllForUser signs a user out everywhere — role change, password
	// change, ban.
	RevokeAllForUser(ctx context.Context, userUUID string) error
}
