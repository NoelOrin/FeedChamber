package middleware

import (
	"net/http/httptest"
	"testing"

	"feedchamber/internal/pkg"

	"github.com/kataras/iris/v12"
	"github.com/stretchr/testify/require"
)

// runMaintenanceGate mounts the gate on an exact path with a given method and
// role, then reports the response. The handler behind it answers 200 when the
// gate lets the request through.
func runMaintenanceGate(t *testing.T, method, path, role string, settings *mockSettingReader) *httptest.ResponseRecorder {
	t.Helper()
	app := iris.New()
	handler := func(ctx iris.Context) {
		if role != "" {
			ctx.Values().Set(pkg.CtxRole, role)
		}
		MaintenanceGate(settings)(ctx)
		if !ctx.IsStopped() {
			ctx.StatusCode(iris.StatusOK)
		}
	}
	switch method {
	case "GET":
		app.Get(path, handler)
	default:
		app.Post(path, handler)
	}
	require.NoError(t, app.Build())
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

func maintenanceOn() *mockSettingReader {
	return &mockSettingReader{values: map[string]string{"site_maintenance": "true"}}
}

// `/notifications/stream` is a long-lived read (SSE) endpoint. Blocking it
// during maintenance silences the notification bell with no visible error:
// the SSE client only surfaces codes in (0, 5000) and 5005 is silently retried.
func TestMaintenanceGate_AllowsNotificationStream(t *testing.T) {
	w := runMaintenanceGate(t, "POST", "/notifications/stream", "user", maintenanceOn())
	require.Equal(t, iris.StatusOK, w.Code, "SSE stream must stay readable during maintenance")
}

// The same fix must not open a hole for the realtime stream either — it is the
// other /stream endpoint covered by the nginx SSE location.
func TestMaintenanceGate_AllowsRealtimeStream(t *testing.T) {
	w := runMaintenanceGate(t, "POST", "/realtime/stream", "user", maintenanceOn())
	require.Equal(t, iris.StatusOK, w.Code, "realtime SSE stream must stay readable during maintenance")
}

// Guard against solving the above by blanket-allowing every POST: a genuine
// mutation must still be rejected for non-admins.
func TestMaintenanceGate_StillBlocksMutations(t *testing.T) {
	w := runMaintenanceGate(t, "POST", "/posts/create", "user", maintenanceOn())
	require.Equal(t, iris.StatusServiceUnavailable, w.Code)
	require.Contains(t, w.Body.String(), `"code":5005`)
}
