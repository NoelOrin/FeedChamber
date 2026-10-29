package gateway

import (
	"strings"
	"sync"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"

	"github.com/kataras/iris/v12"
	"github.com/sirupsen/logrus"
)

// PermissionCheckerMiddleware checks permissions for protected routes.
type PermissionCheckerMiddleware struct {
	deps          Deps
	exactMap      map[string]string   // exact path -> permCode
	prefixMap     map[string]string   // path prefix -> permCode
	patterns      []patternEntry      // parameterized path -> permCode
	spacePatterns []spacePatternEntry // parameterized path with space_id -> permCode
	warnOnceMap   sync.Map            // path -> bool, dedup unmapped route warnings
}

type patternEntry struct {
	pattern  string
	parts    []string
	permCode string
}

type spacePatternEntry struct {
	pattern  string
	parts    []string
	permCode string
}

// NewPermissionChecker creates a new PermissionCheckerMiddleware instance.
func NewPermissionChecker(deps Deps) *PermissionCheckerMiddleware {
	p := &PermissionCheckerMiddleware{
		deps:      deps,
		exactMap:  make(map[string]string),
		prefixMap: make(map[string]string),
	}
	p.initRouteMap()
	return p
}

func (p *PermissionCheckerMiddleware) addExact(path, permCode string) {
	p.exactMap[path] = permCode
}

func (p *PermissionCheckerMiddleware) addPrefix(prefix, permCode string) {
	p.prefixMap[prefix] = permCode
}

func (p *PermissionCheckerMiddleware) addPattern(pattern, permCode string) {
	p.patterns = append(p.patterns, patternEntry{
		pattern:  pattern,
		parts:    strings.Split(pattern, "/"),
		permCode: permCode,
	})
}

func (p *PermissionCheckerMiddleware) addSpacePattern(pattern, permCode string) {
	p.spacePatterns = append(p.spacePatterns, spacePatternEntry{
		pattern:  pattern,
		parts:    strings.Split(pattern, "/"),
		permCode: permCode,
	})
}

func isParam(part string) bool {
	return len(part) > 2 && part[0] == '{' && part[len(part)-1] == '}'
}

func matchRoute(patternParts []string, path string) bool {
	pathParts := strings.Split(path, "/")
	if len(patternParts) != len(pathParts) {
		return false
	}
	for i, pp := range patternParts {
		if isParam(pp) {
			continue
		}
		if pp != pathParts[i] {
			return false
		}
	}
	return true
}

func (p *PermissionCheckerMiddleware) checkPermission(ctx iris.Context, permCode string) bool {
	return p.checkPermissionInSpace(ctx, "", permCode)
}

func (p *PermissionCheckerMiddleware) checkPermissionInSpace(ctx iris.Context, spaceID, permCode string) bool {
	role := ctx.Values().GetString(pkg.CtxRole)
	if role == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return false
	}
	if ctx.Values().GetString(pkg.CtxAuthMethod) == "api_key" {
		if !apiKeyHasPermission(ctx.Values().GetString(pkg.CtxAPIKeyScopes), permCode) {
			pkg.Fail(ctx, pkg.FORBIDDEN, "API Key 权限范围不足")
			return false
		}
	}
	if ctx.Values().GetString(pkg.CtxAuthMethod) == "oauth_client" {
		if !apiKeyHasPermission(ctx.Values().GetString(pkg.CtxClientScopes), permCode) {
			pkg.Fail(ctx, pkg.FORBIDDEN, "OAuth 客户端权限范围不足")
			return false
		}
		return true
	}
	var hasPermission bool
	if spaceID != "" {
		hasPermission = p.deps.Perms.HasPermissionInSpace(role, spaceID, permCode)
	} else {
		hasPermission = p.deps.Perms.HasPermission(role, permCode)
	}
	if !hasPermission {
		pkg.Fail(ctx, pkg.FORBIDDEN)
		return false
	}
	return true
}

// warnUnmappedOnce returns true the first time a given path is seen,
// then false for subsequent calls — dedupes log noise from unmapped routes.
func (p *PermissionCheckerMiddleware) warnUnmappedOnce(path string) bool {
	_, loaded := p.warnOnceMap.LoadOrStore(path, true)
	return !loaded
}

// Middleware checks the current request path against the route-permission map.
// Returns true if the request should continue, false if stopped (401/403).
func (p *PermissionCheckerMiddleware) Middleware(ctx iris.Context) bool {
	path := ctx.Path()

	// 1. Exact path match
	if permCode, ok := p.exactMap[path]; ok {
		return p.checkPermission(ctx, permCode)
	}

	// 2. Prefix match (grouped routes)
	for prefix, permCode := range p.prefixMap {
		if strings.HasPrefix(path, prefix) {
			return p.checkPermission(ctx, permCode)
		}
	}

	// 3. Space pattern match (extract space_id for HasPermissionInSpace)
	for _, spe := range p.spacePatterns {
		if matchRoute(spe.parts, path) {
			// 从路径提取 space_id
			spaceID := extractPathParam(spe.parts, path, "space_id")
			return p.checkPermissionInSpace(ctx, spaceID, spe.permCode)
		}
	}

	// 4. Pattern match (parameterized routes)
	for _, pe := range p.patterns {
		if matchRoute(pe.parts, path) {
			return p.checkPermission(ctx, pe.permCode)
		}
	}

	// No permission mapping — deny by default.
	// Any new protected route MUST be registered in initRouteMap.
	// This prevents accidental passthrough of sensitive endpoints.
	if p.warnUnmappedOnce(path) {
		logrus.Debugf("permission: unmapped protected route %s denied for role %q", path, ctx.Values().GetString(pkg.CtxRole))
	}
	pkg.Fail(ctx, pkg.FORBIDDEN)
	return false
}

// extractPathParam 从匹配的路径中提取指定参数段的值
func extractPathParam(patternParts []string, path, paramName string) string {
	pathParts := strings.Split(path, "/")
	for i, pp := range patternParts {
		if len(pp) > 2 && pp[0] == '{' && pp[len(pp)-1] == '}' {
			inner := pp[1 : len(pp)-1]
			if inner == paramName {
				if i < len(pathParts) {
					return pathParts[i]
				}
			}
		}
	}
	return ""
}

func (p *PermissionCheckerMiddleware) initRouteMap() {
	// ── Posts ──────────────────────────────────────────────
	p.addExact("/api/v1/posts/create", "post:create")
	p.addExact("/api/v1/posts/batch", "post:read")
	p.addPattern("/api/v1/posts/{id}/update", "post:update")
	p.addPattern("/api/v1/posts/{id}/delete", "post:delete")
	p.addPattern("/api/v1/posts/{id}/pin", "post:pin")
	p.addPattern("/api/v1/posts/{id}/lock", "post:lock")
	p.addPattern("/api/v1/posts/{id}/close", "post:create")
	// 状态直改（后台「通过 / 拒绝」）；与审核队列同权限域。
	p.addPattern("/api/v1/posts/{id}/status", "report:manage")
	p.addPattern("/api/v1/posts/{id}/feature", "report:manage")
	p.addPattern("/api/v1/posts/{id}/read", "post:read")
	p.addPattern("/api/v1/posts/{id}/pin/set", "post:pin")
	p.addPattern("/api/v1/posts/{id}/pin/remove", "post:pin")
	p.addExact("/api/v1/posts/pinned/list", "post:read")

	p.addPattern("/api/v1/posts/{postId}/tags/set", "post:create")

	// ── Comments ──────────────────────────────────────────
	p.addPattern("/api/v1/posts/{postId}/contents/create", "comment:create")
	p.addPattern("/api/v1/posts/{postId}/contents/{id}/update", "comment:update")
	p.addPattern("/api/v1/posts/{postId}/contents/{id}/delete", "comment:delete")
	p.addExact("/api/v1/posts/comments/list", "comment:delete")
	p.addPattern("/api/v1/comments/{id}", "comment:delete")
	p.addPattern("/api/v1/comments/{id}/delete", "comment:delete")

	// ── Users ─────────────────────────────────────────────
	p.addExact("/api/v1/user/search", "user:read")
	p.addExact("/api/v1/user/list", "user:read")
	p.addPattern("/api/v1/user/{id}", "user:read")
	p.addPattern("/api/v1/user/{id}/delete", "user:delete")
	p.addPattern("/api/v1/user/{id}/role/update", "user:update")
	p.addPattern("/api/v1/user/{id}/trust_level/update", "user:update")
	p.addPattern("/api/v1/user/{id}/banned", "user:update")
	p.addPattern("/api/v1/user/{id}/unbanned", "user:update")
	p.addPattern("/api/v1/user/{id}/banned/status", "user:update")
	p.addPattern("/api/v1/user/{id}/banned/overview", "user:update")
	p.addPattern("/api/v1/user/{id}/activity", "user:activity:read")
	p.addSpacePattern("/api/v1/user/{id}/space/{space_id}/banned", "user:update")
	p.addSpacePattern("/api/v1/user/{id}/space/{space_id}/unbanned", "user:update")
	p.addSpacePattern("/api/v1/user/{id}/space/{space_id}/banned/status", "user:update")

	// ── Role ──────────────────────────────────────────────
	p.addExact("/api/v1/role/list", "role:read")
	p.addExact("/api/v1/role/create", "role:manage")
	p.addPattern("/api/v1/role/{id}/update", "role:manage")
	p.addPattern("/api/v1/role/{id}/delete", "role:manage")

	// ── Permission ────────────────────────────────────────
	p.addExact("/api/v1/permission/list", "role:read")
	p.addExact("/api/v1/permission/role", "role:read")
	p.addExact("/api/v1/permission/sync", "role:manage")
	p.addExact("/api/v1/permission/space", "role:read")
	p.addExact("/api/v1/permission/space/sync", "role:manage")
	p.addExact("/api/v1/permission/space/clear", "role:manage")

	// ── Groups ────────────────────────────────────────────
	p.addExact("/api/v1/groups/list", "role:read")
	p.addExact("/api/v1/groups/create", "role:manage")
	p.addPattern("/api/v1/groups/{id}/update", "role:manage")
	p.addPattern("/api/v1/groups/{id}/delete", "role:manage")
	p.addPattern("/api/v1/groups/{id}/members/list", "role:read")
	p.addPattern("/api/v1/groups/{id}/members/add", "role:manage")
	p.addPattern("/api/v1/groups/{id}/members/remove", "role:manage")
	p.addPattern("/api/v1/groups/{id}/permissions", "role:read")
	p.addPattern("/api/v1/groups/{id}/permissions/sync", "role:manage")

	// ── Tags (protected CRUD) ─────────────────────────────
	p.addExact("/api/v1/tags/create", "tag:create")
	p.addPattern("/api/v1/tags/{id}/update", "tag:update")
	p.addPattern("/api/v1/tags/{id}/delete", "tag:delete")

	// ── Reports ───────────────────────────────────────────
	p.addExact("/api/v1/reports/create", "report:create")
	p.addExact("/api/v1/reports/list", "report:manage")
	p.addPattern("/api/v1/reports/{id}", "report:manage")
	p.addPattern("/api/v1/reports/{id}/status", "report:manage")
	p.addPattern("/api/v1/reports/{id}/delete", "report:manage")

	// ── Motions (社区自治动议) ─────────────────────────────
	p.addExact("/api/v1/motions/create", "motion:create")
	p.addExact("/api/v1/motions/list", "motion:create")
	p.addPattern("/api/v1/motions/{id}", "motion:create")
	p.addPattern("/api/v1/motions/{id}/vote", "motion:create")
	p.addPattern("/api/v1/motions/{id}/resolve", "motion:manage")
	p.addPattern("/api/v1/motions/{id}/status", "motion:manage")
	p.addPattern("/api/v1/motions/{id}/delete", "motion:manage")

	// ── Spaces ────────────────────────────────────────────
	p.addExact("/api/v1/spaces/create", "space:create")
	p.addSpacePattern("/api/v1/spaces/{space_id}/update", "space:update")
	p.addSpacePattern("/api/v1/spaces/{space_id}/delete", "space:delete") // delete 用全局权限
	p.addSpacePattern("/api/v1/spaces/{space_id}/status", "space:update")

	// ── Admin grouped routes (prefix matches) ────────────
	p.addPrefix("/api/v1/admin/announcements/", "announcement:manage")
	p.addPrefix("/api/v1/admin/invites/", "user:update")
	p.addExact("/api/v1/admin/invites/list", "user:update")
	p.addExact("/api/v1/admin/invites/create", "user:update")
	p.addPattern("/api/v1/admin/users/{id}/approve_registration", "user:update")
	p.addPrefix("/api/v1/admin/settings/", "setting:manage")
	p.addExact("/api/v1/admin/settings", "setting:manage")
	p.addPrefix("/api/v1/admin/batch/", "batch:edit")
	p.addPrefix("/api/v1/admin/moderation/", "report:manage")
	p.addPrefix("/api/v1/admin/webhooks/", model.PermRoleManage)

	// ── Admin exact / pattern ─────────────────────────────
	p.addExact("/api/v1/admin/stats", "stats:read")
	p.addExact("/api/v1/admin/audit_logs/list", "stats:read")
	p.addExact("/api/v1/admin/dashboard", "stats:read")
	p.addExact("/api/v1/admin/search", "admin:search")
	p.addPattern("/api/v1/admin/user/{id}/activities", "user:activity:read")
	p.addExact("/api/v1/admin/reconcile/counters", "reconcile:counters")
	// XP / 等级体系（P1）：配置面与用户 XP 详情都挂 level:manage。
	p.addPrefix("/api/v1/admin/levels/", model.PermLevelManage)
	p.addPrefix("/api/v1/admin/gates/", model.PermGateManage)
	p.addPrefix("/api/v1/admin/xp-rules/", model.PermLevelManage)
	p.addPrefix("/api/v1/admin/xp-adjustments/", model.PermLevelManage)
	p.addExact("/api/v1/admin/trust/recompute", model.PermLevelManage)
	p.addPattern("/api/v1/admin/users/{id}/xp", model.PermLevelManage)
	p.addPattern("/api/v1/admin/users/{id}/xp/adjust", model.PermLevelManage)
	p.addPattern("/api/v1/admin/users/{id}/xp/adjustments/list", model.PermLevelManage)
	p.addExact("/api/v1/admin/banned/list", "banned:manage")
	p.addExact("/api/v1/admin/banned/create", "banned:manage")
	p.addPrefix("/api/v1/admin/backups/", model.PermBackupManage)
	p.addExact("/api/v1/admin/trash/posts/list", "post:delete")
	p.addPattern("/api/v1/admin/trash/posts/{id}/restore", "post:delete")
	p.addExact("/api/v1/admin/trash/comments/list", "comment:delete")
	p.addPattern("/api/v1/admin/trash/comments/{id}/restore", "comment:delete")

	// ── Metrics ──────────────────────────────────────────
	p.addExact("/api/v1/metrics", "admin:metrics")

	// ── Auth ──────────────────────────────────────────────
	p.addExact("/api/v1/auth/reset_password", "user:update")
	// Self-service auth operations: any authenticated user
	p.addExact("/api/v1/auth/logout", "auth:self")
	p.addExact("/api/v1/auth/refresh", "auth:self")
	p.addExact("/api/v1/auth/change_password", "auth:self")
	p.addExact("/api/v1/auth/first_change_password", "auth:self")

	// ── OAuth (consumer account management) ───────────────
	p.addExact("/api/v1/oauth/accounts", "oauth:self")
	p.addPattern("/api/v1/oauth/{provider}/link", "oauth:self")
	p.addPattern("/api/v1/oauth/{provider}/unlink", "oauth:self")
	p.addExact("/api/v1/oauth/authorize", "oauth:self")
	// ── OAuth (provider) ──────────────────────────────────
	p.addExact("/api/v1/oauth/clients", "setting:manage")
	p.addExact("/api/v1/oauth/clients/list", "setting:manage")
	p.addPattern("/api/v1/oauth/clients/{id}/update", "setting:manage")
	p.addPattern("/api/v1/oauth/clients/{id}/reset-secret", "setting:manage")
	p.addPattern("/api/v1/oauth/clients/{id}/delete", "setting:manage")
	p.addExact("/api/v1/themes/preference", "theme:self")
	p.addExact("/api/v1/themes/preference/update", "theme:self")
	p.addPrefix("/api/v1/admin/plugins/", "plugin:manage")

	// ── SSO ──────────────────────────────────────────────
	p.addExact("/api/v1/sso/authorize", "sso:self")

	// ── User self-service ────────────────────────────────
	p.addExact("/api/v1/user/profile", "user:self")
	p.addExact("/api/v1/user/profile/update", "user:self")

	// ── Favorites ──────────────────────────────────────────
	p.addPattern("/api/v1/posts/{postId}/favorite", "favorite:self")
	p.addPattern("/api/v1/posts/{postId}/favorite/status", "favorite:self")
	p.addExact("/api/v1/favorites/list", "favorite:self")

	// ── Content likes & replies ──────────────────────────
	p.addPattern("/api/v1/contents/{contentId}/like", "content:like")
	p.addPattern("/api/v1/contents/{contentId}/like/status", "content:like")
	p.addPattern("/api/v1/contents/{contentId}/replies", model.PermCommentRead)

	// ── Drafts & revisions ────────────────────────────────
	p.addExact("/api/v1/drafts/save", model.PermDraftSelf)
	p.addExact("/api/v1/drafts/list", model.PermDraftSelf)
	p.addPattern("/api/v1/drafts/{id}/delete", model.PermDraftSelf)
	p.addPattern("/api/v1/posts/{id}/revisions/list", model.PermRevisionSelf)
	p.addPattern("/api/v1/posts/{id}/revisions/{revisionId}/restore", model.PermRevisionSelf)
	p.addPattern("/api/v1/comments/{id}/revisions/list", model.PermRevisionSelf)
	p.addPattern("/api/v1/comments/{id}/revisions/{revisionId}/restore", model.PermRevisionSelf)

	// ── Follow ────────────────────────────────────────────
	p.addPattern("/api/v1/users/{userId}/follow", "follow:self")
	p.addPattern("/api/v1/users/{userId}/unfollow", "follow:self")
	p.addPattern("/api/v1/users/{userId}/follow/status", "follow:read")
	p.addPattern("/api/v1/users/{userId}/followers", "follow:read")
	p.addPattern("/api/v1/users/{userId}/following", "follow:read")
	p.addPattern("/api/v1/users/{userId}/counts", "follow:read")

	// ── Upload (use exact + pattern to avoid over-matching future sub-paths) ──
	p.addExact("/api/v1/upload", "upload:self")
	p.addExact("/api/v1/upload/", "upload:self")
	p.addExact("/api/v1/upload/presign", "upload:self")
	p.addExact("/api/v1/upload/confirm", "upload:self")
	p.addExact("/api/v1/upload/list", "upload:self")
	p.addPattern("/api/v1/upload/{id}", "upload:self")
	p.addPattern("/api/v1/upload/{id}/delete", "upload:self")

	// ── Private messages ────────────────────────────────
	p.addExact("/api/v1/messages/conversations/list", model.PermPMSelf)
	p.addExact("/api/v1/messages/conversations/create", model.PermPMSelf)
	p.addPattern("/api/v1/messages/conversations/{id}/messages/list", model.PermPMSelf)
	p.addPattern("/api/v1/messages/conversations/{id}/messages/create", model.PermPMSelf)
	p.addPattern("/api/v1/messages/conversations/{id}/read", model.PermPMSelf)

	// ── Notifications ────────────────────────────────────
	p.addExact("/api/v1/notifications/list", "notification:self")
	p.addExact("/api/v1/notifications/unread_count", "notification:self")
	p.addExact("/api/v1/notifications/read_all", "notification:self")
	p.addExact("/api/v1/notifications/stream", "notification:self")
	p.addPattern("/api/v1/notifications/{id}/read", "notification:self")
	p.addPattern("/api/v1/notifications/{id}/delete", "notification:self")
	p.addExact("/api/v1/realtime/stream", model.PermRealtimeSelf)

	// ── Check-in (addPrefix already covers status & history) ─
	p.addExact("/api/v1/checkin", "checkin:self")
	p.addExact("/api/v1/checkin/", "checkin:self")
	p.addExact("/api/v1/checkin/status", "checkin:self")
	p.addExact("/api/v1/checkin/history", "checkin:self")

	// ── API Keys ──────────────────────────────────────────
	p.addExact("/api/v1/api-keys", "api-key:self")
	p.addExact("/api/v1/api-keys/", "api-key:self")
	p.addExact("/api/v1/api-keys/list", "api-key:self")
	p.addPattern("/api/v1/api-keys/{id}/revoke", "api-key:self")

	// ── Sessions ──────────────────────────────────────────
	p.addExact("/api/v1/sessions", "session:self")
	p.addExact("/api/v1/sessions/", "session:self")
	p.addPattern("/api/v1/sessions/{id}/revoke", "session:self")
	p.addExact("/api/v1/sessions/logout-all", "session:self")

	// ── Security (self-service) ──────────────────────────
	p.addExact("/api/v1/security/events", "security:read")
	p.addExact("/api/v1/security/login-history", "security:read")
	p.addExact("/api/v1/security/report", "security:report")

	// ── Dashboard ──────────────────────────────────────────
	p.addExact("/api/v1/dashboard", "dashboard:read")
	// ── Watches ────────────────────────────────────────────
	p.addExact("/api/v1/watches/set", model.PermFavoriteSelf)
	p.addExact("/api/v1/watches/list", model.PermFavoriteSelf)
	p.addExact("/api/v1/watches/delete", model.PermFavoriteSelf)
	p.addExact("/api/v1/watches/status", model.PermFavoriteSelf)

	// ── Blocks ─────────────────────────────────────────────
	p.addExact("/api/v1/blocks/list", model.PermFollowSelf)
	p.addPattern("/api/v1/users/{userId}/block", model.PermFollowSelf)
	p.addPattern("/api/v1/users/{userId}/unblock", model.PermFollowSelf)
	p.addPattern("/api/v1/users/{userId}/block/status", model.PermFollowRead)

	// ── MFA / email verification ───────────────────────────
	p.addExact("/api/v1/auth/send_verify_email", model.PermAuthSelf)
	p.addExact("/api/v1/auth/verify_email", model.PermAuthSelf)
	p.addExact("/api/v1/auth/mfa/totp/setup", model.PermAuthSelf)
	p.addExact("/api/v1/auth/mfa/totp/enable", model.PermAuthSelf)
	p.addExact("/api/v1/auth/mfa/totp/disable", model.PermAuthSelf)
	p.addExact("/api/v1/auth/mfa/backup/regenerate", model.PermAuthSelf)

	// ── Badges ─────────────────────────────────────────────
	p.addPattern("/api/v1/user/{id}/badges/list", model.PermFollowRead)
}
