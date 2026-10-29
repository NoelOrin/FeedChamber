package gateway

import (
	"net/http"
	"strings"

	"gorm.io/gorm"

	"orinforum/internal/pkg"
	"orinforum/internal/pkg/kvstore"

	"github.com/kataras/iris/v12"
)

// Deps 是网关层的全部依赖，由 server/iris.go 一次构造后复用给
// SessionStore / SecurityManager / Gateway 三个组件。
//
// 字段类型用的是 internal/pkg 里的接缝接口（SettingReader / PermissionChecker /
// BannedChecker），而不是本包自己再声明一套同形接口：那两个包此前各写一份，
// 名字差一个后缀、方法集还不同，读代码要靠猜，接错线只有运行期才暴露。
//
// User revocation 已不是依赖：解析不到活的 user_sessions 行就是未认证
// （见 SessionStore）。只有 OAuth client-credential 子系统还在用 KV 的 `bl:` 标记。
type Deps struct {
	SettingService pkg.SettingReader
	Perms          pkg.PermissionChecker
	Ban            pkg.BannedChecker
	// KV 是会话/限流/OAuth state 的缓存后端。此前声明为 any 并号称"避免直接
	// 依赖"，但同包的 ratelimit.go 与 security.go 一直直接 import kvstore；any 的
	// 实际后果是每个消费点做一次运行时断言，接错线时静默降级成"限流关闭 /
	// 无会话"，而不是编译报错。
	KV kvstore.Store
	DB *gorm.DB
}

// Gateway orchestrates auth, permission, rate limiting, tracing, OAuth, OAuth provider, SSO, API Key,
// Session, and Security management.
type Gateway struct {
	deps       Deps
	auth       *AuthChecker
	permission *PermissionCheckerMiddleware
	rateLimit  *RateLimiter
	tracing    *Tracing
	OAuth      *OAuthManager
	Provider   *OAuthProvider
	SSO        *SSOProvider
	APIKey     *APIKeyManager
	Session    *SessionManager
	Security   *SecurityManager
}

// New constructs a Gateway with the given dependencies.
func New(deps Deps) *Gateway {
	return &Gateway{
		deps:       deps,
		auth:       NewAuthChecker(deps),
		permission: NewPermissionChecker(deps),
		rateLimit:  NewRateLimiter(deps),
		tracing:    NewTracing(),
		OAuth:      NewOAuthManager(deps),
		Provider:   NewOAuthProvider(deps),
		SSO:        NewSSOProvider(deps),
		APIKey:     NewAPIKeyManager(deps),
		Session:    NewSessionManager(deps),
		Security:   NewSecurityManager(deps),
	}
}

// 阶段标记。iris 的 ctx values 是每请求一份，所以两个段各自幂等：
// 同一个请求无论被 Preflight+Enforce 分段挂载，还是被 Handler() 整体挂载，
// tracing / 全局限流 / 凭证解析 / 强制校验都只执行一次。
//
// 这不是防御性冗余，而是修掉一个真实缺陷：v1 层的 Preflight 对 protected 同样生效，
// 若 Handler() 再跑一遍 preflight，受保护请求会以同一个 "ratelimit:"+ip 键计数两次
// （实测 60/分钟的上限在第 30 次请求就返回 429），并多做一次会话查询与滑动续期。
const ctxGatewayStages = "gw_stages"

type stage uint8

const (
	stageTracing stage = 1 << iota
	stageRateLimit
	stageCredential
	stageEnforce
)

func (g *Gateway) ranStage(ctx iris.Context, s stage) bool {
	v, ok := ctx.Values().Get(ctxGatewayStages).(stage)
	return ok && v&s != 0
}

func (g *Gateway) markStage(ctx iris.Context, s stage) {
	v, _ := ctx.Values().Get(ctxGatewayStages).(stage)
	ctx.Values().Set(ctxGatewayStages, v|s)
}

// identityPresent reports whether a preflight already resolved this request's
// identity, so Enforce does not need to resolve it again (a session lookup plus
// a possible sliding renewal). API keys are never resolved by preflight, so an
// API-key request always takes the full resolve path in Enforce.
func identityPresent(ctx iris.Context) bool {
	return ctx.Values().Get(pkg.CtxSession) != nil ||
		ctx.Values().GetString(pkg.CtxAuthMethod) != ""
}

// preflight runs the stages that apply to every /api/v1 request: trace naming,
// the global per-IP ceiling, and *optional* credential resolution. `next` is
// called only when the request is allowed to continue.
func (g *Gateway) preflight(ctx iris.Context, next func()) {
	if !g.ranStage(ctx, stageTracing) {
		g.markStage(ctx, stageTracing)
		g.tracing.Middleware(ctx)
	}
	if !g.ranStage(ctx, stageRateLimit) {
		g.markStage(ctx, stageRateLimit)
		if !g.rateLimit.Middleware(ctx) {
			return
		}
	}
	if !g.ranStage(ctx, stageCredential) {
		g.markStage(ctx, stageCredential)
		g.auth.OptionalMiddleware(ctx)
	}
	next()
}

// enforce requires a live credential and applies ban / permission / risk /
// session-tracking policy. It assumes nothing about preflight having run: the
// two are independent, idempotent segments.
func (g *Gateway) enforce(ctx iris.Context, next func()) {
	if g.ranStage(ctx, stageEnforce) {
		next()
		return
	}
	g.markStage(ctx, stageEnforce)

	if !identityPresent(ctx) {
		// API key auth and session auth are mutually exclusive.
		if isAPIKeyRequest(ctx) {
			if !g.APIKey.Middleware(ctx) {
				return
			}
		} else if !g.auth.Middleware(ctx) {
			return
		}
	}

	// banCheck before permission: banned users get 404, not 403 — the status
	// is pinned so a probe cannot tell a ban from a missing resource, while
	// the envelope still carries the reason for the (already authenticated)
	// caller whose request was refused.
	if !g.banCheck(ctx) {
		pkg.FailStatus(ctx, http.StatusNotFound, pkg.USER_BANNED)
		return
	}
	if !g.permission.Middleware(ctx) {
		return
	}
	g.Security.SecurityCheck(ctx)
	g.Session.TrackingMiddleware(ctx)
	next()
}

// Preflight returns the middleware for every /api/v1 request: tracing, global
// rate limit, optional credential resolution. Mount it once on the version
// party; sibling parties MUST NOT mount gateway stages themselves (Iris copies
// the parent middleware slice, so a sibling `Use` writes into the parent's spare
// slot and rewrites neighbouring parties' chains — see internal/router/AGENTS.md).
func (g *Gateway) Preflight() iris.Handler {
	return func(ctx iris.Context) { g.preflight(ctx, ctx.Next) }
}

// Enforce returns the authorization pipeline: required credential, ban check,
// permission, risk control, session tracking. Mount it on the protected party.
func (g *Gateway) Enforce() iris.Handler {
	return func(ctx iris.Context) { g.enforce(ctx, ctx.Next) }
}

// isAPIKeyRequest reports whether the request carries API key credentials.
// API key auth and credential (session / opaque OAuth token) auth are mutually exclusive:
//   - X-API-Key header → API key (takes precedence over Authorization)
//   - Authorization: Bearer ak_<id> → API key
//   - Authorization: Bearer <other> or HttpOnly cookie → session sid / opaque OAuth token
//
// When both an API key header and a session cookie are present, the API key wins.
func isAPIKeyRequest(ctx iris.Context) bool {
	if ctx.GetHeader("X-API-Key") != "" {
		return true
	}
	authHeader := ctx.GetHeader("Authorization")
	return strings.HasPrefix(authHeader, "Bearer ak_")
}

// banCheck checks ban status at two independent layers:
//  1. Role-based account ban (role == "ban") — no tokens should be issued.
//  2. BannedService-level mute — per-user or per-space restriction.
func (g *Gateway) banCheck(ctx iris.Context) bool {
	role := ctx.Values().GetString(pkg.CtxRole)
	if role == "" {
		// 未登录，放行 —— 匿名路由自行决定
		return true
	}
	// 角色封号检查
	if role == "ban" {
		return false
	}
	// 禁言检查
	userUUID := ctx.Values().GetString(pkg.CtxUserID)
	if userUUID != "" && g.deps.Ban != nil && g.deps.Ban.IsBanned(ctx.Request().Context(), userUUID) {
		return false
	}
	return true
}
