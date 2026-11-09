package service

import (
	"testing"
	"time"

	"feedchamber/internal/pkg/eventbus"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A full notification queue must never block the EventBus worker that is
// dispatching the event: the worker is shared by every subscriber, so a slow
// DB/KV behind the notification service would otherwise stall unrelated events.
// The drop must be observable in logs, not silent.
func TestNotifDispatcher_EnqueueDropsWithoutBlockingWhenFull(t *testing.T) {
	hook := test.NewGlobal()
	defer logrus.StandardLogger().ReplaceHooks(make(logrus.LevelHooks))

	d := newNotifDispatcher(nil)
	// Fill the buffer; no workers are started, so nothing drains it.
	for i := 0; i < notifQueueSize; i++ {
		d.ch <- notifyTask{userID: "fill"}
	}

	done := make(chan struct{})
	go func() {
		d.enqueue(notifyTask{userID: "dropped", notifType: "comment"})
		close(done)
	}()

	select {
	case <-done:
		// enqueue returned promptly — the EventBus worker is not held hostage.
	case <-time.After(2 * time.Second):
		t.Fatal("enqueue blocked on a full notification queue")
	}

	entries := hook.AllEntries()
	require.NotEmpty(t, entries, "a dropped notification task must emit a log line")
	last := entries[len(entries)-1]
	assert.Equal(t, logrus.WarnLevel, last.Level)
	assert.Equal(t, "comment", last.Data["type"])
	assert.Contains(t, last.Message, "drop")

	// The buffered tasks must still be there: a drop must not corrupt or evict
	// already-accepted work.
	assert.Len(t, d.ch, notifQueueSize)
}

// A healthy (non-full) queue accepts the task and stays silent.
func TestNotifDispatcher_EnqueueAcceptsWhenSpaceAvailable(t *testing.T) {
	hook := test.NewGlobal()
	defer logrus.StandardLogger().ReplaceHooks(make(logrus.LevelHooks))

	d := newNotifDispatcher(nil)
	d.enqueue(notifyTask{userID: "u1", notifType: eventbus.EventUserFollowed})

	assert.Len(t, d.ch, 1)
	assert.Empty(t, hook.AllEntries())
}
