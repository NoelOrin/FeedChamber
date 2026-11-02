package middleware

import (
	"strings"

	"feedchamber/internal/pkg"

	"github.com/kataras/iris/v12"
)

// MaintenanceGate blocks non-admin mutating traffic when site_maintenance=true.
// Prefer mounting after the gateway credential resolution so the admin role is available.
func MaintenanceGate(settings pkg.SettingReader) iris.Handler {
	return func(ctx iris.Context) {
		if settings == nil {
			ctx.Next()
			return
		}
		on := pkg.GetSettingOrDefault(ctx.Request().Context(), settings, "site_maintenance", "SITE_MAINTENANCE", "false")
		if on != "true" {
			ctx.Next()
			return
		}

		path := ctx.Path()
		if isMaintenanceAllowPath(path) {
			ctx.Next()
			return
		}

		role := ctx.Values().GetString(pkg.CtxRole)
		if role == "admin" {
			ctx.Next()
			return
		}

		// Read-only GET browsing remains available.
		if ctx.Method() == iris.MethodGet {
			ctx.Next()
			return
		}

		// RPC-style reads (POST .../list|search|...) should stay available.
		if isMaintenanceReadRPC(path) {
			ctx.Next()
			return
		}

		msg := pkg.GetSettingOrDefault(ctx.Request().Context(), settings, "site_maintenance_message", "", "站点维护中，请稍后再试")
		pkg.Fail(ctx, pkg.MAINTENANCE, msg)
	}
}

func isMaintenanceAllowPath(path string) bool {
	return strings.HasSuffix(path, "/health") ||
		strings.Contains(path, "/settings/public") ||
		strings.Contains(path, "/auth/login") ||
		strings.Contains(path, "/auth/register") ||
		strings.Contains(path, "/auth/refresh") ||
		strings.Contains(path, "/auth/forgot_password") ||
		strings.Contains(path, "/auth/reset_password_by_code") ||
		strings.HasPrefix(path, "/swagger") ||
		strings.Contains(path, "/.well-known/")
}

func isMaintenanceReadRPC(path string) bool {
	// Strict suffix allow-list for common read RPC endpoints.
	readSuffixes := []string{
		"/list",
		"/search",
		"/tree",
		"/published",
		"/detail",
		"/public",
		"/providers",
		"/config",
		"/configs",
		"/pending",
		"/profile",
		"/counts",
		"/history",
		"/batch", // posts/batch fetch only (admin batch paths end with /users|/posts|/comments)
		"/like/status",
		"/favorite/status",
		"/follow/status",
		"/banned/status",
		"/unread_count",
	}
	for _, suffix := range readSuffixes {
		if strings.HasSuffix(path, suffix) {
			// Keep admin mutation batch endpoints blocked for non-admin.
			if suffix == "/batch" && strings.Contains(path, "/admin/batch/") {
				return false
			}
			return true
		}
	}
	return false
}
