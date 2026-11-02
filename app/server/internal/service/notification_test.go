package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/pkg/kvstore"
	"feedchamber/internal/pkg/testnats"
	"feedchamber/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockNotificationRepo struct {
	mu            sync.Mutex
	items         map[string]*model.Notification
	markReadCalls int
}

func newMockNotificationRepo() *mockNotificationRepo {
	return &mockNotificationRepo{items: make(map[string]*model.Notification)}
}

func (m *mockNotificationRepo) Create(_ context.Context, n *model.Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *n
	m.items[n.ID] = &cp
	return nil
}

func (m *mockNotificationRepo) FindByID(_ context.Context, id string) (*model.Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.items[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *n
	return &cp, nil
}

func (m *mockNotificationRepo) ListByUser(_ context.Context, userID string, offset, limit int) ([]model.Notification, error) {
	return nil, nil
}

func (m *mockNotificationRepo) ListByUserFiltered(_ context.Context, userID, typ, read string, offset, limit int) ([]model.Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []model.Notification
	for _, n := range m.items {
		if n.UserID != userID {
			continue
		}
		if typ != "" && typ != "all" && n.Type != typ {
			continue
		}
		if read == "unread" && n.IsRead {
			continue
		}
		if read == "read" && !n.IsRead {
			continue
		}
		result = append(result, *n)
	}
	if offset > len(result) {
		return []model.Notification{}, nil
	}
	result = result[offset:]
	if limit < len(result) {
		result = result[:limit]
	}
	return result, nil
}

func (m *mockNotificationRepo) CountByUser(_ context.Context, userID string) (int64, error) {
	return 0, nil
}

func (m *mockNotificationRepo) CountByUserFiltered(_ context.Context, userID, typ, read string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var count int64
	for _, n := range m.items {
		if n.UserID != userID {
			continue
		}
		if typ != "" && typ != "all" && n.Type != typ {
			continue
		}
		if read == "unread" && n.IsRead {
			continue
		}
		if read == "read" && !n.IsRead {
			continue
		}
		count++
	}
	return count, nil
}

func (m *mockNotificationRepo) CountUnread(_ context.Context, userID string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var count int64
	for _, n := range m.items {
		if n.UserID == userID && !n.IsRead {
			count++
		}
	}
	return count, nil
}

func (m *mockNotificationRepo) MarkRead(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.markReadCalls++
	n, ok := m.items[id]
	if !ok {
		return repository.ErrNotFound
	}
	n.IsRead = true
	return nil
}

func (m *mockNotificationRepo) MarkAllRead(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, n := range m.items {
		if n.UserID == userID {
			n.IsRead = true
		}
	}
	return nil
}

func (m *mockNotificationRepo) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, id)
	return nil
}

func newTestNotificationService(t *testing.T, repo *mockNotificationRepo) (*notificationService, kvstore.Store) {
	t.Helper()
	if repo == nil {
		repo = newMockNotificationRepo()
	}
	kv := testnats.NewKVStore(t, "notification_"+t.Name())
	svc := &notificationService{repo: repo, kv: kv}
	return svc, kv
}

func TestNotificationMarkRead_DecrementsUnreadCache(t *testing.T) {
	repo := newMockNotificationRepo()
	svc, kv := newTestNotificationService(t, repo)
	ctx := context.Background()

	n := &model.Notification{
		ID:     "n1",
		UserID: "u1",
		Type:   "comment",
		IsRead: false,
	}
	require.NoError(t, repo.Create(ctx, n))
	require.NoError(t, repo.Create(ctx, &model.Notification{
		ID:     "n2",
		UserID: "u1",
		Type:   "comment",
		IsRead: false,
	}))
	require.NoError(t, kv.Set(ctx, svc.unreadKey("u1"), "3", 0))

	err := svc.MarkRead(ctx, "n1", "u1")
	require.NoError(t, err)

	val, err := kv.Get(ctx, svc.unreadKey("u1"))
	require.NoError(t, err)
	assert.Equal(t, "1", val)

	got, err := repo.FindByID(ctx, "n1")
	require.NoError(t, err)
	assert.True(t, got.IsRead)
	assert.Equal(t, 1, repo.markReadCalls)
}

func TestNotificationMarkRead_AlreadyRead_NoKVDecr(t *testing.T) {
	repo := newMockNotificationRepo()
	svc, kv := newTestNotificationService(t, repo)
	ctx := context.Background()

	n := &model.Notification{
		ID:     "n1",
		UserID: "u1",
		Type:   "comment",
		IsRead: true,
	}
	require.NoError(t, repo.Create(ctx, n))
	require.NoError(t, kv.Set(ctx, svc.unreadKey("u1"), "3", 0))

	err := svc.MarkRead(ctx, "n1", "u1")
	require.NoError(t, err)

	val, err := kv.Get(ctx, svc.unreadKey("u1"))
	require.NoError(t, err)
	assert.Equal(t, "3", val)
	assert.Equal(t, 0, repo.markReadCalls)
}

func TestNotificationMarkRead_WrongUser(t *testing.T) {
	repo := newMockNotificationRepo()
	svc, kv := newTestNotificationService(t, repo)
	ctx := context.Background()

	n := &model.Notification{
		ID:     "n1",
		UserID: "u1",
		Type:   "comment",
		IsRead: false,
	}
	require.NoError(t, repo.Create(ctx, n))
	require.NoError(t, kv.Set(ctx, svc.unreadKey("u1"), "3", 0))

	err := svc.MarkRead(ctx, "n1", "u2")
	require.Error(t, err)

	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.NOT_FOUND, appErr.Code)

	val, err := kv.Get(ctx, svc.unreadKey("u1"))
	require.NoError(t, err)
	assert.Equal(t, "3", val)
	assert.Equal(t, 0, repo.markReadCalls)
}

func TestNotificationMarkRead_NotFound(t *testing.T) {
	repo := newMockNotificationRepo()
	svc, _ := newTestNotificationService(t, repo)

	err := svc.MarkRead(context.Background(), "missing", "u1")
	require.Error(t, err)

	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.NOT_FOUND, appErr.Code)
}

func TestNotificationListFiltered_UsesServerSideFilters(t *testing.T) {
	repo := newMockNotificationRepo()
	svc, _ := newTestNotificationService(t, repo)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &model.Notification{ID: "n1", UserID: "u1", Type: "comment", IsRead: false}))
	require.NoError(t, repo.Create(ctx, &model.Notification{ID: "n2", UserID: "u1", Type: "comment", IsRead: true}))
	require.NoError(t, repo.Create(ctx, &model.Notification{ID: "n3", UserID: "u1", Type: "follow", IsRead: false}))

	items, total, err := svc.ListFiltered(ctx, "u1", 1, 10, "comment", "unread")
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(1), total)
	require.Equal(t, "n1", items[0].ID)
}

func TestSubscribeNotifications_LikeUsesPostIDAsReference(t *testing.T) {
	repo := newMockNotificationRepo()
	svc, _ := newTestNotificationService(t, repo)
	// No-op email dispatcher: buffered channel, no workers (avoids nil userRepo panic).
	svc.emailDisp = &emailDispatcher{
		ch:   make(chan emailTask, 16),
		svc:  svc,
		quit: make(chan struct{}),
	}

	bus := testnats.NewEventBus(t)
	disp := SubscribeNotifications(bus, svc)
	t.Cleanup(func() { disp.Shutdown() })

	bus.Publish(eventbus.Event{
		Type: eventbus.EventContentLiked,
		Ctx:  context.Background(),
		Payload: map[string]any{
			"user_id":     "author-1",
			"sender_id":   "liker-1",
			"sender_name": "Liker",
			"content_id":  "content-1",
			"post_id":     "post-1",
		},
	})

	require.Eventually(t, func() bool {
		repo.mu.Lock()
		defer repo.mu.Unlock()
		return len(repo.items) == 1
	}, 2*time.Second, 10*time.Millisecond)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	var got *model.Notification
	for _, n := range repo.items {
		got = n
	}
	require.NotNil(t, got)
	assert.Equal(t, "like", got.Type)
	assert.Equal(t, "author-1", got.UserID)
	assert.Equal(t, "post-1", got.ReferenceID, "like notification deep-link must use post_id")
	assert.Equal(t, "liker-1", got.SenderID)
}
