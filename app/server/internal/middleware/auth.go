// Authentication itself lives in internal/gateway (session / API key resolution).
// This file holds the gates that run *after* identity is known: dynamic
// login-required switches driven by settings, and route-level permission checks.
package middleware

import (
	"sync"
	"time"

	"feedchamber/internal/pkg"

	"github.com/kataras/iris/v12"
)

// settingCache provides a short TTL cache for AuthGate setting values
// so that transient KV failures don't block all public routes.
type authGateCache struct {
	mu    sync.RWMutex
	store map[string]settingEntry
}

type settingEntry struct {
	val       string
	fetchedAt time.Time
}

const settingCacheTTL = 30 * time.Second

// gateCache is the process-level cache. Injecting into AuthGate would require
// changing the Deps interface; a package-level var is acceptable for a single-
// instance server.
var gateCache authGateCache

func init() {
	gateCache.store = make(map[string]settingEntry)
}

// ResetAuthGateCache clears the process-level gateCache.
// Intended for tests that need a clean cache state between runs.
func ResetAuthGateCache() {
	gateCache.mu.Lock()
	defer gateCache.mu.Unlock()
	gateCache.store = make(map[string]settingEntry)
}

// cachedSettingGet returns the setting value with a 30s TTL cache.
// Hits cache on fresh (≤30s) reads, skipping KV entirely.
// On KV error, returns the last cached value (if fresh enough) or empty string + error.
func cachedSettingGet(deps Deps, ctx iris.Context, key string) (string, error) {
	// Fast path: check TTL cache first
	gateCache.mu.RLock()
	entry, ok := gateCache.store[key]
	gateCache.mu.RUnlock()
	if ok && time.Since(entry.fetchedAt) < settingCacheTTL {
		return entry.val, nil
	}

	val, err := deps.Settings.Get(ctx.Request().Context(), key)
	if err == nil {
		gateCache.mu.Lock()
		gateCache.store[key] = settingEntry{val: val, fetchedAt: time.Now()}
		gateCache.mu.Unlock()
		return val, nil
	}

	// KV failed — try stale cache (allow up to 2× TTL degradation)
	gateCache.mu.RLock()
	entry, ok = gateCache.store[key]
	gateCache.mu.RUnlock()
	if ok && time.Since(entry.fetchedAt) < 2*settingCacheTTL {
		return entry.val, nil
	}

	// No usable cache — 设置项不存在等同于空值（默认关闭鉴权）
	if appErr, ok := err.(*pkg.AppError); ok && appErr.Code == pkg.NOT_FOUND {
		gateCache.mu.Lock()
		gateCache.store[key] = settingEntry{val: "", fetchedAt: time.Now()}
		gateCache.mu.Unlock()
		return "", nil
	}
	// 其他错误（KV 不可用等）仍 fail-closed
	return "", err
}

// AuthGate returns a handler that blocks unauthenticated requests
// when the given setting key is "true".
func AuthGate(deps Deps, settingKey string) iris.Handler {
	return func(ctx iris.Context) {
		if deps.Settings != nil {
			val, err := cachedSettingGet(deps, ctx, settingKey)
			if err != nil {
				// Fail-closed: no KV result and no fresh cache
				pkg.Fail(ctx, pkg.INTERNAL_ERROR)
				ctx.StopExecution()
				return
			}
			if val == "true" && ctx.Values().GetString(pkg.CtxUserID) == "" {
				pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
				ctx.StopExecution()
				return
			}
		}
		ctx.Next()
	}
}

// RequirePermission checks role-based permission using the injected PermissionChecker.
func RequirePermission(deps Deps, permCode string) iris.Handler {
	return func(ctx iris.Context) {
		role := ctx.Values().GetString(pkg.CtxRole)
		if role == "" {
			pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
			ctx.StopExecution()
			return
		}
		if deps.Perms == nil || !deps.Perms.HasPermission(role, permCode) {
			pkg.Fail(ctx, pkg.FORBIDDEN)
			ctx.StopExecution()
			return
		}
		ctx.Next()
	}
}
