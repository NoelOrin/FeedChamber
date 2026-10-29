package pkg

import (
	"net"
	"strings"

	"github.com/kataras/iris/v12"
)

// ClientIP extracts the real client IP, respecting reverse proxy headers.
// It checks X-Forwarded-For (first entry) and X-Real-IP, falling back to
// RemoteAddr with the port stripped.
//
// Security note: these headers are only trustworthy when the server is behind
// a trusted reverse proxy, which is what `app.Configuration.RemoteAddrHeaders`
// is configured with at startup.
func ClientIP(ctx iris.Context) string {
	if fwd := ctx.GetHeader("X-Forwarded-For"); fwd != "" {
		// X-Forwarded-For: client, proxy1, proxy2 — first entry is the client.
		if idx := strings.Index(fwd, ","); idx > 0 {
			return strings.TrimSpace(fwd[:idx])
		}
		return strings.TrimSpace(fwd)
	}
	if realIP := ctx.GetHeader("X-Real-IP"); realIP != "" {
		return strings.TrimSpace(realIP)
	}
	// Strip port from RemoteAddr (e.g. "1.2.3.4:45678" → "1.2.3.4").
	ip, _, err := net.SplitHostPort(ctx.RemoteAddr())
	if err != nil {
		return ctx.RemoteAddr()
	}
	return ip
}
