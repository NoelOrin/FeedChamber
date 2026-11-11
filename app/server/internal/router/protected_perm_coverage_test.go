package router

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"feedchamber/internal/gateway"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
)

// 本文件锁住「网关 path→permission 映射」对 protected 路由的覆盖率。
//
// 背景：网关的 permission.Middleware 对**未映射**的 protected 路径直接
// Fail(FORBIDDEN) 返回，而它排在路由级 middleware.RequirePermission 之前
// （见 gateway.enforce）。因此一条 protected 路由只要漏了 initRouteMap 登记，
// 就是「恒 403」——路由级权限码写了也不会执行，等同功能坏死。
//
// 2026-11-11 实测踩到过 4 条：POST /spaces/reorder、/spaces/{space_id}/move、
// /permission/space/effective、/permission/space/tree。此前 permission_test.go
// 是逐端点断言，没有任何测试把「router 里注册的全部 protected 路径」与
// initRouteMap 对一遍，所以这类漏登记只能靠线上 403 发现。
//
// 做法：扫描 router 包源码（与 middleware_chain_test.go 同一套手段），提取
// protected party 上注册的全部路径，把 {param} 换成具体值后逐条喂给网关的
// permission 中间件，要求它放行。放行 = 至少命中一条映射。

// permissivePermChecker 让任何权限码都判为有，从而把「未映射」与「无权限」区分开：
// 中间件拒绝只可能是没命中映射（默认拒绝），而不是权限不足。
type permissivePermChecker struct{}

func (permissivePermChecker) HasPermission(string, string) bool                { return true }
func (permissivePermChecker) HasPermissionInSpace(string, string, string) bool { return true }

var (
	// protected.Post("/x" / protected.Get("/x"：protected party 自身（前缀 /api/v1）
	reProtectedDirect = regexp.MustCompile(`\bprotected\.(Post|Get|Options)\(\s*"([^"]*)"`)
	// protected.PartyFunc("/x", func(s iris.Party) { —— 记下 (prefix, 变量名)
	reProtectedParty = regexp.MustCompile(`\bprotected\.PartyFunc\(\s*"([^"]*)"\s*,\s*func\(\s*(\w+)\s+iris\.Party`)
	// <var>.Post("/x"：PartyFunc 闭包内部注册
	reVarRoute = regexp.MustCompile(`^\s*(\w+)\.(Post|Get|Options)\(\s*"([^"]*)"`)
)

type protectedRoute struct {
	method string
	path   string
	file   string
	line   int
}

// collectProtectedRoutes 扫描非测试源码，返回 protected party 上注册的全部路由。
// 只追踪一层 PartyFunc（当前代码最深一层）；解析条数过少会让测试显性失败。
func collectProtectedRoutes(t *testing.T) []protectedRoute {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []protectedRoute
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		prefix, partyVar := "", ""
		for i, line := range strings.Split(string(raw), "\n") {
			code := line
			if idx := strings.Index(code, "//"); idx >= 0 {
				code = code[:idx]
			}
			if m := reProtectedParty.FindStringSubmatch(code); m != nil {
				prefix, partyVar = m[1], m[2]
				continue
			}
			if partyVar != "" && strings.Contains(code, "})") {
				prefix, partyVar = "", ""
				continue
			}
			if m := reProtectedDirect.FindStringSubmatch(code); m != nil {
				out = append(out, protectedRoute{method: m[1], path: "/api/v1" + m[2], file: f, line: i + 1})
				continue
			}
			if partyVar == "" {
				continue
			}
			if m := reVarRoute.FindStringSubmatch(code); m != nil && m[1] == partyVar {
				out = append(out, protectedRoute{method: m[2], path: "/api/v1" + prefix + m[3], file: f, line: i + 1})
			}
		}
	}
	return out
}

// concretize 把 {param} 段换成固定值，便于按字面匹配。
func concretize(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if len(p) > 2 && p[0] == '{' && p[len(p)-1] == '}' {
			parts[i] = "x"
		}
	}
	return strings.Join(parts, "/")
}

// TestProtectedRoutesAreMapped 断言每条 protected 路由都能通过网关权限门。
//
// 实现照 permission_test.go 的做法：把权限中间件挂在 app 级 Use 上，再注册真实
// 路径——这样 ctx.Path() 拿到的是真实请求路径，与生产一致（直接伪造 URL 不行：
// ctx.Path() 在未匹配路由时为空）。
func TestProtectedRoutesAreMapped(t *testing.T) {
	routes := collectProtectedRoutes(t)
	if len(routes) < 100 {
		t.Fatalf("只解析出 %d 条 protected 路由，解析逻辑可能已失效", len(routes))
	}

	pc := gateway.NewPermissionChecker(gateway.Deps{Perms: permissivePermChecker{}})

	// 同一具体路径去重：{id} 与 {space_id} 等不同参数名会 concretize 成同一串。
	paths := map[string][]protectedRoute{}
	var uniq []string
	for _, r := range routes {
		c := concretize(r.path)
		if _, ok := paths[c]; !ok {
			paths[c] = append(paths[c], r)
			uniq = append(uniq, c)
		}
	}

	allowed := map[string]bool{}
	app := iris.New()
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set("user_role", "admin")
		ctx.Next() // 中间件必须显式继续，否则后续中间件与 handler 都不执行
	})
	app.Use(func(ctx iris.Context) {
		if !pc.Middleware(ctx) {
			return
		}
		ctx.Next()
	})
	ok := func(ctx iris.Context) { ctx.StatusCode(iris.StatusNoContent) }
	for _, c := range uniq {
		app.Post(c, ok)
		app.Get(c, ok)
	}

	tester := httptest.New(t, app)
	var unmapped []string
	for _, c := range uniq {
		r := paths[c][0]
		if r.method == "Get" {
			tester.GET(c).Expect()
		} else {
			tester.POST(c).Expect()
		}
		allowed[c] = true // 占位，下面按响应状态判定
	}
	_ = allowed

	// 用状态码判定：映射命中且权限通过 → 204；未映射 → 403（默认拒绝）。
	for _, c := range uniq {
		r := paths[c][0]
		var code int
		if r.method == "Get" {
			code = tester.GET(c).Expect().Raw().StatusCode
		} else {
			code = tester.POST(c).Expect().Raw().StatusCode
		}
		if code == iris.StatusForbidden || code == iris.StatusUnauthorized {
			unmapped = append(unmapped, fmt.Sprintf("%s  (%s:%d)", c, r.file, r.line))
		}
	}

	sort.Strings(unmapped)
	if len(unmapped) > 0 {
		t.Fatalf("以下 protected 路由未在 gateway.initRouteMap 登记，会被默认拒绝而恒 403（新增 protected 路由必须同时登记映射）：\n  %s",
			strings.Join(unmapped, "\n  "))
	}
}
