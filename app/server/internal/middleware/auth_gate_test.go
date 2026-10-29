package middleware

import (
	"context"
	"net/http/httptest"
	"testing"

	"feedchamber/internal/pkg"

	"github.com/kataras/iris/v12"
	"github.com/stretchr/testify/require"
)

// Tests for the gates that run *after* identity is known: AuthGate (dynamic
// login requirement from settings) and
// RequirePermission (role → permission code).
//
// Replaces the pre-session-auth auth_test.go, which exercised JWTAuth /
// OptionalJWTAuth / BanCheck / RequireRole / RequireOwnerOrPermission — all
// deleted in the opaque-session migration (see AGENTS.md "已删除且不要再引用").

// ── test doubles ────────────────────────────────────────────────────────────

// mockSettingReader is the SettingReader double for AuthGate. A missing key is
// reported as NOT_FOUND, which AuthGate treats as "gate disabled"
// (default-open) rather than a hard failure. calls counts cache misses.
type mockSettingReader struct {
	values map[string]string
	err    error
	calls  int
}

func (m *mockSettingReader) Get(_ context.Context, key string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	m.calls++
	if v, ok := m.values[key]; ok {
		return v, nil
	}
	return "", pkg.NewAppError(pkg.NOT_FOUND)
}

// runWithRole builds an app that sets the authenticated role (if any) and then
// invokes handler. When handler does not stop execution the route answers 200.
func runWithRole(t *testing.T, handler iris.Handler, role string) *httptest.ResponseRecorder {
	t.Helper()
	app := iris.New()
	app.Post("/x", func(ctx iris.Context) {
		if role != "" {
			ctx.Values().Set(pkg.CtxRole, role)
		}
		handler(ctx)
		if !ctx.IsStopped() {
			ctx.StatusCode(iris.StatusOK)
		}
	})
	require.NoError(t, app.Build())
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("POST", "/x", nil))
	return w
}

// ── RequirePermission ───────────────────────────────────────────────────────

func TestRequirePermission_NoRole(t *testing.T) {
	w := runWithRole(t, RequirePermission(Deps{Perms: &fakePerms{globalAllow: true}}, "post:create"), "")
	require.Equal(t, iris.StatusUnauthorized, w.Code)
}

func TestRequirePermission_Allowed(t *testing.T) {
	w := runWithRole(t, RequirePermission(Deps{Perms: &fakePerms{globalAllow: true}}, "post:create"), "user")
	require.Equal(t, iris.StatusOK, w.Code)
}

func TestRequirePermission_Denied(t *testing.T) {
	w := runWithRole(t, RequirePermission(Deps{Perms: &fakePerms{globalAllow: false}}, "post:create"), "user")
	require.Equal(t, iris.StatusForbidden, w.Code)
}

func TestRequirePermission_NilPermsDenied(t *testing.T) {
	w := runWithRole(t, RequirePermission(Deps{}, "post:create"), "user")
	require.Equal(t, iris.StatusForbidden, w.Code)
}

// ── AuthGate ────────────────────────────────────────────────────────────────

// runAuthGate registers AuthGate on POST /x. authenticated pre-sets the user id
// on the request context, mimicking what the gateway session resolution does.
func runAuthGate(t *testing.T, settings *mockSettingReader, authenticated bool) *httptest.ResponseRecorder {
	t.Helper()
	ResetAuthGateCache()
	app := iris.New()
	app.Post("/x", func(ctx iris.Context) {
		if authenticated {
			ctx.Values().Set(pkg.CtxUserID, "u1")
		}
		AuthGate(Deps{Settings: settings}, "gate.login")(ctx)
		if !ctx.IsStopped() {
			ctx.StatusCode(iris.StatusOK)
		}
	})
	require.NoError(t, app.Build())
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("POST", "/x", nil))
	return w
}

func TestAuthGate_DisabledPasses(t *testing.T) {
	w := runAuthGate(t, &mockSettingReader{values: map[string]string{"gate.login": "false"}}, false)
	require.Equal(t, iris.StatusOK, w.Code)
}

func TestAuthGate_EnabledAnonymousBlocked(t *testing.T) {
	w := runAuthGate(t, &mockSettingReader{values: map[string]string{"gate.login": "true"}}, false)
	require.Equal(t, iris.StatusUnauthorized, w.Code)
}

func TestAuthGate_EnabledAuthenticatedPasses(t *testing.T) {
	w := runAuthGate(t, &mockSettingReader{values: map[string]string{"gate.login": "true"}}, true)
	require.Equal(t, iris.StatusOK, w.Code)
}

func TestAuthGate_SettingMissingDefaultOpen(t *testing.T) {
	w := runAuthGate(t, &mockSettingReader{values: map[string]string{}}, false)
	require.Equal(t, iris.StatusOK, w.Code)
}

// A backend failure with no usable cache entry is fail-closed (500): we must
// not silently drop the login requirement when the settings store is down.
func TestAuthGate_KVFailureFailClosed(t *testing.T) {
	w := runAuthGate(t, &mockSettingReader{err: pkg.NewAppError(pkg.INTERNAL_ERROR)}, false)
	require.Equal(t, iris.StatusInternalServerError, w.Code)
}

func TestAuthGate_CacheHitAvoidsSettings(t *testing.T) {
	ResetAuthGateCache()
	settings := &mockSettingReader{values: map[string]string{"gate.login": "true"}}
	app := iris.New()
	app.Post("/x", func(ctx iris.Context) {
		ctx.Values().Set(pkg.CtxUserID, "u1")
		AuthGate(Deps{Settings: settings}, "gate.login")(ctx)
		if !ctx.IsStopped() {
			ctx.StatusCode(iris.StatusOK)
		}
	})
	require.NoError(t, app.Build())

	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("POST", "/x", nil))
	require.Equal(t, iris.StatusOK, w.Code)
	require.Equal(t, 1, settings.calls, "first request must read the setting")

	// Second request within the TTL must hit gateCache, not the settings reader.
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, httptest.NewRequest("POST", "/x", nil))
	require.Equal(t, iris.StatusOK, w2.Code)
	require.Equal(t, 1, settings.calls, "second request must be served from gateCache")
}
