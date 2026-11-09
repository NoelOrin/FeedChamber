package service

import (
	"context"
	"testing"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/pkg/testnats"
	"feedchamber/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidWatchTargetAndLevel(t *testing.T) {
	assert.True(t, validTargetType(model.WatchTargetPost))
	assert.True(t, validTargetType(model.WatchTargetTag))
	// 版面关注已于 2026-10-28 移除（前台无入口、后端不再接受）。这条断言是收口的钉子：
	// 谁想把 "space" 加回 validTargetType，必须先回答「前台入口在哪」。
	assert.False(t, validTargetType("space"))
	assert.False(t, validTargetType("user"))

	assert.True(t, validLevel(model.WatchLevelWatching))
	assert.True(t, validLevel(model.WatchLevelTracking))
	assert.True(t, validLevel(model.WatchLevelMuted))
	assert.False(t, validLevel("loud"))
}

// stubWatchRepo / stubTagRepo 只实现 NotifyWatchers 会调用的两个方法；其余方法
// 由内嵌的 nil interface 兜底（一旦扇出逻辑新增了查询，测试会 panic 提醒补桩）。
type stubWatchRepo struct {
	repository.WatchRepository
	activeIDs []string
}

func (r *stubWatchRepo) ListActiveUserIDs(_ context.Context, _, _ string) ([]string, error) {
	return r.activeIDs, nil
}

type stubTagRepo struct {
	repository.TagRepository
}

func (r *stubTagRepo) GetTagsByPostID(_ context.Context, _ string) ([]model.Tag, error) {
	return nil, nil
}

// collectNotifiedWatchers drains the async EventBus recipients into a slice.
// NATS delivery + worker dispatch is asynchronous, so we wait briefly for the
// stream to go quiet instead of asserting on a single immediate receive.
func collectNotifiedWatchers(t *testing.T, got <-chan string) []string {
	t.Helper()
	var recipients []string
	deadline := time.After(3 * time.Second)
	for {
		select {
		case uid := <-got:
			recipients = append(recipients, uid)
		case <-time.After(300 * time.Millisecond):
			return recipients
		case <-deadline:
			return recipients
		}
	}
}

// A single comment can reach the same person as the post author AND as a topic
// watcher. The comment fan-out already notified the author; the shared `notified`
// set exists precisely so the watch step skips them. Regression guard: a prior
// refactor aliased the publish loop to that shared set and re-notified everyone.
func TestNotifyWatchers_SkipsAlreadyNotifiedRecipients(t *testing.T) {
	bus := testnats.NewEventBus(t)
	got := make(chan string, 16)
	bus.Subscribe(eventbus.EventContentWatched, func(e eventbus.Event) {
		uid, _ := e.Payload["user_id"].(string)
		got <- uid
	})

	// author-1 is already in `notified` (post author); watcher-1 is only a watcher.
	svc := &watchService{
		repo:    &stubWatchRepo{activeIDs: []string{"author-1", "watcher-1"}},
		tagRepo: &stubTagRepo{},
		bus:     bus,
	}
	notified := map[string]struct{}{"author-1": {}}

	svc.NotifyWatchers(context.Background(), "post-1", "commenter", "Commenter", "A Post", notified)

	recipients := collectNotifiedWatchers(t, got)
	assert.ElementsMatch(t, []string{"watcher-1"}, recipients,
		"post author already notified by the comment fan-out must NOT receive a second (watch) notification")
}

// The watch step must still record its own recipients in the shared set, so any
// later fan-out step added for the same event keeps deduplicating against them.
func TestNotifyWatchers_RegistersNewRecipientsInSharedSet(t *testing.T) {
	bus := testnats.NewEventBus(t)
	got := make(chan string, 16)
	bus.Subscribe(eventbus.EventContentWatched, func(e eventbus.Event) {
		uid, _ := e.Payload["user_id"].(string)
		got <- uid
	})

	svc := &watchService{
		repo:    &stubWatchRepo{activeIDs: []string{"watcher-1"}},
		tagRepo: &stubTagRepo{},
		bus:     bus,
	}
	notified := map[string]struct{}{}

	svc.NotifyWatchers(context.Background(), "post-1", "commenter", "Commenter", "A Post", notified)

	require.ElementsMatch(t, []string{"watcher-1"}, collectNotifiedWatchers(t, got))
	_, ok := notified["watcher-1"]
	assert.True(t, ok, "newly notified watcher must be recorded in the shared set so later fan-out steps skip them")
}
