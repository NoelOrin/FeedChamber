package middleware

import (
	"github.com/kataras/iris/v12"

	"feedchamber/internal/pkg"
)

// RegisterNotFoundHandlers 把 404/405 的信封钩子挂到 app 上。
//
// 只能做成函数而不是中间件：OnErrorCode 是 app 级 API（`app.OnErrorCode`），
// 而 iris.Handler 拿不到 *iris.Application，无法在中间件里完成注册。
//
// 同时 MUST 在 app.Configure 里启用 iris.WithFireMethodNotAllowed，否则方法不匹配
// 会落到 404 分支，用户得到「接口不存在」而不是「方法不允许」。
func RegisterNotFoundHandlers(app *iris.Application) {
	app.OnErrorCode(iris.StatusNotFound, func(ctx iris.Context) {
		pkg.FailStatus(ctx, iris.StatusNotFound, pkg.NOT_FOUND, "接口不存在")
	})

	// 405 用 INVALID_PARAMS(2001) 而不是自造码：错误码是单一共享命名空间
	// （internal/pkg/errors.go 的约定），方法用错属于「请求方式不对」的调用方问题，
	// 与参数错误同档；新增错误码需要走 openspec 提案。
	app.OnErrorCode(iris.StatusMethodNotAllowed, func(ctx iris.Context) {
		pkg.FailStatus(ctx, iris.StatusMethodNotAllowed, pkg.INVALID_PARAMS,
			"请求方法不允许：该接口不接受 "+ctx.Method())
	})
}
