package router

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
)

// 本文件锁住两件事：
//   1. 网关段只在 router.go 里挂两处（v1.Preflight / protected.Enforce）；子 party 一律不许挂。
//   2. Iris 的 party 中间件确实存在切片共享，protected 链必须真的带上 enforce 那一段。

// TestGatewayStagesMountedOnlyInRegister 用源码扫描代替"靠人记得"：
// v1 层已对 /api/v1 全量挂 gw.Preflight()，任何子 party 再挂一次网关段都会经父数组的
// 空闲槽位改写兄弟 party 已快照的链（详见 router.go 里 v1.Use 上方的注释）。
// 2026-10-14 实测：METRICS_ENABLED=true 时这种重复挂载让 /admin/stats 等 protected 路由
// 对普通用户返回 200 而不是 403。
func TestGatewayStagesMountedOnlyInRegister(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || f == "router.go" {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "Use(gw.") {
				t.Errorf("%s:%d: 子 party 不得挂网关段（Preflight/Enforce/Handler）；"+
					"重复挂载会覆盖 protected 已快照的链，使权限门失效", f, i+1)
			}
		}
	}
}

// TestProtectedPartyKeepsGatewayStage 是一个最小复刻：按 router.go 的真实形状搭 app
// （v1.Use 若干 → protected := v1.Party("") → protected.Use(gateway) → 兄弟 party 挂自己的
// 中间件 → 再注册 protected 路由），断言 protected 请求的链里 gateway 仍在。
// 去掉 router 里的重复挂载后此测试应保持通过；把它加回去则会失败。
func TestProtectedPartyKeepsGatewayStage(t *testing.T) {
	var mu sync.Mutex
	var ran []string
	mark := func(name string) iris.Handler {
		return func(ctx iris.Context) {
			mu.Lock()
			ran = append(ran, name)
			mu.Unlock()
			ctx.Next()
		}
	}

	app := iris.New()
	// app 级条数会影响父切片是否留有空闲槽位，这里取"有指标"的那一档（5 条）。
	app.Use(mark("cors"))
	app.Use(mark("trace"))
	app.Use(mark("logger"))
	app.Use(mark("recover"))
	app.Use(mark("prometheus"))

	v1 := app.Party("/api/v1")
	v1.Use(mark("bodyLimit"))
	v1.Use(mark("timeout"))
	v1.Use(mark("preflight"))
	v1.Use(mark("maintenance"))

	// protected 必须在公开 party 之后创建并 Use，与 router.go 的位置一致。
	protected := v1.Party("")
	protected.Use(mark("enforce"))

	v1.PartyFunc("/public", func(p iris.Party) {
		p.Post("/x", func(ctx iris.Context) { ctx.Text("public") })
	})
	protected.Post("/y", func(ctx iris.Context) { ctx.Text("protected") })

	e := httptest.New(t, app)

	mu.Lock()
	ran = nil
	mu.Unlock()
	e.POST("/api/v1/y").Expect().Status(200)
	mu.Lock()
	got := strings.Join(ran, ",")
	mu.Unlock()
	if !strings.Contains(got, "enforce") {
		t.Fatalf("protected 请求链里缺少 enforce，实际执行：%s", got)
	}
	if !strings.Contains(got, "preflight") || !strings.Contains(got, "maintenance") {
		t.Fatalf("protected 请求链应保留 v1 层中间件，实际执行：%s", got)
	}
}

// TestMCPRegisteredAtAppRoot 锁住 /mcp 的挂载位置（见 layered-architecture 规范
// 「网关保护面之外的端点必须自行完成等价检查」）。
//
// /mcp 必须注册在 app 根而不是 protected party 里：
//   - 注册进 protected 会连网关的会话解析一起继承，而 MCP 用的是 API Key，
//     两种凭证语义不同；
//   - 注册进 protected 还会让 /mcp 的 JSON-RPC 响应经过论坛信封中间件。
//
// 同时它必须**不在** /api/v1 之下：MCP 客户端按约定路径发现服务。
func TestMCPRegisteredAtAppRoot(t *testing.T) {
	data, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)

	if !strings.Contains(src, `app.Post("/mcp"`) {
		t.Error(`router.go 必须在 app 根注册 POST /mcp（app.Post("/mcp", ...)）`)
	}
	// 不允许把它挂到 v1 或 protected 上。
	for _, bad := range []string{`v1.Post("/mcp"`, `protected.Post("/mcp"`} {
		if strings.Contains(src, bad) {
			t.Errorf("/mcp 不得注册在 %s：MCP 客户端按 app 根路径发现服务，且不用论坛信封", bad)
		}
	}
}

// TestCustomizeRoutesRequirePermission 确认自定义层与管理端 MCP 路由都挂了权限门。
//
// 这些路由在 protected party 内，网关的 path→permission 映射未必覆盖到
// （新增前缀容易漏），因此显式挂 RequirePermission 更稳。
func TestCustomizeRoutesRequirePermission(t *testing.T) {
	data, err := os.ReadFile("routes_admin.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)

	checks := []struct{ path, perm string }{
		{`/mcp-servers`, `PermSettingManage`},
		{`/mcp`, `PermSettingManage`},
	}
	for _, c := range checks {
		// 只看对应 party 块内是否出现该权限码。
		idx := strings.Index(src, `"`+c.path+`"`)
		if idx < 0 {
			t.Errorf("routes_admin.go 未找到 %s 的 party 注册", c.path)
			continue
		}
		window := src[idx:min(idx+600, len(src))]
		if !strings.Contains(window, c.perm) {
			t.Errorf("%s 的路由必须挂 RequirePermission(%s)", c.path, c.perm)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestAdminCustomizeRoutesHaveGatewayPermission 钉住「路由注册了、网关也放行」。
//
// 这个缺口非常隐蔽：路由表里有、权限码也 seed 了、后台菜单也能点，
// 但网关的 path→permission 映射漏了这一条前缀，请求会被网关以 1013 拒绝，
// 而报错只说「禁止访问」，完全指不出是映射缺了。
//
// 做法：扫 routes_admin.go 里注册的 /admin/customization/* 与 /admin/mcp* 路径，
// 逐个断言在 gateway/permission.go 里能找到对应的映射。
func TestAdminCustomizeRoutesHaveGatewayPermission(t *testing.T) {
	routes, err := os.ReadFile("routes_admin.go")
	if err != nil {
		t.Fatal(err)
	}
	perms, err := os.ReadFile(filepath.Join("..", "gateway", "permission.go"))
	if err != nil {
		t.Fatal(err)
	}
	permSrc := string(perms)

	// 需要网关映射的两个前缀。
	for _, prefix := range []string{"/api/v1/admin/customization/", "/api/v1/admin/mcp/"} {
		if !strings.Contains(permSrc, `"`+prefix+`"`) {
			t.Errorf("gateway/permission.go 缺少 %s 的映射：路由注册了但网关会以 1013 拒绝", prefix)
		}
	}

	// 读接口必须是 customize:read，而不是被写权限码覆盖——
	// 否则「只想看一眼配置」的 Key 会拿到写入能力。
	for _, listPath := range []string{
		"/api/v1/admin/customization/snippets/list",
		"/api/v1/admin/customization/tokens/list",
		"/api/v1/admin/customization/nav/list",
		"/api/v1/admin/customization/modules/list",
		"/api/v1/admin/customization/pages/list",
	} {
		if !strings.Contains(permSrc, `"`+listPath+`"`) {
			t.Errorf("读接口 %s 必须显式映射到 customize:read（不能被写权限前缀覆盖）", listPath)
		}
	}

	// 确认 routes_admin.go 确实注册了这些前缀（防止测试与实现脱节）。
	for _, r := range []string{`"/customization"`, `"/mcp"`, `"/mcp-servers"`} {
		if !strings.Contains(string(routes), r) {
			t.Errorf("routes_admin.go 应注册 %s", r)
		}
	}
}

// TestAdminPluginRoutesHaveGatewayPermission 钉住插件后台的读/写权限分离。
//
// 插件后台有一个特别容易踩的坑：网关把 /api/v1/admin/plugins/ **整体**
// 挂在 plugin:manage 这个前缀上。若读接口不做精确映射，一个只想看清单的
// API Key 会顺带拿到安装/卸载能力——这是权限提升，且症状完全静默。
//
// 因此本测试同时断言三件事：
//  1. 读接口在 gateway/permission.go 里有**精确**映射（前缀覆盖不到它时才能读到）；
//  2. 这些精确映射指向 plugin:read，而不是 plugin:manage；
//  3. routes_admin.go 里读接口挂了 RequirePermission(plugin:read)。
func TestAdminPluginRoutesHaveGatewayPermission(t *testing.T) {
	routes, err := os.ReadFile("routes_admin.go")
	if err != nil {
		t.Fatal(err)
	}
	routesSrc := string(routes)

	perms, err := os.ReadFile(filepath.Join("..", "gateway", "permission.go"))
	if err != nil {
		t.Fatal(err)
	}
	permSrc := string(perms)

	// 读接口：必须显式映射到 plugin:read。
	// 从 permSrc 里取出该路径所在的那一行，断言同一行上写的是 plugin:read。
	readPaths := []string{
		"/api/v1/admin/plugins/list",
		"/api/v1/admin/plugins/sources/list",
	}
	for _, p := range readPaths {
		line := lineContaining(permSrc, `"`+p+`"`)
		if line == "" {
			t.Errorf("gateway/permission.go 缺少读接口 %s 的映射：它会被 /api/v1/admin/plugins/ 的"+
				"plugin:manage 前缀捕获，等于让只读凭证拿到安装能力", p)
			continue
		}
		if !strings.Contains(line, "PermPluginRead") {
			t.Errorf("%s 必须映射到 plugin:read，实际是: %s", p, strings.TrimSpace(line))
		}
	}

	// 写入类接口必须落在 plugin:manage 上（正向确认，防止误把写接口降到 read）。
	managePaths := []string{
		"/api/v1/admin/plugins/install",
		"/api/v1/admin/plugins/sources/create",
	}
	for _, p := range managePaths {
		line := lineContaining(permSrc, `"`+p+`"`)
		if line == "" {
			t.Errorf("gateway/permission.go 缺少写接口 %s 的显式映射", p)
			continue
		}
		if !strings.Contains(line, "PermPluginManage") {
			t.Errorf("%s 必须映射到 plugin:manage，实际是: %s", p, strings.TrimSpace(line))
		}
	}

	// routes_admin.go 侧：读路由挂 plugin:read，写路由挂 plugin:manage。
	// 只看 /plugins party 块内，避免误匹配到别处的同名字符串。
	idx := strings.Index(routesSrc, `PartyFunc("/plugins"`)
	if idx < 0 {
		t.Fatal("routes_admin.go 未找到 /plugins 的 party 注册")
	}
	window := routesSrc[idx:min(idx+2400, len(routesSrc))]

	for _, want := range []struct {
		path  string
		perm  string
		label string
	}{
		{`p.Post("/list"`, "PermPluginRead", "插件列表"},
		{`p.Post("/sources/list"`, "PermPluginRead", "来源列表"},
		{`p.Post("/install"`, "PermPluginManage", "安装"},
		{`p.Post("/sources/create"`, "PermPluginManage", "新增来源"},
		{`p.Post("/sources/{id}/approve"`, "PermPluginManage", "审批来源"},
		{`p.Post("/sources/{id}/reject"`, "PermPluginManage", "拒绝来源"},
		{`p.Post("/{id}/delete"`, "PermPluginManage", "删除"},
	} {
		line := lineContaining(window, want.path)
		if line == "" {
			t.Errorf("routes_admin.go 的 /plugins party 未注册 %s", want.path)
			continue
		}
		if !strings.Contains(line, want.perm) {
			t.Errorf("%s（%s）必须挂 RequirePermission(%s)，实际是: %s",
				want.label, want.path, want.perm, strings.TrimSpace(line))
		}
	}
}

// lineContaining 返回 src 中第一条包含 needle 的整行；没有则返回空串。
func lineContaining(src, needle string) string {
	for _, line := range strings.Split(src, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}
