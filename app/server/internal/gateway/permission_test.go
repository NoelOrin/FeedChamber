package gateway

import (
	"net/http"
	"testing"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
	"github.com/stretchr/testify/require"
)

// mockPermChecker implements PermissionChecker for testing.
type mockPermChecker struct {
	hasPerm func(role, code string) bool
}

func (m *mockPermChecker) HasPermission(role, code string) bool {
	if m.hasPerm != nil {
		return m.hasPerm(role, code)
	}
	return false
}

func (m *mockPermChecker) HasPermissionInSpace(role, spaceID, code string) bool {
	return m.HasPermission(role, code)
}

func setupPermApp(pc *PermissionCheckerMiddleware, role string) *iris.Application {
	app := iris.New()
	if role != "" {
		app.Use(func(ctx iris.Context) {
			ctx.Values().Set("user_role", role)
			ctx.Next()
		})
	}
	app.Use(func(ctx iris.Context) {
		if !pc.Middleware(ctx) {
			return
		}
		ctx.Next()
	})
	return app
}

func okHandler(ctx iris.Context) {
	ctx.JSON(iris.Map{"ok": true})
}

func TestPermMiddleware_NoMapping_Denies(t *testing.T) {
	mock := &mockPermChecker{}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "admin")
	app.Post("/api/v1/unmapped/route", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/unmapped/route").Expect().Status(http.StatusForbidden)
}

func TestPermMiddleware_NoRole_Returns401(t *testing.T) {
	mock := &mockPermChecker{}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "") // no role set
	app.Post("/api/v1/posts/create", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/posts/create").Expect().Status(http.StatusUnauthorized)
}

func TestPermMiddleware_PermissionDenied_Returns403(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return false // deny all
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "user")
	app.Post("/api/v1/posts/create", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/posts/create").Expect().Status(http.StatusForbidden)
}

func TestPermMiddleware_Allowed_Passes(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return role == "admin" && code == "post:create"
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "admin")
	app.Post("/api/v1/posts/create", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/posts/create").Expect().Status(http.StatusOK).JSON().Object().Value("ok").Equal(true)
}

func TestPermMiddleware_ExactPath_MultiplePermCodes(t *testing.T) {
	allowed := map[string]bool{}
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return allowed[code]
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "editor")
	app.Post("/api/v1/posts/create", okHandler)
	app.Post("/api/v1/posts/comments/list", okHandler)

	// Only grant post:create, not comment:delete
	allowed["post:create"] = true

	e := httptest.New(t, app)
	e.POST("/api/v1/posts/create").Expect().Status(http.StatusOK)
	e.POST("/api/v1/posts/comments/list").Expect().Status(http.StatusForbidden)
}

func TestPermMiddleware_PrefixMatch_Announcements(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return code == "announcement:manage"
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "moderator")
	app.Post("/api/v1/admin/announcements/list", okHandler)
	app.Post("/api/v1/admin/announcements/create", okHandler)
	app.Post("/api/v1/admin/announcements/42/update", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/admin/announcements/list").Expect().Status(http.StatusOK)
	e.POST("/api/v1/admin/announcements/create").Expect().Status(http.StatusOK)
	e.POST("/api/v1/admin/announcements/42/update").Expect().Status(http.StatusOK)
}

func TestPermMiddleware_PrefixMatch_Settings(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return role == "admin" && code == "setting:manage"
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "admin")
	app.Post("/api/v1/admin/settings/batch", okHandler)
	app.Post("/api/v1/admin/settings/site_name/update", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/admin/settings/batch").Expect().Status(http.StatusOK)
	e.POST("/api/v1/admin/settings/site_name/update").Expect().Status(http.StatusOK)
}

func TestPermMiddleware_PrefixMatch_Batch(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return code == "batch:edit"
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "editor")
	app.Post("/api/v1/admin/batch/users", okHandler)
	app.Post("/api/v1/admin/batch/posts", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/admin/batch/users").Expect().Status(http.StatusOK)
	e.POST("/api/v1/admin/batch/posts").Expect().Status(http.StatusOK)
}

func TestPermMiddleware_PrefixMatch_Moderation(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return code == "report:manage"
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "mod")
	app.Post("/api/v1/admin/moderation/pending", okHandler)
	app.Post("/api/v1/admin/moderation/rules/list", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/admin/moderation/pending").Expect().Status(http.StatusOK)
	e.POST("/api/v1/admin/moderation/rules/list").Expect().Status(http.StatusOK)
}

func TestPermMiddleware_PrefixMatch_Webhooks(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return role == "admin" && code == "role:manage"
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "admin")
	app.Post("/api/v1/admin/webhooks/create", okHandler)
	app.Post("/api/v1/admin/webhooks/list", okHandler)
	app.Post("/api/v1/admin/webhooks/42/deliveries/list", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/admin/webhooks/create").Expect().Status(http.StatusOK)
	e.POST("/api/v1/admin/webhooks/list").Expect().Status(http.StatusOK)
	e.POST("/api/v1/admin/webhooks/42/deliveries/list").Expect().Status(http.StatusOK)
}

// 契约 1：POST /posts/{id}/status 受 report:manage 约束（后台「通过 / 拒绝」）。
func TestPermMiddleware_PostStatusRequiresReportManage(t *testing.T) {
	allowed := &mockPermChecker{
		hasPerm: func(role, code string) bool { return code == "report:manage" },
	}
	pc := NewPermissionChecker(Deps{Perms: allowed})
	app := setupPermApp(pc, "moderator")
	app.Post("/api/v1/posts/{id}/status", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/posts/p1/status").Expect().Status(http.StatusOK)

	denied := &mockPermChecker{hasPerm: func(role, code string) bool { return code == "post:create" }}
	pcDenied := NewPermissionChecker(Deps{Perms: denied})
	appDenied := setupPermApp(pcDenied, "user")
	appDenied.Post("/api/v1/posts/{id}/status", okHandler)

	httptest.New(t, appDenied).POST("/api/v1/posts/p1/status").Expect().Status(http.StatusForbidden)
}

func TestPermMiddleware_ExactPath_NoTrailingSlash(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return code == "upload:self" || code == "api-key:self" || code == "session:self" || code == "checkin:self"
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "user")
	app.Post("/api/v1/upload", okHandler)
	app.Post("/api/v1/api-keys", okHandler)
	app.Post("/api/v1/api-keys/42/revoke", okHandler)
	app.Get("/api/v1/sessions", okHandler)
	app.Post("/api/v1/checkin", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/upload").Expect().Status(http.StatusOK)
	e.POST("/api/v1/api-keys").Expect().Status(http.StatusOK)
	e.POST("/api/v1/api-keys/42/revoke").Expect().Status(http.StatusOK)
	e.GET("/api/v1/sessions").Expect().Status(http.StatusOK)
	e.POST("/api/v1/checkin").Expect().Status(http.StatusOK)
}

func TestPermMiddleware_APIKeyScopeRestrictsPermission(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool { return role == "user" },
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("user_role", "user")
		ctx.Values().Set("auth_method", "api_key")
		ctx.Values().Set("api_key_scopes", "read")
		ctx.Next()
	})
	app.Use(func(ctx iris.Context) {
		if !pc.Middleware(ctx) {
			return
		}
		ctx.Next()
	})
	app.Post("/api/v1/posts/batch", okHandler)
	app.Post("/api/v1/posts/create", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/posts/batch").Expect().Status(http.StatusOK)
	e.POST("/api/v1/posts/create").Expect().Status(http.StatusForbidden)
}

func TestPermMiddleware_PatternMatch_SingleParam(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return role == "admin" && code == "post:delete"
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "admin")
	app.Post("/api/v1/posts/{id}/delete", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/posts/42/delete").Expect().Status(http.StatusOK)
	e.POST("/api/v1/posts/abc/delete").Expect().Status(http.StatusOK)
}

func TestPermMiddleware_PatternMatch_TwoParams(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return code == "user:update"
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "admin")
	app.Post("/api/v1/user/{id}/space/{space_id}/banned", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/user/u1/space/s42/banned").Expect().Status(http.StatusOK)
}

func TestPermMiddleware_PatternMatch_NoPermission(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return false
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "user")
	app.Post("/api/v1/posts/{id}/update", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/posts/99/update").Expect().Status(http.StatusForbidden)
}

func TestPermMiddleware_GroupsRoutes(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return role == "admin" && (code == "role:manage" || code == "role:read")
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "admin")
	app.Post("/api/v1/groups/list", okHandler)
	app.Post("/api/v1/groups/create", okHandler)
	app.Post("/api/v1/groups/{id}/update", okHandler)
	app.Post("/api/v1/groups/{id}/delete", okHandler)
	app.Post("/api/v1/groups/{id}/members/list", okHandler)
	app.Post("/api/v1/groups/{id}/members/add", okHandler)
	app.Post("/api/v1/groups/{id}/members/remove", okHandler)
	app.Post("/api/v1/groups/{id}/permissions", okHandler)
	app.Post("/api/v1/groups/{id}/permissions/sync", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/groups/list").Expect().Status(http.StatusOK)
	e.POST("/api/v1/groups/create").Expect().Status(http.StatusOK)
	e.POST("/api/v1/groups/g1/update").Expect().Status(http.StatusOK)
	e.POST("/api/v1/groups/g1/delete").Expect().Status(http.StatusOK)
	e.POST("/api/v1/groups/g1/members/list").Expect().Status(http.StatusOK)
	e.POST("/api/v1/groups/g1/members/add").Expect().Status(http.StatusOK)
	e.POST("/api/v1/groups/g1/members/remove").Expect().Status(http.StatusOK)
	e.POST("/api/v1/groups/g1/permissions").Expect().Status(http.StatusOK)
	e.POST("/api/v1/groups/g1/permissions/sync").Expect().Status(http.StatusOK)
}

func TestPermMiddleware_UserTrustLevelRoute(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool { return code == "user:update" },
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "admin")
	app.Post("/api/v1/user/{id}/trust_level/update", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/user/u1/trust_level/update").Expect().Status(http.StatusOK)
}

func TestPermMiddleware_UnmappedSubpath_Denies(t *testing.T) {
	// Unmapped routes are deny-by-default and must not reach the handler.
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			t.Error("should not be called")
			return false
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "user")
	app.Post("/api/v1/unknown/route", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/unknown/route").Expect().Status(http.StatusForbidden)
}

func TestPermMiddleware_GetRoute_Allowed(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return code == "user:read"
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "admin")
	app.Get("/api/v1/user/{id}", okHandler)

	e := httptest.New(t, app)
	e.GET("/api/v1/user/uuid-1").Expect().Status(http.StatusOK)
}

func TestPermMiddleware_GetRoute_Denied(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return false
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "user")
	app.Post("/api/v1/user/search", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/user/search").Expect().Status(http.StatusForbidden)
}

func TestPermMiddleware_AllPermCodes_Mapped(t *testing.T) {
	// Verify mapped routes are accessible when perm is granted
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return true // grant all
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "superadmin")

	routes := []string{
		"/api/v1/posts/create",
		"/api/v1/posts/comments/list",
		"/api/v1/user/search",
		"/api/v1/user/list",
		"/api/v1/role/list",
		"/api/v1/role/create",
		"/api/v1/permission/list",
		"/api/v1/permission/role",
		"/api/v1/permission/sync",
		"/api/v1/permission/space",
		"/api/v1/permission/space/sync",
		"/api/v1/permission/space/clear",
		"/api/v1/tags/create",
		"/api/v1/reports/create",
		"/api/v1/reports/list",
		"/api/v1/spaces/create",
		"/api/v1/admin/stats",
		"/api/v1/admin/dashboard",
		"/api/v1/admin/search",
		"/api/v1/admin/audit_logs/list",
		"/api/v1/admin/reconcile/counters",
		"/api/v1/admin/banned/list",
		"/api/v1/messages/conversations/list",
		"/api/v1/messages/conversations/create",
		"/api/v1/dashboard",
	}

	for _, route := range routes {
		app.Post(route, okHandler)
	}

	e := httptest.New(t, app)
	for _, route := range routes {
		e.POST(route).Expect().Status(http.StatusOK)
	}
}

func TestPermMiddleware_PatternMatch_AdminReports(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return code == "report:manage"
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "moderator")
	app.Get("/api/v1/reports/{id}", okHandler)
	app.Post("/api/v1/reports/{id}/status", okHandler)
	app.Post("/api/v1/reports/{id}/delete", okHandler)

	e := httptest.New(t, app)
	e.GET("/api/v1/reports/1").Expect().Status(http.StatusOK)
	e.POST("/api/v1/reports/1/status").Expect().Status(http.StatusOK)
	e.POST("/api/v1/reports/1/delete").Expect().Status(http.StatusOK)
}

func TestPermMiddleware_PatternMatch_CommentsAdmin(t *testing.T) {
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			return code == "comment:delete"
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "mod")
	app.Get("/api/v1/comments/{id}", okHandler)
	app.Post("/api/v1/comments/{id}/delete", okHandler)

	e := httptest.New(t, app)
	e.GET("/api/v1/comments/5").Expect().Status(http.StatusOK)
	e.POST("/api/v1/comments/5/delete").Expect().Status(http.StatusOK)
}

func TestNewPermissionChecker_NonNil(t *testing.T) {
	mock := &mockPermChecker{}
	pc := NewPermissionChecker(Deps{Perms: mock})
	require.NotNil(t, pc)
	require.NotNil(t, pc.exactMap)
	require.NotNil(t, pc.prefixMap)
}

func TestPermMiddleware_PMAndPhase2Routes(t *testing.T) {
	got := map[string]string{}
	mock := &mockPermChecker{
		hasPerm: func(role, code string) bool {
			got[code] = code
			return true
		},
	}
	pc := NewPermissionChecker(Deps{Perms: mock})
	app := setupPermApp(pc, "user")
	app.Post("/api/v1/messages/conversations/list", okHandler)
	app.Post("/api/v1/messages/conversations/create", okHandler)
	app.Post("/api/v1/messages/conversations/{id}/messages/list", okHandler)
	app.Post("/api/v1/messages/conversations/{id}/messages/create", okHandler)
	app.Post("/api/v1/messages/conversations/{id}/read", okHandler)
	app.Post("/api/v1/posts/{id}/close", okHandler)
	app.Post("/api/v1/posts/{id}/read", okHandler)
	app.Post("/api/v1/admin/audit_logs/list", okHandler)

	e := httptest.New(t, app)
	e.POST("/api/v1/messages/conversations/list").Expect().Status(http.StatusOK)
	e.POST("/api/v1/messages/conversations/create").Expect().Status(http.StatusOK)
	e.POST("/api/v1/messages/conversations/c1/messages/list").Expect().Status(http.StatusOK)
	e.POST("/api/v1/messages/conversations/c1/messages/create").Expect().Status(http.StatusOK)
	e.POST("/api/v1/messages/conversations/c1/read").Expect().Status(http.StatusOK)
	e.POST("/api/v1/posts/p1/close").Expect().Status(http.StatusOK)
	e.POST("/api/v1/posts/p1/read").Expect().Status(http.StatusOK)
	e.POST("/api/v1/admin/audit_logs/list").Expect().Status(http.StatusOK)

	require.Equal(t, "pm:self", got["pm:self"])
	require.Equal(t, "post:create", got["post:create"])
	require.Equal(t, "post:read", got["post:read"])
	require.Equal(t, "stats:read", got["stats:read"])
}
