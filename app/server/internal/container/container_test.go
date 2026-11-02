package container

import (
	"context"
	"sync"
	"testing"

	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg/testnats"
	"feedchamber/internal/repository"
	"feedchamber/internal/service"

	"github.com/stretchr/testify/require"
)

// recordingSessionStore is a service.SessionStore double that remembers the
// instance the container was handed, so the reuse assertion is meaningful.
type recordingSessionStore struct {
	mu     sync.Mutex
	issues int
}

func (r *recordingSessionStore) Issue(_ context.Context, u *model.User, ip, ua string) (*model.UserSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.issues++
	return &model.UserSession{SessionID: "sid", UserUUID: u.ID}, nil
}

func (r *recordingSessionStore) Rotate(_ context.Context, old *model.UserSession) (*model.UserSession, error) {
	return &model.UserSession{SessionID: "rotated"}, nil
}

func (r *recordingSessionStore) Revoke(_ context.Context, _ string) error           { return nil }
func (r *recordingSessionStore) RevokeAllForUser(_ context.Context, _ string) error { return nil }

// TestNew_ReusesInjectedServices is the M-25 regression guard. container.New
// receives pre-built SpaceService and the session store (server/iris.go hands
// the SAME store instance to the gateway middleware). The container must keep
// those instances — not silently build new ones — so the handler layer and the
// auth layer share one revocation authority. This matters most for Sessions: a
// forked view would let a revoked credential pass at one layer but not the
// other.
func TestNew_ReusesInjectedServices(t *testing.T) {
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: ":memory:"})
	require.NoError(t, err)

	kv := testnats.NewKVStore(t, "container_test")
	bus := testnats.NewEventBus(t)

	permSvc := service.NewPermissionService(
		repository.NewPermissionRepository(store),
		repository.NewSpaceRolePermissionRepository(store),
		repository.NewGroupRepository(store),
		kv,
	)
	settingSvc := service.NewSettingService(repository.NewSettingRepository(store), kv)
	bannedSvc := service.NewBannedService(repository.NewBannedRepository(store), kv, db.NewTransactionManager(store))
	spaceSvc := service.NewSpaceService(
		repository.NewSpaceRepository(store),
		repository.NewPostRepository(store),
		db.NewTransactionManager(store),
		kv,
	)
	sessions := &recordingSessionStore{}

	c := New(Deps{
		Config:     config.Config{},
		Store:      store,
		KV:         kv,
		EventBus:   bus,
		Permission: permSvc,
		Setting:    settingSvc,
		Banned:     bannedSvc,
		Space:      spaceSvc,
		Sessions:   sessions,
	})

	require.True(t, c.SpaceSvc == spaceSvc,
		"container must reuse the injected SpaceService, not construct its own")
	require.True(t, service.SessionStore(sessions) == c.SessionSvc,
		"container must reuse the injected session store, not construct its own")
	require.NotNil(t, c.Tx, "one shared transaction manager must be built")
}
