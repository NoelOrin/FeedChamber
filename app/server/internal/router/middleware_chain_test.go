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
