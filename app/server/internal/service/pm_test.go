package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── in-memory conversation repo for PM service tests ──────────────────────

type memConversationRepo struct {
	mu            sync.Mutex
	conversations map[string]*model.Conversation
	members       map[string]map[string]*model.ConversationMember // convID -> userID -> member
	messages      map[string][]*model.Message                     // convID -> msgs
}

func newMemConversationRepo() *memConversationRepo {
	return &memConversationRepo{
		conversations: map[string]*model.Conversation{},
		members:       map[string]map[string]*model.ConversationMember{},
		messages:      map[string][]*model.Message{},
	}
}

func (r *memConversationRepo) CreateConversation(_ context.Context, c *model.Conversation, memberUserIDs []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c.ID == "" {
		c.ID = pkg.NewID()
	}
	cp := *c
	r.conversations[c.ID] = &cp
	m := map[string]*model.ConversationMember{}
	for _, uid := range memberUserIDs {
		m[uid] = model.NewConversationMember(c.ID, uid)
	}
	r.members[c.ID] = m
	return nil
}

func (r *memConversationRepo) FindByID(_ context.Context, id string) (*model.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.conversations[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *c
	return &cp, nil
}

func (r *memConversationRepo) FindDirect(_ context.Context, userA, userB string) (*model.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var oldest *model.Conversation
	for id, members := range r.members {
		if len(members) != 2 {
			continue
		}
		if _, okA := members[userA]; !okA {
			continue
		}
		if _, okB := members[userB]; !okB {
			continue
		}
		c := r.conversations[id]
		if oldest == nil || c.CreatedAt.Before(oldest.CreatedAt) {
			cp := *c
			oldest = &cp
		}
	}
	if oldest == nil {
		return nil, repository.ErrNotFound
	}
	return oldest, nil
}

func (r *memConversationRepo) ListForUser(_ context.Context, userID string, offset, limit int) ([]model.ConversationListItem, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var items []model.ConversationListItem
	for id, members := range r.members {
		if _, ok := members[userID]; !ok {
			continue
		}
		c := r.conversations[id]
		peerID := ""
		for uid := range members {
			if uid != userID {
				peerID = uid
				break
			}
		}
		var lastMsg string
		var lastAt *time.Time
		if msgs := r.messages[id]; len(msgs) > 0 {
			lastMsg = msgs[len(msgs)-1].Body
			t := msgs[len(msgs)-1].CreatedAt
			lastAt = &t
		}
		items = append(items, model.ConversationListItem{
			ID:            id,
			PeerID:        peerID,
			LastMessage:   lastMsg,
			LastMessageAt: lastAt,
			CreatedAt:     c.CreatedAt,
			UpdatedAt:     c.UpdatedAt,
		})
	}
	total := len(items)
	if offset > total {
		return []model.ConversationListItem{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return items[offset:end], total, nil
}

func (r *memConversationRepo) IsMember(_ context.Context, conversationID, userID string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	members, ok := r.members[conversationID]
	if !ok {
		return false, nil
	}
	_, ok = members[userID]
	return ok, nil
}

func (r *memConversationRepo) CreateMessage(_ context.Context, m *model.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m.ID == "" {
		m.ID = pkg.NewID()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	cp := *m
	r.messages[m.ConversationID] = append(r.messages[m.ConversationID], &cp)
	if c, ok := r.conversations[m.ConversationID]; ok {
		t := m.CreatedAt
		c.LastMessageAt = &t
		c.UpdatedAt = t
	}
	return nil
}

func (r *memConversationRepo) ListMessages(_ context.Context, conversationID string, offset, limit int) ([]model.Message, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	src := r.messages[conversationID]
	total := len(src)
	// Newest-first paging over chronological storage: offset 0 = latest window.
	if offset >= total || limit <= 0 {
		return []model.Message{}, total, nil
	}
	end := total - offset
	start := end - limit
	if start < 0 {
		start = 0
	}
	out := make([]model.Message, 0, end-start)
	// Return newest-first so service can reverse to chronological.
	for i := end - 1; i >= start; i-- {
		out = append(out, *src[i])
	}
	return out, total, nil
}

func (r *memConversationRepo) MarkRead(_ context.Context, conversationID, userID string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	members, ok := r.members[conversationID]
	if !ok {
		return repository.ErrNotFound
	}
	m, ok := members[userID]
	if !ok {
		return repository.ErrNotFound
	}
	m.LastReadAt = &at
	return nil
}

// ── user / notification stubs ─────────────────────────────────────────────

type memUserRepo struct {
	users map[string]*model.User
}

func newMemUserRepo(ids ...string) *memUserRepo {
	r := &memUserRepo{users: map[string]*model.User{}}
	for _, id := range ids {
		r.users[id] = &model.User{ID: id, Username: id, DisplayName: id}
	}
	return r
}

func (r *memUserRepo) FindByID(_ context.Context, id string) (*model.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *u
	return &cp, nil
}
func (r *memUserRepo) FindByEmail(context.Context, string) (*model.User, error) {
	return nil, repository.ErrNotFound
}
func (r *memUserRepo) FindByUsername(context.Context, string) (*model.User, error) {
	return nil, repository.ErrNotFound
}
func (r *memUserRepo) FindByIdentifier(context.Context, string) (*model.User, error) {
	return nil, repository.ErrNotFound
}
func (r *memUserRepo) List(context.Context, int, int) ([]model.User, error) { return nil, nil }
func (r *memUserRepo) Search(context.Context, string, string, int, int) ([]model.User, error) {
	return nil, nil
}
func (r *memUserRepo) CountUsersByPermission(context.Context, string) (int64, error) { return 0, nil }
func (r *memUserRepo) GetActivityStats(context.Context, string, int) (map[string]int, error) {
	return map[string]int{}, nil
}
func (r *memUserRepo) CountPosts(context.Context, string) (int64, error)            { return 0, nil }
func (r *memUserRepo) CountCommentsByUser(context.Context, string) (int64, error)   { return 0, nil }
func (r *memUserRepo) CountLikesByUser(context.Context, string) (int64, error)      { return 0, nil }
func (r *memUserRepo) CountLikesReceived(context.Context, string) (int64, error)    { return 0, nil }
func (r *memUserRepo) IncrPoints(context.Context, string, int) error                { return nil }
func (r *memUserRepo) ListByPoints(context.Context, int, int) ([]model.User, error) { return nil, nil }
func (r *memUserRepo) ListByCheckIns(context.Context, int, int) ([]model.User, error) {
	return nil, nil
}
func (r *memUserRepo) ListByPostCount(context.Context, int, int) ([]model.User, error) {
	return nil, nil
}
func (r *memUserRepo) Count(_ context.Context) (int64, error)                    { return 0, nil }
func (r *memUserRepo) CountDistinctPostAuthors(_ context.Context) (int64, error) { return 0, nil }
func (r *memUserRepo) Create(context.Context, *model.User) error                 { return nil }
func (r *memUserRepo) Update(context.Context, *model.User) error                 { return nil }
func (r *memUserRepo) Delete(context.Context, string) error                      { return nil }

type memNotifSvc struct {
	mu    sync.Mutex
	items []createdNotif
}

type createdNotif struct {
	userID, notifType, content, referenceID, senderID string
}

func (s *memNotifSvc) Create(_ context.Context, userID, notifType, content, referenceID, senderID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, createdNotif{userID, notifType, content, referenceID, senderID})
	return nil
}
func (s *memNotifSvc) List(context.Context, string, int, int) ([]model.Notification, error) {
	return nil, nil
}
func (s *memNotifSvc) ListFiltered(context.Context, string, int, int, string, string) ([]model.Notification, int64, error) {
	return nil, 0, nil
}
func (s *memNotifSvc) CountByUser(context.Context, string) (int64, error) { return 0, nil }
func (s *memNotifSvc) CountUnread(context.Context, string) (int64, error) { return 0, nil }
func (s *memNotifSvc) MarkRead(context.Context, string, string) error     { return nil }
func (s *memNotifSvc) MarkAllRead(context.Context, string) error          { return nil }
func (s *memNotifSvc) Delete(context.Context, string, string) error       { return nil }

func newTestPMService() (PMService, *memConversationRepo, *memNotifSvc) {
	repo := newMemConversationRepo()
	users := newMemUserRepo("user-a", "user-b", "user-c")
	notif := &memNotifSvc{}
	return NewPMService(repo, users, notif, nil), repo, notif
}

// ── tests ─────────────────────────────────────────────────────────────────

func TestPM_StartDirect_RejectSelf(t *testing.T) {
	svc, _, _ := newTestPMService()
	_, _, err := svc.StartDirect(context.Background(), "user-a", "user-a", "hi")
	require.Error(t, err)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.INVALID_PARAMS, appErr.Code)
}

func TestPM_StartDirect_Idempotent(t *testing.T) {
	svc, repo, notif := newTestPMService()
	ctx := context.Background()

	c1, m1, err := svc.StartDirect(ctx, "user-a", "user-b", "hello")
	require.NoError(t, err)
	require.NotNil(t, c1)
	require.NotNil(t, m1)
	assert.Equal(t, "hello", m1.Body)

	c2, m2, err := svc.StartDirect(ctx, "user-a", "user-b", "again")
	require.NoError(t, err)
	assert.Equal(t, c1.ID, c2.ID, "should reuse existing direct conversation")
	assert.Nil(t, m2, "existing conversation must not auto-send another message")

	// only the first bootstrap message exists
	msgs, total, err := repo.ListMessages(ctx, c1.ID, 0, 50)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, msgs, 1)
	assert.Equal(t, "hello", msgs[0].Body)

	// single notification for the bootstrap send
	notif.mu.Lock()
	defer notif.mu.Unlock()
	require.Len(t, notif.items, 1)
	assert.Equal(t, "user-b", notif.items[0].userID)
	assert.Equal(t, "pm", notif.items[0].notifType)
	assert.Equal(t, c1.ID, notif.items[0].referenceID)
	assert.Equal(t, "user-a", notif.items[0].senderID)
}

func TestPM_ListMessages_LatestWindow(t *testing.T) {
	svc, repo, _ := newTestPMService()
	ctx := context.Background()

	c := model.NewConversation()
	require.NoError(t, repo.CreateConversation(ctx, c, []string{"user-a", "user-b"}))

	// create 25 chronological messages
	for i := 1; i <= 25; i++ {
		msg := model.NewMessage(c.ID, "user-a", "m"+itoa(i))
		// force stable order via sequential timestamps
		msg.CreatedAt = time.Unix(int64(i), 0).UTC()
		require.NoError(t, repo.CreateMessage(ctx, msg))
	}

	// page 1 should be the latest 10 messages, chronological oldest→newest
	page1, total, err := svc.ListMessages(ctx, "user-a", c.ID, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 25, total)
	require.Len(t, page1, 10)
	assert.Equal(t, "m16", page1[0].Body)
	assert.Equal(t, "m25", page1[9].Body)

	// page 2 is the next older window
	page2, _, err := svc.ListMessages(ctx, "user-a", c.ID, 2, 10)
	require.NoError(t, err)
	require.Len(t, page2, 10)
	assert.Equal(t, "m6", page2[0].Body)
	assert.Equal(t, "m15", page2[9].Body)
}

// tiny helper to avoid strconv import churn in tests
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func TestPM_Send_NonMemberForbidden(t *testing.T) {
	svc, repo, _ := newTestPMService()
	ctx := context.Background()

	c := model.NewConversation()
	require.NoError(t, repo.CreateConversation(ctx, c, []string{"user-a", "user-b"}))

	_, err := svc.Send(ctx, "user-c", c.ID, "sneaky")
	require.Error(t, err)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.FORBIDDEN, appErr.Code)
}

func TestPM_ListMessages_NonMemberForbidden(t *testing.T) {
	svc, repo, _ := newTestPMService()
	ctx := context.Background()

	c := model.NewConversation()
	require.NoError(t, repo.CreateConversation(ctx, c, []string{"user-a", "user-b"}))

	_, _, err := svc.ListMessages(ctx, "user-c", c.ID, 1, 20)
	require.Error(t, err)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.FORBIDDEN, appErr.Code)
}

func TestPM_MarkRead_NonMemberForbidden(t *testing.T) {
	svc, repo, _ := newTestPMService()
	ctx := context.Background()

	c := model.NewConversation()
	require.NoError(t, repo.CreateConversation(ctx, c, []string{"user-a", "user-b"}))

	err := svc.MarkRead(ctx, "user-c", c.ID)
	require.Error(t, err)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.FORBIDDEN, appErr.Code)
}

func TestPM_Send_CreatesNotification(t *testing.T) {
	svc, repo, notif := newTestPMService()
	ctx := context.Background()

	c := model.NewConversation()
	require.NoError(t, repo.CreateConversation(ctx, c, []string{"user-a", "user-b"}))

	msg, err := svc.Send(ctx, "user-a", c.ID, "ping")
	require.NoError(t, err)
	assert.Equal(t, "ping", msg.Body)

	notif.mu.Lock()
	defer notif.mu.Unlock()
	require.Len(t, notif.items, 1)
	assert.Equal(t, "pm", notif.items[0].notifType)
	assert.Equal(t, c.ID, notif.items[0].referenceID)
	assert.Equal(t, "user-b", notif.items[0].userID)
}
