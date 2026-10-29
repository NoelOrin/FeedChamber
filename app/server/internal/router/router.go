// Package router 注册全部 HTTP 路由。
//
// 布局：Register 只留下 v1 party 的创建、app 根的 well-known/SEO 注册、
// v1.Use 中间件链和 protected party 的网关挂载，其余按域拆到 routes_*.go，
// Register 按原顺序依次调用这些段函数。
//
// 顺序即语义，属于硬约束：
//   - 固定路径必须注册在参数路径之前；
//   - 同一前缀会注册两次（/auth、/oauth、/sso、/tags、/spaces、/posts、/user：
//     公开一次、protected 一次），两段的相对位置决定最终路由表；
//   - Iris 在"注册路由的那一刻"对 party 当前的中间件做快照，且 Party() 生成的
//     子 party 与父 party 共用底层数组的空闲槽位。因此
//     protected := v1.Party("") 与 protected.Use(gw.Enforce()) 必须留在公开 party
//     之后、任何 protected 路由之前的原位置：把它俩提前会改写后续路由的中间件链
//     （实测 protected 面会丢掉网关链，权限门随之失效）。
//
// 所以各段函数体是原 Register 体的逐字连续切片（含原有的 // ── 分节注释），只做代码
// 移动：不重新分组、不调整中间件、不改顺序。函数开头的 v1/protected/h/deps/gw
// 局部别名只为让切片与原文完全一致而存在。
package router

import (
	"time"

	"orinforum/internal/gateway"
	"orinforum/internal/handler"
	"orinforum/internal/middleware"

	"github.com/kataras/iris/v12"
)

// reg 一次 Register 调用内的共享状态：两个 party、处理器聚合体、门控依赖与已在
// composition root 装配好的网关。各段函数以方法的形式挂在它上面，
// 免得六个参数在十几个函数签名里重复一遍。
type reg struct {
	// app 留给 app 根级别的注册段；当前 app 根注册（well-known、SEO）仍内联在 Register 里。
	app *iris.Application
	v1  iris.Party // /api/v1，挂 PathBodyLimit / RequestTimeout / gw.Preflight / MaintenanceGate
	// protected 挂 gw.Enforce()。它只在 Register 内的原始位置（公开 party 之后、
	// 任何 protected 路由之前）才创建并赋值：party 的中间件在注册路由那一刻取快照，
	// 且子 party 与父 party 共用底层数组空闲槽，创建时机提前会改写后续路由的中间件链。
	protected iris.Party
	h         *handler.Handler
	deps      middleware.Deps
	gw        *gateway.Gateway
}

// Register wires all routes.
//
// deps 提供门控中间件所需的共享服务，gw 是已在 composition root 装配好的网关
// （认证/限流/权限/会话/安全一条流水线）。路由层只挂路由与中间件，不再自己构造网关：
// 此前它接收 *gorm.DB 并就地 gateway.New(...)，让"注册路由"这一层同时承担装配职责，
// 还要对 deps 里的服务做 unchecked 类型断言。
//
// Register 自身只留 v1 party 的创建、app 根注册、v1.Use 链与 protected 的网关挂载，
// 路由按域分给同包的段函数，调用顺序就是原函数体的顺序。
func Register(app *iris.Application, h *handler.Handler, deps middleware.Deps, gw *gateway.Gateway) {
	v1 := app.Party("/api/v1")

	// ── Well-Known (OIDC SSO discovery, app root) ─────────────────
	app.Get("/.well-known/openid-configuration", gw.SSO.Discovery)
	app.Get("/.well-known/jwks.json", gw.SSO.JWKS)

	// ── Body size limit (global 1MB, upload 10MB) ─────────────
	v1.Use(middleware.PathBodyLimit(1*1024*1024, map[string]int64{
		"/api/v1/upload": 10 * 1024 * 1024,
	}))

	// ── Request timeout (30s, SSE /stream auto-skipped) ────────
	v1.Use(middleware.RequestTimeout(30 * time.Second))
	// Preflight = trace + 全局限流 + 可选会话解析，对 /api/v1 全量生效：公开端点据此识别
	// 登录用户，MaintenanceGate 据此认出管理员角色，protected 面的请求也已完成凭证解析，
	// 所以下游的 gw.Enforce() 不再重复计数、不做第二次会话查询（阶段幂等，见
	// internal/gateway/gateway.go 的 stage 标记）。
	//
	// 子 party MUST NOT 再各自挂任何网关段：Iris 的 Party() 只复制父链切片，兄弟 party 的
	// Use 会写进父数组的同一个空闲槽位，覆盖 protected 已快照到的 gw.Enforce()，权限门随之
	// 失效。是否有空闲槽取决于 app 级中间件条数（METRICS_ENABLED 会改变它），所以这个坑
	// 只在部分配置下显形。由 middleware_chain_test.go 用源码扫描钉住。
	v1.Use(gw.Preflight())
	v1.Use(middleware.MaintenanceGate(deps.Settings))

	// ── Public (no auth required) ─────────────────────────────
	v1.Get("/health", h.Health)
	app.Get("/sitemap.xml", h.SEO.SitemapIndex)
	app.Get("/sitemap-posts.xml", h.SEO.Sitemap)
	app.Get("/sitemap-spaces.xml", h.SEO.SitemapSpaces)
	app.Get("/sitemap-tags.xml", h.SEO.SitemapTags)
	app.Get("/robots.txt", h.SEO.Robots)
	app.Get("/latest.rss", h.SEO.LatestRSS)

	r := &reg{app: app, v1: v1, h: h, deps: deps, gw: gw}
	// 其余路由按域拆在同包的 routes_*.go 里，下面的调用顺序就是原函数体的顺序。
	r.publicRoutes()
	r.publicOptionalRoutes()

	// ── Protected (require auth) ───────────────────────────────
	protected := v1.Party("")
	protected.Use(gw.Enforce())
	r.protected = protected

	r.dashboardRoutes()
	r.protectedAuthRoutes()
	r.userRoutes()
	r.accessRoutes()
	r.tagManageRoutes()
	r.postRoutes()
	r.engagementRoutes()
	r.uploadRoutes()
	r.messageRoutes()
	r.reportRoutes()
	r.motionRoutes()
	r.accountRoutes()
	r.spaceManageRoutes()
	r.adminRoutes()
	r.trustRoutes()
	r.gateRoutes()
}
