// Package authz 是「一个凭证 + 一个权限码」的唯一判定点。
//
// 为什么需要它：同一份判定逻辑此前散在三处，且三处语义各不相同——
//
//	gateway      api_key = scope 且 角色；oauth_client = 只认 scope
//	middleware   RequirePermission 只查角色权限表（API Key 的 scope 被忽略）
//	internal/mcp api_key = scope 或 角色（**比网关宽一格**，等于 scope 形同虚设）
//
// 三处不一致的后果是真实的提权路径：admin 的 read-only API Key 能通过 /mcp
// 调用写工具，也能穿过 route 级 RequirePermission。现在它们都调 Allowed。
//
// 因此本包只做两件事：把粗粒度 scope 展开成权限码集合，以及按凭证来源选判据。
// 它不持有状态、不读配置、不依赖 service 层。
package authz

import (
	"sort"
	"strings"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
)

// scopeRegistry 把粗粒度 API Key scope 映射到精确权限码。
//
// 精确权限码也可以直接当 scope 用（例如 "post:read,report:manage"）。
//
// 注意：customize:* 刻意不并入任何粗粒度 scope。并入任意一项，都会让所有
// 存量持该 scope 的 Key 在部署瞬间获得向全站注入 JS 的能力——权限提升且无报错。
// internal/mcp/scope_test.go 用源码扫描钉住这条约束。
var scopeRegistry = map[string][]string{
	"read": {
		model.PermPostRead, model.PermCommentRead, model.PermUserRead,
		model.PermRoleRead, model.PermFollowRead, model.PermSecurityRead,
	},
	"write": {
		model.PermPostCreate, model.PermPostUpdate, model.PermPostDelete,
		model.PermCommentCreate, model.PermCommentUpdate, model.PermCommentDelete,
		model.PermTagCreate, model.PermTagUpdate, model.PermTagDelete,
		model.PermSpaceCreate, model.PermSpaceUpdate, model.PermSpaceDelete,
		model.PermReportCreate, model.PermFavoriteSelf, model.PermContentLike,
		model.PermFollowSelf, model.PermUploadSelf, model.PermNotificationSelf,
		model.PermPMSelf, model.PermCheckinSelf,
	},
	"admin": {
		model.PermRoleManage, model.PermSettingManage, model.PermReportManage,
		model.PermAnnouncementManage, model.PermBatchEdit, model.PermBannedManage,
		model.PermStatsRead, model.PermAdminSearch, model.PermAdminMetrics,
		model.PermReconcileCounters, model.PermUserUpdate, model.PermUserDelete,
		model.PermUserActivityRead,
	},
}

// RolePermissions 是判定所需的最小权限面（角色 → 权限码）。
//
// 刻意比 pkg.PermissionChecker 窄：本包只需要这一条查询，接口越小越容易
// 在测试与其它模块里复用。
type RolePermissions interface {
	HasPermission(role, code string) bool
}

// Credential 是一次请求解析出的凭证，字段与 gateway 写进 iris context 的值一致。
type Credential struct {
	// Method 取值 ""（会话）/ pkg.AuthMethodAPIKey / pkg.AuthMethodOAuthClient。
	Method string
	// Scopes 是 API Key 或 OAuth 客户端登记的 scope 串。
	Scopes string
	// Role 是凭证所属用户的角色；机器凭证也可能有（API Key 绑定了用户）。
	Role string
}

// Allowed 判定该凭证是否持有某个权限码。
//
// 语义（与各模块文档一致）：
//   - 会话：只看角色权限表；
//   - API Key：scope 与角色权限**同时**覆盖——scope 是凭证的上限；
//   - OAuth 客户端：只认 scope（客户端不是用户，没有角色）；
//   - 空权限码：登录即可，恒放行；
//   - 未知凭证来源：拒绝（新增来源必须显式实现，避免静默放行）。
func Allowed(cred Credential, perms RolePermissions, code string) bool {
	if code == "" {
		return true
	}
	switch cred.Method {
	case "":
		return roleHas(perms, cred.Role, code)
	case pkg.AuthMethodAPIKey:
		return ScopeHas(cred.Scopes, code) && roleHas(perms, cred.Role, code)
	case pkg.AuthMethodOAuthClient:
		return ScopeHas(cred.Scopes, code)
	default:
		return false
	}
}

// ScopeHas 判定权限码是否落在凭证的 scope 内（不看角色）。
//
// customize:publish 这类「只能由凭证显式授予」的权限码走这条路径。
func ScopeHas(raw, code string) bool {
	if code == "" {
		return true
	}
	_, ok := Expand(raw)[code]
	return ok
}

// Expand 把 scope 串展开成权限码集合。粗粒度 scope 按注册表展开，
// 其余按「精确码即 scope」原样收下。
func Expand(raw string) map[string]struct{} {
	out := make(map[string]struct{}, 8)
	for _, scope := range splitScopes(raw) {
		if mapped, ok := scopeRegistry[scope]; ok {
			for _, code := range mapped {
				out[code] = struct{}{}
			}
			continue
		}
		out[scope] = struct{}{}
	}
	return out
}

// ValidScopeString 校验 scope 串：每一项要么是注册表里的粗粒度 scope，
// 要么是带冒号的精确权限码。空串不合法（API Key 至少需要一个 scope）。
func ValidScopeString(raw string) bool {
	scopes := splitScopes(raw)
	if len(scopes) == 0 {
		return false
	}
	for _, scope := range scopes {
		if _, ok := scopeRegistry[scope]; ok {
			continue
		}
		if !strings.Contains(scope, ":") {
			return false
		}
	}
	return true
}

// KnownScopes 返回粗粒度 scope 名单（后台展示用），已排序。
func KnownScopes() []string {
	out := make([]string, 0, len(scopeRegistry))
	for name := range scopeRegistry {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func roleHas(perms RolePermissions, role, code string) bool {
	return perms != nil && role != "" && perms.HasPermission(role, code)
}

func splitScopes(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	out := make([]string, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// NormalizeList 去重并排序 scope 串，返回规范化后的切片。
//
// 存储用规范化结果：同一把 Key 的 scope 串在任何路径下都一样，
// 便于 dedupe 与审计对比。
func NormalizeList(raw string) []string {
	seen := make(map[string]struct{}, 4)
	out := make([]string, 0, 4)
	for _, s := range splitScopes(raw) {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// Normalize 返回规范化后的 scope 串。
func Normalize(raw string) string { return strings.Join(NormalizeList(raw), ",") }

// ValueGetter 是 iris.Values 的最小读接口（ctx.Values() 即满足）。
//
// 用接口而不是直接吃 iris 类型：本包因此不依赖 web 框架，测试与
// 其它适配层都能传入任何同名实现。
type ValueGetter interface {
	GetString(key string) string
}

// CredentialFrom 从请求上下文还原凭证。
//
// 两个 scope 键按凭证来源区分：API Key 读 api_key_scopes，OAuth 客户端读
// client_scopes。取错键的表现是「scope 为空 → 一律拒绝」，看起来像权限配置错了，
// 所以这段映射只留这一份实现。
func CredentialFrom(values ValueGetter) Credential {
	method := values.GetString(pkg.CtxAuthMethod)
	scopes := ""
	switch method {
	case pkg.AuthMethodAPIKey:
		scopes = values.GetString(pkg.CtxAPIKeyScopes)
	case pkg.AuthMethodOAuthClient:
		scopes = values.GetString(pkg.CtxClientScopes)
	}
	return Credential{
		Method: method,
		Scopes: scopes,
		Role:   values.GetString(pkg.CtxRole),
	}
}
