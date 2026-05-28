package middleware

import (
	"strings"

	"orinforum/internal/model"
	"orinforum/internal/pkg"

	"github.com/kataras/iris/v12"
)

func JWTAuth() iris.Handler {
	return func(ctx iris.Context) {
		auth := ctx.GetHeader("Authorization")
		if auth == "" {
			pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
			ctx.StopExecution()
			return
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		if tokenStr == auth {
			tokenStr = auth
		}
		claims, err := pkg.ParseToken(tokenStr)
		if err != nil {
			pkg.Fail(ctx, pkg.TOKEN_WRONG)
			ctx.StopExecution()
			return
		}
		if pkg.IsTokenExpired(claims) {
			pkg.Fail(ctx, pkg.TOKEN_EXPIRED)
			ctx.StopExecution()
			return
		}
		ctx.Values().Set("username", claims.Username)
		ctx.Values().Set("user_uuid", claims.UserUUID)
		ctx.Values().Set("user_role", claims.Role)
		ctx.Values().Set("claims", claims)
		ctx.Next()
	}
}

func RequireRole(roles ...string) iris.Handler {
	return func(ctx iris.Context) {
		role := ctx.Values().GetString("user_role")
		if role == "" {
			pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
			ctx.StopExecution()
			return
		}
		for _, r := range roles {
			if role == r {
				ctx.Next()
				return
			}
		}
		pkg.Fail(ctx, pkg.FORBIDDEN)
		ctx.StopExecution()
	}
}

func BanCheck() iris.Handler {
	return func(ctx iris.Context) {
		role := ctx.Values().GetString("user_role")
		if role == "" {
			ctx.Next()
			return
		}
		if model.HasBanRole(role) {
			pkg.Fail(ctx, pkg.USER_BANNED)
			ctx.StopExecution()
			return
		}
		ctx.Next()
	}
}