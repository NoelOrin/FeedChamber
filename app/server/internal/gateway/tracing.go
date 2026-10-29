package gateway

import (
	"orinforum/internal/pkg/tracing"

	"github.com/kataras/iris/v12"
)

// Tracing names the request span once routing has resolved the endpoint.
type Tracing struct{}

// NewTracing creates a new Tracing instance.
func NewTracing() *Tracing {
	return &Tracing{}
}

// Middleware annotates the trace with the matched route template.
//
// The trace itself is established earlier by middleware.Trace, at app level and
// therefore before the request log is built; this hop runs after route matching,
// which is the first point where the endpoint name is known. It keeps the same
// span id (the ingress span is what X-Request-ID exposes) and does not call
// ctx.Next(), because the gateway pipeline owns the chain.
func (t *Tracing) Middleware(ctx iris.Context) {
	req := ctx.Request()
	tc, ok := tracing.TraceFromContext(req.Context())
	if !ok {
		return
	}
	route := ctx.GetCurrentRoute()
	if route == nil {
		return
	}
	name := route.Path()
	if name == "" || name == tc.Name {
		return
	}

	tc.Name = name
	ctx.ResetRequest(req.WithContext(tracing.ContextWithTrace(req.Context(), tc)))
}
