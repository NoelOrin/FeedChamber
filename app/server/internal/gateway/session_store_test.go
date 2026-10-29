package gateway

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"orinforum/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type stubKV struct {
	baseKV
	data map[string]string
	ttls map[string]time.Duration
}

func newStubKV() *stubKV {
	return &stubKV{data: map[string]string{}, ttls: map[string]time.Duration{}}
}

func (k *stubKV) Set(_ context.Context, key, value string, ttl time.Duration) error {
	k.data[key] = value
	k.ttls[key] = ttl
	return nil
}

func (k *stubKV) Get(_ context.Context, key string) (string, error) {
	return k.data[key], nil
}

func (k *stubKV) Delete(_ context.Context, key string) error {
	delete(k.data, key)
	delete(k.ttls, key)
	return nil
}

func newStore(t *testing.T) (*SessionStore, *stubKV, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.UserSession{}, &model.OAuthToken{}, &model.User{}))
	kv := newStubKV()
	store := NewSessionStore(Deps{DB: db, KV: kv})
	require.NotNil(t, store)
	return store, kv, db
}

func newTestSession(sid, userUUID string, expiresAt int64) *model.UserSession {
	return &model.UserSession{
		SessionID: sid, UserUUID: userUUID, Username: "lin", Role: "user",
		ExpiresAt: expiresAt, CreatedAt: time.Now().Unix(),
	}
}

func TestNewSessionStore_MissingDeps(t *testing.T) {
	assert.Nil(t, NewSessionStore(Deps{KV: newStubKV()}), "no DB")
	assert.Nil(t, NewSessionStore(Deps{DB: &gorm.DB{}}), "no KV")
}

func TestNewSessionID_UniqueAndOpaque(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		id, err := NewSessionID()
		require.NoError(t, err)
		assert.Len(t, id, 64, "256-bit hex")
		assert.False(t, seen[id], "ids must not repeat")
		seen[id] = true
		assert.NotContains(t, id, ".", "must not look like a JWT")
	}
}

func TestSessionStore_CreateAndLookup(t *testing.T) {
	store, _, db := newStore(t)
	ctx := context.Background()
	sess := newTestSession("sid-a", "01USER", time.Now().Add(time.Hour).Unix())

	require.NoError(t, store.Create(ctx, sess))

	got, err := store.Lookup(ctx, "sid-a")
	require.NoError(t, err)
	assert.Equal(t, "01USER", got.UserUUID)
	assert.Equal(t, "lin", got.Username, "denormalised fields must survive the round trip")
	assert.Equal(t, "user", got.Role)
	assert.Greater(t, got.LastActive, int64(0))

	var count int64
	require.NoError(t, db.Model(&model.UserSession{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestSessionStore_LookupUnknown(t *testing.T) {
	store, _, _ := newStore(t)
	_, err := store.Lookup(context.Background(), "nope")
	assert.ErrorIs(t, err, ErrSessionNotFound)

	_, err = store.Lookup(context.Background(), "")
	assert.ErrorIs(t, err, ErrSessionNotFound, "empty credential must never resolve")
}

// A cache hit must not be trusted past the session's own expiry, otherwise a
// KV entry that outlived its invalidation would keep authenticating requests.
func TestSessionStore_CachedSessionStillRespectsExpiry(t *testing.T) {
	store, kv, _ := newStore(t)
	ctx := context.Background()
	sess := newTestSession("sid-old", "01USER", time.Now().Add(time.Hour).Unix())

	require.NoError(t, store.Create(ctx, sess))
	require.NotEmpty(t, kv.data[sessionCachePrefix+"sid-old"], "create should warm the cache")

	// Age the cached entry past expiry without touching the authoritative row.
	stale := *sess
	stale.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	payload, err := json.Marshal(&stale)
	require.NoError(t, err)
	kv.data[sessionCachePrefix+"sid-old"] = string(payload)

	_, err = store.Lookup(ctx, "sid-old")
	assert.ErrorIs(t, err, ErrSessionNotFound)
	assert.NotContains(t, kv.data, sessionCachePrefix+"sid-old", "stale entry should be evicted")
}

// Revocation has to clear the cache too, otherwise the credential keeps
// resolving from KV even though the authoritative row is gone.
func TestSessionStore_RevokeClearsCache(t *testing.T) {
	store, kv, db := newStore(t)
	ctx := context.Background()
	require.NoError(t, store.Create(ctx, newTestSession("sid-r", "01USER", time.Now().Add(time.Hour).Unix())))

	require.NoError(t, store.Revoke(ctx, "sid-r"))

	assert.NotContains(t, kv.data, sessionCachePrefix+"sid-r", "cache entry must be dropped")
	var count int64
	require.NoError(t, db.Model(&model.UserSession{}).Where("session_id = ?", "sid-r").Count(&count).Error)
	assert.Equal(t, int64(0), count)

	_, err := store.Lookup(ctx, "sid-r")
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionStore_RevokeAllForUser(t *testing.T) {
	store, kv, db := newStore(t)
	ctx := context.Background()
	later := time.Now().Add(time.Hour).Unix()
	require.NoError(t, store.Create(ctx, newTestSession("u1-a", "01AAA", later)))
	require.NoError(t, store.Create(ctx, newTestSession("u1-b", "01AAA", later)))
	require.NoError(t, store.Create(ctx, newTestSession("u2-a", "01BBB", later)))

	require.NoError(t, store.RevokeAllForUser(ctx, "01AAA"))

	var remaining []string
	require.NoError(t, db.Model(&model.UserSession{}).Pluck("session_id", &remaining).Error)
	assert.Equal(t, []string{"u2-a"}, remaining)

	for key := range kv.data {
		assert.False(t, strings.HasPrefix(key, sessionCachePrefix+"u1-"), "revoked user must have no cached sessions")
	}
	_, err := store.Lookup(ctx, "u1-a")
	assert.ErrorIs(t, err, ErrSessionNotFound)
	_, err = store.Lookup(ctx, "u2-a")
	assert.NoError(t, err, "other users unaffected")
}

func TestSessionStore_TouchThrottled(t *testing.T) {
	store, _, db := newStore(t)
	ctx := context.Background()
	sess := newTestSession("sid-t", "01USER", time.Now().Add(time.Hour).Unix())
	require.NoError(t, store.Create(ctx, sess))

	first := sess.LastActive
	time.Sleep(1100 * time.Millisecond)

	store.Touch(ctx, sess, time.Minute)
	assert.Equal(t, first, sess.LastActive, "within the interval it must be a no-op")

	store.Touch(ctx, sess, 0)
	assert.Greater(t, sess.LastActive, first)

	var stored model.UserSession
	require.NoError(t, db.Where("session_id = ?", "sid-t").First(&stored).Error)
	assert.Equal(t, sess.LastActive, stored.LastActive, "touch must persist")
}

func TestSessionStore_ExtendSlidesExpiry(t *testing.T) {
	store, kv, db := newStore(t)
	ctx := context.Background()
	sess := newTestSession("sid-e", "01USER", time.Now().Add(time.Hour).Unix())
	require.NoError(t, store.Create(ctx, sess))
	original := sess.ExpiresAt

	expires := store.Extend(ctx, sess, 72*time.Hour)
	assert.Greater(t, expires, original)

	var stored model.UserSession
	require.NoError(t, db.Where("session_id = ?", "sid-e").First(&stored).Error)
	assert.Equal(t, expires, stored.ExpiresAt)
	assert.Equal(t, 72*time.Hour, kv.ttls[sessionCachePrefix+"sid-e"].Round(time.Minute),
		"cache TTL must track the new remaining lifetime")

	got, err := store.Lookup(ctx, "sid-e")
	require.NoError(t, err)
	assert.Equal(t, expires, got.ExpiresAt)
}
