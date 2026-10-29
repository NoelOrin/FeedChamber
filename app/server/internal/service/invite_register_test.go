package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"orinforum/internal/mock"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── test doubles ─────────────────────────────────────────────────────────────

// fakeSessionStore implements service.SessionStore: it mints sequential ids
// so register/login tests can assert an opaque credential was issued.
type fakeSessionStore struct {
	mu       sync.Mutex
	issued   []*model.UserSession
	revoked  []string
	revokedU []string
	next     int
}

func (f *fakeSessionStore) Issue(_ context.Context, user *model.User, ip, userAgent string) (*model.UserSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	sess := &model.UserSession{
		SessionID:  "fake-sid-" + string(rune('a'+f.next%26)),
		UserUUID:   user.ID,
		Username:   user.Username,
		Role:       user.Role,
		IP:         ip,
		UserAgent:  userAgent,
		DeviceName: userAgent,
	}
	f.issued = append(f.issued, sess)
	return sess, nil
}

func (f *fakeSessionStore) Rotate(_ context.Context, old *model.UserSession) (*model.UserSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	return &model.UserSession{SessionID: "rotated-sid", UserUUID: old.UserUUID}, nil
}

func (f *fakeSessionStore) Revoke(_ context.Context, sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revoked = append(f.revoked, sessionID)
	return nil
}

func (f *fakeSessionStore) RevokeAllForUser(_ context.Context, userUUID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revokedU = append(f.revokedU, userUUID)
	return nil
}

type inviteTestInviteRepo struct {
	mu     sync.Mutex
	byCode map[string]*model.Invite
	byID   map[string]*model.Invite
}

func newInviteTestInviteRepo() *inviteTestInviteRepo {
	return &inviteTestInviteRepo{byCode: map[string]*model.Invite{}, byID: map[string]*model.Invite{}}
}

func (m *inviteTestInviteRepo) Create(_ context.Context, inv *model.Invite) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *inv
	m.byCode[inv.Code] = &cp
	m.byID[inv.ID] = &cp
	return nil
}
func (m *inviteTestInviteRepo) FindByCode(_ context.Context, code string) (*model.Invite, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, ok := m.byCode[code]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *inv
	return &cp, nil
}
func (m *inviteTestInviteRepo) FindByID(_ context.Context, id string) (*model.Invite, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, ok := m.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *inv
	return &cp, nil
}
func (m *inviteTestInviteRepo) List(_ context.Context, offset, limit int) ([]model.Invite, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var all []model.Invite
	for _, inv := range m.byID {
		all = append(all, *inv)
	}
	total := int64(len(all))
	if offset > len(all) {
		return nil, total, nil
	}
	all = all[offset:]
	if limit < len(all) {
		all = all[:limit]
	}
	return all, total, nil
}
func (m *inviteTestInviteRepo) ConsumeByCode(_ context.Context, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, ok := m.byCode[code]
	if !ok {
		return repository.ErrNotFound
	}
	now := time.Now()
	if inv.ExpiresAt != nil && !inv.ExpiresAt.After(now) {
		return repository.ErrInviteExpired
	}
	if inv.MaxUses > 0 && inv.UsedCount >= inv.MaxUses {
		return repository.ErrInviteExhausted
	}
	inv.UsedCount++
	return nil
}
func (m *inviteTestInviteRepo) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, ok := m.byID[id]
	if !ok {
		return repository.ErrNotFound
	}
	delete(m.byID, id)
	delete(m.byCode, inv.Code)
	return nil
}

type inviteTestUserRepo struct {
	mu    sync.Mutex
	users map[string]*model.User
}

func newInviteTestUserRepo() *inviteTestUserRepo {
	return &inviteTestUserRepo{users: map[string]*model.User{}}
}

func (m *inviteTestUserRepo) Create(_ context.Context, u *model.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *u
	m.users[u.ID] = &cp
	return nil
}
func (m *inviteTestUserRepo) FindByID(_ context.Context, id string) (*model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *u
	return &cp, nil
}
func (m *inviteTestUserRepo) FindByEmail(_ context.Context, email string) (*model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Email == email && email != "" {
			cp := *u
			return &cp, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (m *inviteTestUserRepo) FindByUsername(_ context.Context, username string) (*model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Username == username && username != "" {
			cp := *u
			return &cp, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (m *inviteTestUserRepo) FindByIdentifier(ctx context.Context, id string) (*model.User, error) {
	if u, err := m.FindByUsername(ctx, id); err == nil {
		return u, nil
	}
	return m.FindByEmail(ctx, id)
}
func (m *inviteTestUserRepo) List(context.Context, int, int) ([]model.User, error) { return nil, nil }
func (m *inviteTestUserRepo) Search(context.Context, string, string, int, int) ([]model.User, error) {
	return nil, nil
}
func (m *inviteTestUserRepo) CountUsersByPermission(context.Context, string) (int64, error) {
	return 0, nil
}
func (m *inviteTestUserRepo) GetActivityStats(context.Context, string, int) (map[string]int, error) {
	return nil, nil
}
func (m *inviteTestUserRepo) CountPosts(context.Context, string) (int64, error) { return 0, nil }
func (m *inviteTestUserRepo) CountCommentsByUser(context.Context, string) (int64, error) {
	return 0, nil
}
func (m *inviteTestUserRepo) CountLikesByUser(context.Context, string) (int64, error) { return 0, nil }
func (m *inviteTestUserRepo) CountLikesReceived(context.Context, string) (int64, error) {
	return 0, nil
}
func (m *inviteTestUserRepo) IncrPoints(context.Context, string, int) error { return nil }
func (m *inviteTestUserRepo) ListByPoints(context.Context, int, int) ([]model.User, error) {
	return nil, nil
}
func (m *inviteTestUserRepo) ListByCheckIns(context.Context, int, int) ([]model.User, error) {
	return nil, nil
}
func (m *inviteTestUserRepo) ListByPostCount(context.Context, int, int) ([]model.User, error) {
	return nil, nil
}
func (m *inviteTestUserRepo) Count(context.Context) (int64, error)                    { return int64(len(m.users)), nil }
func (m *inviteTestUserRepo) CountDistinctPostAuthors(context.Context) (int64, error) { return 0, nil }
func (m *inviteTestUserRepo) Update(_ context.Context, u *model.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *u
	m.users[u.ID] = &cp
	return nil
}
func (m *inviteTestUserRepo) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.users, id)
	return nil
}

type inviteTestCredRepo struct {
	mu    sync.Mutex
	creds map[string]*model.Credential
}

func newInviteTestCredRepo() *inviteTestCredRepo {
	return &inviteTestCredRepo{creds: map[string]*model.Credential{}}
}

func (m *inviteTestCredRepo) Upsert(_ context.Context, c *model.Credential) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *c
	m.creds[c.UserID] = &cp
	return nil
}
func (m *inviteTestCredRepo) FindByUserID(_ context.Context, userID string) (*model.Credential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.creds[userID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *c
	return &cp, nil
}
func (m *inviteTestCredRepo) Delete(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.creds, userID)
	return nil
}

type inviteTestMetaRepo struct{}

func (m *inviteTestMetaRepo) Get(context.Context, string) (string, bool, error) {
	return "", false, nil
}
func (m *inviteTestMetaRepo) Set(context.Context, string, string) error { return nil }
func (m *inviteTestMetaRepo) Delete(context.Context, string) error      { return nil }

type inviteTestNopTx struct{}

func (inviteTestNopTx) WithTx(_ context.Context, fn func(context.Context) error) error {
	return fn(context.Background())
}

type inviteTestUserSvc struct {
	users *inviteTestUserRepo
}

func (s *inviteTestUserSvc) GetByID(ctx context.Context, id string) (*model.User, error) {
	return s.users.FindByID(ctx, id)
}
func (s *inviteTestUserSvc) GetByUsername(context.Context, string) (*model.User, error) {
	return nil, nil
}
func (s *inviteTestUserSvc) List(context.Context, int, int) ([]model.User, error) { return nil, nil }
func (s *inviteTestUserSvc) Search(context.Context, string, string, int, int) ([]model.User, error) {
	return nil, nil
}
func (s *inviteTestUserSvc) UpdateProfile(context.Context, string, *string, *string, *string, *string, *string, *string) error {
	return nil
}
func (s *inviteTestUserSvc) UpdateRole(ctx context.Context, id, role string) error {
	u, err := s.users.FindByID(ctx, id)
	if err != nil {
		return err
	}
	u.Role = role
	return s.users.Update(ctx, u)
}
func (s *inviteTestUserSvc) Delete(context.Context, string) error { return nil }
func (s *inviteTestUserSvc) GetPublicProfile(context.Context, string) (*model.PublicProfileResponse, error) {
	return nil, nil
}
func (s *inviteTestUserSvc) GetActivity(context.Context, string, int) (*model.ActivityResponse, error) {
	return nil, nil
}

func newTestAuth(mode string, invites *inviteTestInviteRepo, users *inviteTestUserRepo) *authService {
	if users == nil {
		users = newInviteTestUserRepo()
	}
	if invites == nil {
		invites = newInviteTestInviteRepo()
	}
	settings := mock.NewSettingService().
		WithSetting("register_enabled", "true").
		WithSetting("register_mode", mode).
		WithSetting("password_min_length", "8").
		WithSetting("bcrypt_cost", "10")
	return &authService{
		userRepo:   users,
		credRepo:   newInviteTestCredRepo(),
		metaRepo:   &inviteTestMetaRepo{},
		settingSvc: settings,
		inviteSvc:  NewInviteService(invites),
		userSvc:    &inviteTestUserSvc{users: users},
		txMgr:      inviteTestNopTx{},
		sessions:   &fakeSessionStore{},
	}
}

func TestInviteService_CreateAndConsume(t *testing.T) {
	repo := newInviteTestInviteRepo()
	svc := NewInviteService(repo)

	inv, err := svc.Create(context.Background(), "admin1", &CreateInviteRequest{MaxUses: 1})
	require.NoError(t, err)
	require.NotEmpty(t, inv.Code)
	assert.Equal(t, 1, inv.MaxUses)

	require.NoError(t, svc.ConsumeValid(context.Background(), inv.Code))
	// second use should fail
	err = svc.ConsumeValid(context.Background(), inv.Code)
	require.Error(t, err)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.INVALID_PARAMS, appErr.Code)
}

func TestInviteService_ConcurrentConsumeRespectsMaxUses(t *testing.T) {
	repo := newInviteTestInviteRepo()
	svc := NewInviteService(repo)
	inv, err := svc.Create(context.Background(), "admin1", &CreateInviteRequest{MaxUses: 1, Code: "ONCEONLY"})
	require.NoError(t, err)

	const workers = 20
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		success  int
		failures int
	)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			err := svc.ConsumeValid(context.Background(), inv.Code)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				success++
				return
			}
			failures++
			var appErr *pkg.AppError
			require.True(t, errors.As(err, &appErr))
			assert.Equal(t, pkg.INVALID_PARAMS, appErr.Code)
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, success)
	assert.Equal(t, workers-1, failures)

	got, err := repo.FindByCode(context.Background(), inv.Code)
	require.NoError(t, err)
	assert.Equal(t, 1, got.UsedCount)
}

func TestInviteService_Expired(t *testing.T) {
	repo := newInviteTestInviteRepo()
	svc := NewInviteService(repo)
	past := time.Now().Add(-time.Hour)
	inv, err := svc.Create(context.Background(), "admin1", &CreateInviteRequest{
		Code:      "EXPIREDCODE",
		MaxUses:   0,
		ExpiresAt: &past,
	})
	// Create rejects past expiry
	require.Error(t, err)
	assert.Nil(t, inv)

	// inject expired invite directly
	exp := time.Now().Add(-time.Minute)
	_ = repo.Create(context.Background(), model.NewInvite("OLDCODE", "admin1", 0, &exp))
	err = svc.ConsumeValid(context.Background(), "OLDCODE")
	require.Error(t, err)
}

func TestRegister_InviteMode_RequiresCode(t *testing.T) {
	svc := newTestAuth("invite", nil, nil)
	_, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "alice01",
		Email:    "alice01@example.com",
		Password: "password123",
	})
	require.Error(t, err)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Contains(t, appErr.Message, "邀请码")
}

func TestRegister_InviteMode_ConsumesCode(t *testing.T) {
	invites := newInviteTestInviteRepo()
	inviteSvc := NewInviteService(invites)
	inv, err := inviteSvc.Create(context.Background(), "admin1", &CreateInviteRequest{MaxUses: 1, Code: "HELLOINVITE"})
	require.NoError(t, err)

	svc := newTestAuth("invite", invites, nil)
	resp, err := svc.Register(context.Background(), &RegisterRequest{
		Username:   "bobuser",
		Email:      "bob@example.com",
		Password:   "password123",
		InviteCode: inv.Code,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "user", resp.User.Role)

	got, err := invites.FindByCode(context.Background(), inv.Code)
	require.NoError(t, err)
	assert.Equal(t, 1, got.UsedCount)
}

func TestRegister_ApproveMode_PendingUser(t *testing.T) {
	users := newInviteTestUserRepo()
	svc := newTestAuth("approve", nil, users)
	resp, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "pending1",
		Email:    "pending1@example.com",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.Equal(t, "pending_user", resp.User.Role)

	require.NoError(t, svc.ApproveRegistration(context.Background(), resp.User.ID))
	u, err := users.FindByID(context.Background(), resp.User.ID)
	require.NoError(t, err)
	assert.Equal(t, "user", u.Role)
}

func TestApproveRegistration_RejectsNonPending(t *testing.T) {
	users := newInviteTestUserRepo()
	_ = users.Create(context.Background(), &model.User{ID: "u1", Username: "x", Role: "user"})
	svc := newTestAuth("all", nil, users)
	err := svc.ApproveRegistration(context.Background(), "u1")
	require.Error(t, err)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.INVALID_PARAMS, appErr.Code)
}

func TestRegister_AllMode_StillWorks(t *testing.T) {
	svc := newTestAuth("all", nil, nil)
	resp, err := svc.Register(context.Background(), &RegisterRequest{
		Username: "charlie",
		Email:    "charlie@example.com",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.Equal(t, "user", resp.User.Role)
	assert.NotEmpty(t, resp.Token)
}
