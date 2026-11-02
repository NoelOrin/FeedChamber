package mcp

import (
	"context"
	"net/http"
	"strings"
	"time"

	"feedchamber/internal/authz"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/ratelimit"
)

// Enforcer 补齐 /mcp 因挂载在应用根而缺失的网关保护。
//
// 这是本设计里最容易做错、后果最严重的一处：/mcp 不在 gw.Enforce() 覆盖内，
// 少了这层，一个已被封禁或禁言的用户的 API Key 仍然能通过 MCP 发帖、
// 改站点配置。四项检查一个都不能省：
//
//  1. 凭证：解析并校验 API Key（含启用开关、过期、吊销）；
//  2. 封禁：全局禁言 + 封号角色；
//  3. 权限：工具声明的权限码；
//  4. 限流：按凭证的预算。
//
// 维护模式（站点维护中禁止非管理员写入）也在这里，因为网关的
// MaintenanceGate 同样只作用于 /api/v1。
type Enforcer struct {
	settings pkg.SettingReader
	perms    pkg.PermissionChecker
	banned   pkg.BannedChecker
	limiter  *ratelimit.Limiter
	db       APIKeyLookup
	verify   VerifyPassword
}

// APIKeyLookup 是 Enforcer 校验凭证所需的最小数据访问接缝。
//
// 不复用 gateway.APIKeyManager：那个 manager 直接写 iris 响应并 stop 执行，
// 与 MCP 的 JSON-RPC 错误形状不兼容。这里只取「查到 key + 查到用户」两件事，
// 校验语义（bcrypt / 过期 / 吊销）在 Enforcer 内保持一致。
type APIKeyLookup interface {
	// LookupAPIKey 按 key_id 查出 API Key 行；不存在返回 (nil, nil)。
	LookupAPIKey(ctx context.Context, keyID string) (*model.APIKey, error)
	// LookupUser 查用户（用于取角色与封禁状态）；不存在返回 (nil, nil)。
	LookupUser(ctx context.Context, userID string) (*model.User, error)
}

// VerifyPassword 是 bcrypt 校验的接缝，抽出来便于测试。
type VerifyPassword func(hash, plain string) bool

// EnforcerDeps 是 Enforcer 的依赖。
type EnforcerDeps struct {
	Settings pkg.SettingReader
	Perms    pkg.PermissionChecker
	Banned   pkg.BannedChecker
	Limiter  *ratelimit.Limiter
	Lookup   APIKeyLookup
	// Verify 默认用 bcrypt.CompareHashAndPassword；测试可替换。
	Verify VerifyPassword
}

// Identity 是一次成功鉴权的结果。
type Identity struct {
	UserID string
	Role   string
	Scopes string
	// CanPublish 由 Enforcer.CanPublishCustomize 计算后填入，供工具使用。
	CanPublish bool
	// bannedInWrite 表示该用户被禁言：读工具仍可调用，写工具必须拒绝。
	// 不导出——判定逻辑只在 Authorize 里，外部不该自行解读。
	bannedInWrite bool
}

func NewEnforcer(d EnforcerDeps) *Enforcer {
	return &Enforcer{
		settings: d.Settings,
		perms:    d.Perms,
		banned:   d.Banned,
		limiter:  d.Limiter,
		db:       d.Lookup,
		verify:   d.Verify,
	}
}

// Enabled 表示 MCP Server 总开关是否打开。关闭时一律拒绝。
//
// 同时要求 API Key 认证开着：站长的「关掉 API Key」是凭证层面的止血开关，
// 若 /mcp 不看它，泄露的 Key 就仍能在这条通道上使用——这条通道没有会话
// 鉴权兜底，只能靠这里。
func (e *Enforcer) Enabled(ctx context.Context) bool {
	if e.settings == nil {
		return false
	}
	mcpOn, err := e.settings.Get(ctx, model.SettingMCPEnabled)
	if err != nil || mcpOn != "true" {
		return false
	}
	apiKeyOn, err := e.settings.Get(ctx, model.SettingAPIKeyEnabled)
	if err != nil {
		return false
	}
	return apiKeyOn == "true"
}

// Authenticate 解析并校验 API Key，返回调用者身份。
//
// 返回的 *Error 已经是可以直接回给客户端（经 responseSet 转成 JSON-RPC error）
// 的协议级错误，调用方不需要再包一层。
func (e *Enforcer) Authenticate(ctx context.Context, header http.Header) (*Identity, *Error) {
	if !e.Enabled(ctx) {
		return nil, newError(CodeForbidden, "MCP 服务未启用")
	}
	if e.db == nil {
		return nil, newError(CodeInternalError, "凭据校验不可用")
	}

	key := extractAPIKey(header)
	if key == "" {
		return nil, newError(CodeForbidden, "缺少 API Key")
	}
	if !strings.HasPrefix(key, "ak_") {
		return nil, newError(CodeForbidden, "API Key 格式错误")
	}
	keyID := key[3:]

	apiKey, err := e.db.LookupAPIKey(ctx, keyID)
	if err != nil {
		return nil, newError(CodeInternalError, "查询 API Key 失败")
	}
	if apiKey == nil {
		return nil, newError(CodeForbidden, "API Key 无效")
	}
	// 密码哈希校验：默认走 bcrypt（与 gateway 同语义），测试可注入替身
	// 以免每个用例都付一次 bcrypt 成本。
	verify := e.verify
	if verify == nil {
		verify = defaultVerify
	}
	if !verify(apiKey.SecretHash, key) {
		return nil, newError(CodeForbidden, "API Key 无效")
	}
	if apiKey.ExpiresAt != nil && time.Now().Unix() > *apiKey.ExpiresAt {
		return nil, newError(CodeForbidden, "API Key 已过期")
	}
	if apiKey.Status == "revoked" {
		return nil, newError(CodeForbidden, "API Key 已被吊销")
	}

	user, err := e.db.LookupUser(ctx, apiKey.UserUUID)
	if err != nil {
		return nil, newError(CodeInternalError, "查询用户失败")
	}
	if user == nil {
		return nil, newError(CodeForbidden, "凭证所属用户不存在")
	}

	ident := &Identity{
		UserID: user.ID,
		Role:   user.Role,
		Scopes: apiKey.Scopes,
	}

	// ── ban 检查（与网关 banCheck 等价） ──
	// 封号角色直接拒；禁言表命中则拒写。这里在「认证」阶段就拦住整类操作，
	// 因为 MCP 没有「只读请求放行」的分层——工具级判定在 Authorize 里。
	if user.Role == "ban" {
		return nil, newError(CodeForbidden, "账号已被封禁")
	}
	if e.banned != nil && e.banned.IsBanned(ctx, user.ID) {
		// 禁言不阻断读工具，标记在 Authorize 里按工具类型判定。
		ident.bannedInWrite = true
	}

	// ── 限流 ──
	if !e.allow(ctx, apiKey) {
		return nil, newError(CodeRateLimited, "超出限流预算")
	}

	return ident, nil
}

// Authorize 校验一次具体工具调用是否被允许。
//
// 顺序：维护模式 → 禁言（写类）→ 权限码。任一项失败都返回协议级错误。
func (e *Enforcer) Authorize(ctx context.Context, ident *Identity, tool Tool) *Error {
	// ── 维护模式 ──
	// 与 middleware.MaintenanceGate 同语义：维护中只放管理员，且只挡写。
	if tool.Write && ident.Role != "admin" && e.maintenanceOn(ctx) {
		return newError(CodeForbidden, "站点维护中，暂不接受写入")
	}
	// ── 禁言 ──
	if tool.Write && ident.bannedInWrite {
		return newError(CodeForbidden, "账号已被禁言，无法执行写操作")
	}
	// ── 权限 ──
	if tool.Perm != "" {
		if !e.hasPerm(ident, tool.Perm) {
			return newError(CodeForbidden, "权限不足: "+tool.Perm)
		}
	}
	return nil
}

// hasPerm 判定身份是否持有某权限码。
//
// API Key 的 scope 优先：key 上的 scope → 权限码映射由 gateway 的
// apiKeyScopeRegistry 承担，这里只做「精确码是否在 scope 里」与
// 「角色权限表是否授予」两件事。精确码可以当 scope 直接用（与 gateway 一致）。
func (e *Enforcer) hasPerm(ident *Identity, code string) bool {
	// customize:publish 只能由 API Key 的显式 scope 授予，**不走角色继承**。
	//
	// 理由：admin 角色默认持有 customize:publish（它本来就能在后台改这些配置），
	// 若这里也接受角色继承，那么「admin 创建的任意 API Key」都会自动具备
	// 让 JS 直接生效的能力——草稿保护对最需要它的那批凭证完全失效。
	// 规范里「凭证额外持有 publish 权限」的「额外」必须落在凭证本身，
	// 否则这条分级策略就只是纸面上的。
	if code == model.PermCustomizePublish {
		return authz.ScopeHas(ident.Scopes, code)
	}

	// 其余权限码与网关同一判据：scope 与角色同时覆盖（scope 是凭证上限）。
	//
	// 这里曾经是「scope 或角色」，比网关宽一格：admin 的 read-only Key 只要
	// 角色表里有 customize:write 就能调 upsert_snippet。判据现在只在
	// internal/authz.Allowed 里实现一次。
	return authz.Allowed(authz.Credential{
		Method: pkg.AuthMethodAPIKey,
		Scopes: ident.Scopes,
		Role:   ident.Role,
	}, e.perms, code)
}

// CanPublishCustomize 判定该凭证是否能让高风险自定义内容直接生效。
func (e *Enforcer) CanPublishCustomize(ident *Identity) bool {
	return e.hasPerm(ident, model.PermCustomizePublish)
}

func (e *Enforcer) maintenanceOn(ctx context.Context) bool {
	if e.settings == nil {
		return false
	}
	v, err := e.settings.Get(ctx, "site_maintenance")
	if err != nil {
		return false
	}
	return v == "true"
}

// allow 执行限流检查。KV 不可用时引擎 fail-open，这里不额外收紧：
// 与全站限流语义保持一致，避免 MCP 成为唯一的硬失败点。
func (e *Enforcer) allow(ctx context.Context, key *model.APIKey) bool {
	if e.limiter == nil {
		return true
	}
	limit := key.RateLimit
	if limit <= 0 {
		limit = 120
	}
	outcome := e.limiter.Check(ctx, "mcp", ratelimit.GlobalKey("ak:"+key.KeyID), limit, time.Minute)
	return outcome != ratelimit.Denied
}

// extractAPIKey 取 X-API-Key 或 Authorization: Bearer。
func extractAPIKey(h http.Header) string {
	if v := strings.TrimSpace(h.Get("X-API-Key")); v != "" {
		return v
	}
	auth := strings.TrimSpace(h.Get("Authorization"))
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func splitScopes(raw string) []string {
	out := make([]string, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}
