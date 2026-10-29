package gateway

import (
	"net/http"
	"testing"

	"feedchamber/internal/middleware"
	"feedchamber/internal/pkg/tracing"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// asChainMember mirrors how the gateway pipeline uses Tracing: the middleware is
// called directly and the pipeline itself calls ctx.Next() afterwards, so
// Tracing.Middleware deliberately does not advance the chain.
func asChainMember(tr *Tracing) iris.Handler {
	return func(ctx iris.Context) {
		tr.Middleware(ctx)
		ctx.Next()
	}
}

// TestTracingNamesSpanAfterRouteMatch covers the reason this hop exists: the
// route template is only known after matching, and the trace id must survive it.
func TestTracingNamesSpanAfterRouteMatch(t *testing.T) {
	var captured tracing.Context

	app := iris.New()
	app.Use(middleware.Trace(true))
	app.Get("/posts/{id}", asChainMember(NewTracing()), func(ctx iris.Context) {
		tc, ok := tracing.TraceFromContext(ctx.Request().Context())
		require.True(t, ok, "trace must reach the handler")
		captured = tc
		ctx.StatusCode(http.StatusOK)
	})

	e := httptest.New(t, app)
	// Registered on a party-less route, iris reports the template with the
	// /{param} placeholder rather than the concrete id.
	e.GET("/posts/abc123").
		Expect().Status(http.StatusOK).
		Header("X-Request-ID").NotEmpty()

	require.NotEmpty(t, captured.TraceIDHex())
	assert.Contains(t, captured.Name, "/posts/")
	assert.NotContains(t, captured.Name, "abc123", "span name must be the template, not the concrete path")
}

// TestTracingWithoutUpstreamTraceDoesNotBlock guards the no-op path: a route
// chain that has no trace in context must still reach the handler.
func TestTracingWithoutUpstreamTraceDoesNotBlock(t *testing.T) {
	ran := false
	app := iris.New()
	app.Get("/plain", asChainMember(NewTracing()), func(ctx iris.Context) {
		ran = true
		ctx.StatusCode(http.StatusOK)
	})

	e := httptest.New(t, app)
	e.GET("/plain").Expect().Status(http.StatusOK)

	assert.True(t, ran, "Tracing must not block the chain when no trace exists")
}
