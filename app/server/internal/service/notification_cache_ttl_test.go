package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"feedchamber/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The unread-count cache has a 5-minute TTL whose purpose is to periodically
// re-sync with the DB. CountUnread honours that TTL (Get -> isExpired -> DB
// fallback), but Create uses IncrBy, which reads the raw JetStream value and
// NEVER checks the TTL marker. If the cache expires and no read has purged it
// yet, Create increments the stale value and publishes a wrong unread count to
// every SSE client. This test pins that the pushed count matches the DB.
func TestNotificationCreate_RefreshesExpiredCacheBeforeIncrement(t *testing.T) {
	repo := newMockNotificationRepo()
	svc, kv := newTestNotificationService(t, repo)
	svc.emailDisp = &emailDispatcher{
		ch:   make(chan emailTask, 16),
		svc:  svc,
		quit: make(chan struct{}),
	}
	ctx := context.Background()
	const userID = "u1"

	// One genuinely unread notification in the DB.
	require.NoError(t, repo.Create(ctx, &model.Notification{
		ID: "existing", UserID: userID, Type: "comment", IsRead: false,
	}))

	// Plant a stale cached count that is inconsistent with the DB, with a TTL
	// that is about to expire. We deliberately do NOT call Get afterwards: the
	// physical KV value stays stale until something checks the TTL marker.
	require.NoError(t, kv.Set(ctx, svc.unreadKey(userID), "5", 1*time.Second))
	time.Sleep(1200 * time.Millisecond)

	// Subscribe before Create so we capture the exact payload pushed to SSE.
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch, err := kv.Subscribe(subCtx, "notif:"+userID)
	require.NoError(t, err)

	// A second notification arrives. DB unread count is now 2.
	require.NoError(t, svc.Create(ctx, userID, "comment", "hi", "post-1", "sender"))

	var pushed int64
	select {
	case payload := <-ch:
		var msg struct {
			Unread int64 `json:"unread"`
		}
		require.NoError(t, json.Unmarshal(payload, &msg))
		pushed = msg.Unread
	case <-time.After(2 * time.Second):
		t.Fatal("no unread update published")
	}

	count, err := svc.CountUnread(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count, "cache must converge to the DB count")
	assert.Equal(t, int64(2), pushed, "SSE push must carry the DB-accurate count, not an increment of the expired stale cache")
}
