package model

import "time"

// Session lifetime policy — the single source of truth shared by login
// issuance, the gateway auth middleware (sliding renewal) and the cookie
// helpers, so a session's DB expiry and its cookie Max-Age never diverge.
const (
	// SessionTTL is the absolute lifetime of a user session credential.
	// Active sessions are renewed to this horizon, they never outlive it
	// without a fresh renewal.
	SessionTTL = 7 * 24 * time.Hour

	// SessionRenewRatio is the sliding-renewal threshold: when a session's
	// remaining lifetime drops below this fraction of SessionTTL, an
	// authenticated request extends it back to the full horizon.
	SessionRenewRatio = 0.5
)

// UserSession is the server-side record of a logged-in browser/device.
//
// SessionID is the opaque credential the client presents (via the
// `master_session` HttpOnly cookie or `Authorization: Bearer`); it carries no
// structure of its own, so every request is resolved against this table.
// Username/Role are denormalised to keep the hot path to a single lookup —
// they are safe to cache here because role changes revoke all sessions.
type UserSession struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	SessionID  string `gorm:"uniqueIndex;not null" json:"session_id"`
	UserUUID   string `gorm:"index;not null" json:"user_uuid"`
	Username   string `json:"username"`
	Role       string `json:"role"`
	IP         string `json:"ip"`
	UserAgent  string `json:"user_agent"`
	DeviceName string `json:"device_name"`
	Location   string `json:"location"`
	LastActive int64  `json:"last_active"`
	CreatedAt  int64  `json:"created_at"`
	ExpiresAt  int64  `json:"expires_at"`
}

func (UserSession) TableName() string { return "user_sessions" }
