package middleware

import "github.com/kataras/iris/v12"

func CORS() iris.Handler {
	return func(ctx iris.Context) {
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		ctx.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if ctx.Method() == "OPTIONS" {
			ctx.StatusCode(iris.StatusNoContent)
			return
		}
		ctx.Next()
	}
}

func Logger() iris.Handler {
	return func(ctx iris.Context) {
		ctx.Application().Logger().Infof("%s %s", ctx.Method(), ctx.Path())
		ctx.Next()
	}
}
