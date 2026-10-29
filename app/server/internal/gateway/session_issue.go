package gateway

import (
	"context"
	"time"

	"feedchamber/internal/model"
)

// sessionTouchInterval throttles last_active writes so a busy page does not
// hammer the DB on every request (SessionStore.Touch is a no-op inside the
// window).
const sessionTouchInterval = time.Minute

// Issue mints a fresh opaque credential plus its server-side row for the
// user. Login/Register owns session creation — the tracking middleware never
// creates rows anymore. Expiry is `now + model.SessionTTL`; the same horizon
// is reused for the sliding renewal and the cookie Max-Age so the three never
// diverge.
func (s *SessionStore) Issue(ctx context.Context, user *model.User, ip, userAgent string) (*model.UserSession, error) {
	sid, err := NewSessionID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	sess := &model.UserSession{
		SessionID:  sid,
		UserUUID:   user.ID,
		Username:   user.Username,
		Role:       user.Role,
		IP:         ip,
		UserAgent:  userAgent,
		DeviceName: extractDeviceName(userAgent),
		CreatedAt:  now.Unix(),
		ExpiresAt:  now.Add(model.SessionTTL).Unix(),
	}
	if err := s.Create(ctx, sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// Rotate swaps a session for a brand-new credential carrying the same user
// and device context, then revokes the old one. Used by POST /auth/refresh:
// an old (possibly leaked) sid dies the moment a fresh one is handed out.
func (s *SessionStore) Rotate(ctx context.Context, old *model.UserSession) (*model.UserSession, error) {
	sid, err := NewSessionID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	fresh := &model.UserSession{
		SessionID:  sid,
		UserUUID:   old.UserUUID,
		Username:   old.Username,
		Role:       old.Role,
		IP:         old.IP,
		UserAgent:  old.UserAgent,
		DeviceName: old.DeviceName,
		Location:   old.Location,
		CreatedAt:  now.Unix(),
		ExpiresAt:  now.Add(model.SessionTTL).Unix(),
	}
	if err := s.Create(ctx, fresh); err != nil {
		return nil, err
	}
	if err := s.Revoke(ctx, old.SessionID); err != nil {
		return nil, err
	}
	return fresh, nil
}

// shouldRenew reports whether an active session has slid below the renewal
// threshold (less than 50% of its lifetime left) and should be extended back
// to the absolute horizon on this request.
func shouldRenew(sess *model.UserSession, now time.Time) bool {
	remaining := time.Until(time.Unix(sess.ExpiresAt, 0))
	return remaining < time.Duration(model.SessionRenewRatio*float64(model.SessionTTL))
}
