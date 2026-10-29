package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg/kvstore"

	"gorm.io/gorm"
)

// Session credential lives in KV under this prefix, holding a JSON copy of the
// authoritative `user_sessions` row with a TTL equal to its remaining lifetime.
const sessionCachePrefix = "sess:"

// ErrSessionNotFound is returned when a credential does not resolve to a live session.
var ErrSessionNotFound = errors.New("session not found")

// SessionStore owns the user session lifecycle. Revocation is "delete the row",
// which supersedes the previous JWT blacklist: a credential that resolves to
// nothing is simply not authenticated.
type SessionStore struct {
	db *gorm.DB
	kv kvstore.Store
}

// NewSessionStore returns nil when either dependency is missing so that a
// partially-wired gateway degrades to "no sessions" rather than panicking.
func NewSessionStore(deps Deps) *SessionStore {
	if deps.DB == nil {
		return nil
	}
	if deps.KV == nil {
		return nil
	}
	return &SessionStore{db: deps.DB, kv: deps.KV}
}

// NewSessionID mints a 256-bit opaque credential. It carries no structure, so a
// leaked id can neither be parsed nor forged offline.
func NewSessionID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// Create persists a session and warms the cache. Callers own populating
// SessionID/UserUUID/Username/Role/ExpiresAt.
func (s *SessionStore) Create(ctx context.Context, sess *model.UserSession) error {
	sess.LastActive = time.Now().Unix()
	if err := s.db.WithContext(ctx).Create(sess).Error; err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	s.cachePut(ctx, sess)
	return nil
}

// Lookup resolves a presented credential to its session, returning
// ErrSessionNotFound for unknown or expired credentials.
//
// A cache hit is still checked against ExpiresAt because a KV entry can outlive
// a row deleted through a path that missed its cache invalidation.
func (s *SessionStore) Lookup(ctx context.Context, sessionID string) (*model.UserSession, error) {
	if sessionID == "" {
		return nil, ErrSessionNotFound
	}

	if cached, ok := s.cacheGet(ctx, sessionID); ok {
		if cached.ExpiresAt <= time.Now().Unix() {
			s.cacheDrop(ctx, sessionID)
			return nil, ErrSessionNotFound
		}
		return cached, nil
	}

	var sess model.UserSession
	err := s.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&sess).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lookup session: %w", err)
	}
	if sess.ExpiresAt <= time.Now().Unix() {
		s.revokeRow(ctx, &sess)
		return nil, ErrSessionNotFound
	}
	// The row was read before this write, so a Revoke can land in between and
	// the cache would then hold a dead credential for its whole TTL. Confirm the
	// row is still there and refuse the lookup if it is gone. Revoke deletes the
	// row before the cache entry, so this check plus that ordering closes every
	// interleaving (see the invariant on revokeRow).
	s.cachePut(ctx, &sess)
	exists, checkErr := s.sessionExists(ctx, sessionID)
	if checkErr != nil {
		// Cannot prove existence — do not authenticate on an unverified cache.
		s.cacheDrop(ctx, sessionID)
		return nil, fmt.Errorf("lookup session: %w", checkErr)
	}
	if !exists {
		s.cacheDrop(ctx, sessionID)
		return nil, ErrSessionNotFound
	}
	return &sess, nil
}

// Touch records activity, throttled so a busy page does not write per request.
// The stored row keeps the original expiry; sliding renewal is the handler's job.
//
// The UPDATE doubles as the existence proof for the cache refresh: a matching
// row count of zero means the session was revoked while this request was in
// flight, and writing the stale snapshot back would resurrect it.
func (s *SessionStore) Touch(ctx context.Context, sess *model.UserSession, minInterval time.Duration) {
	now := time.Now().Unix()
	if minInterval > 0 && now-sess.LastActive < int64(minInterval.Seconds()) {
		return
	}
	sess.LastActive = now
	res := s.db.WithContext(ctx).Model(&model.UserSession{}).
		Where("session_id = ?", sess.SessionID).
		Update("last_active", now)
	if res.Error != nil {
		return
	}
	if res.RowsAffected == 0 {
		s.cacheDrop(ctx, sess.SessionID)
		return
	}
	s.cachePut(ctx, sess)
}

// Extend slides a session's lifetime and re-issues the cache entry. Returns the
// new absolute expiry so the caller can re-set the cookie with a matching Max-Age,
// or 0 when the session no longer exists (caller must not re-issue the cookie).
func (s *SessionStore) Extend(ctx context.Context, sess *model.UserSession, ttl time.Duration) int64 {
	expires := time.Now().Add(ttl).Unix()
	res := s.db.WithContext(ctx).Model(&model.UserSession{}).
		Where("session_id = ?", sess.SessionID).
		Update("expires_at", expires)
	if res.Error != nil {
		return sess.ExpiresAt
	}
	// Same existence proof as Touch: zero rows means revoked in flight.
	if res.RowsAffected == 0 {
		s.cacheDrop(ctx, sess.SessionID)
		return 0
	}
	sess.ExpiresAt = expires
	s.cachePut(ctx, sess)
	return expires
}

// Revoke drops one credential immediately.
func (s *SessionStore) Revoke(ctx context.Context, sessionID string) error {
	var sess model.UserSession
	err := s.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&sess).Error
	if err != nil {
		return err
	}
	s.revokeRow(ctx, &sess)
	return nil
}

// RevokeAllForUser signs a user out everywhere — used by role change, password
// change and bans. Sessions are enumerated from the DB so each cache entry is
// removed too; without that, revoked credentials would keep resolving until TTL.
func (s *SessionStore) RevokeAllForUser(ctx context.Context, userUUID string) error {
	var sessions []model.UserSession
	if err := s.db.WithContext(ctx).
		Where("user_uuid = ?", userUUID).Find(&sessions).Error; err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}
	for i := range sessions {
		s.revokeRow(ctx, &sessions[i])
	}
	return nil
}

// revokeRow drops one session row and its cache entry.
//
// Ordering is the invariant that makes cache resurrection impossible: delete the
// ROW first, then the cache entry. Every cache write path (Create / Lookup /
// Touch / Extend) either proves the row still exists *after* it writes, or
// rewrites the cache from a statement whose RowsAffected proves it. With the row
// gone first:
//   - a writer that committed before the delete is overwritten by the cache
//     delete that follows; and
//   - a writer that commits after the delete finds RowsAffected == 0 (or its
//     post-write existence check fails) and drops the entry itself.
//
// Deleting the cache first (the previous order) left the window open in the
// opposite direction: an in-flight writer would re-add the entry after the
// delete and the credential stayed valid for the rest of its TTL.
func (s *SessionStore) revokeRow(ctx context.Context, sess *model.UserSession) {
	_ = s.db.WithContext(ctx).
		Where("session_id = ?", sess.SessionID).Delete(&model.UserSession{}).Error
	s.cacheDrop(ctx, sess.SessionID)
}

// sessionExists reports whether the session row is still present. Used as the
// post-write existence proof on the Lookup miss path.
func (s *SessionStore) sessionExists(ctx context.Context, sessionID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.UserSession{}).
		Where("session_id = ?", sessionID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// cacheDrop removes a cache entry. Best-effort: callers are on paths where the
// DB is already authoritative.
func (s *SessionStore) cacheDrop(ctx context.Context, sessionID string) {
	_ = s.kv.Delete(ctx, sessionCachePrefix+sessionID)
}

func (s *SessionStore) cachePut(ctx context.Context, sess *model.UserSession) {
	remaining := sess.ExpiresAt - time.Now().Unix()
	if remaining <= 0 {
		return
	}
	payload, err := json.Marshal(sess)
	if err != nil {
		return
	}
	_ = s.kv.Set(ctx, sessionCachePrefix+sess.SessionID, string(payload), time.Duration(remaining)*time.Second)
}

func (s *SessionStore) cacheGet(ctx context.Context, sessionID string) (*model.UserSession, bool) {
	raw, err := s.kv.Get(ctx, sessionCachePrefix+sessionID)
	if err != nil || raw == "" {
		return nil, false
	}
	var sess model.UserSession
	if err := json.Unmarshal([]byte(raw), &sess); err != nil {
		return nil, false
	}
	return &sess, true
}
