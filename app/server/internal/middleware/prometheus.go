package middleware

import (
	"strconv"
	"time"

	"orinforum/internal/metrics"

	"github.com/kataras/iris/v12"
)

// Prometheus returns Iris middleware that records request duration and count.
func Prometheus() iris.Handler {
	return func(ctx iris.Context) {
		start := time.Now()
		ctx.Next()
		elapsed := time.Since(start).Seconds()

		method := ctx.Method()
		path := normalizePath(ctx.Path())
		status := strconv.Itoa(ctx.GetStatusCode())

		metrics.RequestDuration.WithLabelValues(method, path, status).Observe(elapsed)
		metrics.RequestsTotal.WithLabelValues(method, path, status).Inc()

		// Request/response sizes — best-effort, do not panic on -1.
		if reqSize := ctx.Request().ContentLength; reqSize > 0 {
			metrics.RequestSizeBytes.Observe(float64(reqSize))
		}
		respSize := ctx.ResponseWriter().Written()
		if respSize > 0 {
			metrics.ResponseSizeBytes.Observe(float64(respSize))
		}
	}
}

// normalizePath collapses path segments that look like IDs so we don't
// get one time-series per resource instance. e.g. /api/v1/users/abc123 → /api/v1/users/:id
func normalizePath(path string) string {
	parts := splitPath(path)
	changed := false
	for i, p := range parts {
		if isMetricID(p) {
			parts[i] = ":id"
			changed = true
		}
	}
	if !changed {
		return path
	}
	result := ""
	for _, p := range parts {
		result += "/" + p
	}
	return result
}

// isMetricID collapses numeric resource IDs as well as longer ID-like segments,
// while preserving known short static path segments.
func isMetricID(s string) bool {
	if len(s) == 0 {
		return false
	}
	allDigits := true
	for _, c := range s {
		if c < '0' || c > '9' {
			allDigits = false
			break
		}
	}
	if allDigits && len(s) <= 20 {
		return true
	}
	return isLikelyID(s)
}

func splitPath(path string) []string {
	if len(path) == 0 {
		return nil
	}
	var parts []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}

// isLikelyID returns true for path segments that look like database IDs
// (hex strings, UUIDs, numeric IDs, CUIDs, etc.).
func isLikelyID(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Numeric ID
	if s[0] >= '0' && s[0] <= '9' {
		num := true
		for _, c := range s {
			if c < '0' || c > '9' {
				num = false
				break
			}
		}
		if num && len(s) >= 5 && len(s) <= 20 {
			return true
		}
	}
	// UUID / hex / CUID patterns — long alphanumeric with no word boundaries
	if len(s) >= 12 && len(s) <= 36 {
		hasDigit := false
		for _, c := range s {
			if c >= '0' && c <= '9' {
				hasDigit = true
			}
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || c == '-') {
				return false
			}
		}
		return hasDigit
	}
	return false
}
