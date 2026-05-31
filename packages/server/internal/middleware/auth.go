package middleware

import (
	"context"
	"strings"

	"orinforum/internal/model"
	"orinforum/internal/pkg"

	"github.com/kataras/iris/v12"
)

// settingReader is injected at startup to read community settings.
type settingReader interface {
	Get(ctx context.Context, key string) (string, error)
}

var settingSvc settingReader

// SetSettingReader injects the setting service (called at startup).
func SetSettingReader(s settingReader) {
	settingSvc = s
}

func isAuthRequired(key string) bool {
	if settingSvc == nil {
		return false
	}
	val, err := settingSvc.Get(context.Background(), key)
	if err != nil {
		return false
	}
	return val == "true"
}

// OptionalJWTAuth parses the token if present but never blocks the request.
// Sets the same context values as JWTAuth when a valid token is provided.
func OptionalJWTAuth() iris.Handler {
	return func(ctx iris.Context) {
		auth := ctx.GetHeader("Authorization")
		if auth != "" {
			tokenStr := strings.TrimPrefix(auth, "Bearer ")
			if claims, err := pkg.ParseToken(tokenStr); err == nil && !pkg.IsTokenExpired(claims) {
				ctx.Values().Set("username", claims.Username)
				ctx.Values().Set("user_uuid", claims.UserUUID)
				ctx.Values().Set("user_role", claims.Role)
				ctx.Values().Set("claims", claims)
			}
		}
		ctx.Next()
	}
}

// PostAuthGate blocks unauthenticated requests when post_auth_required=true.
func PostAuthGate() iris.Handler {
	return func(ctx iris.Context) {
		if isAuthRequired(model.SettingPostAuthRequired) && ctx.Values().GetString("user_uuid") == "" {
			pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
			ctx.StopExecution()
			return
		}
		ctx.Next()
	}
}

// CommentAuthGate blocks unauthenticated requests when comment_auth_required=true.
func CommentAuthGate() iris.Handler {
	return func(ctx iris.Context) {
		if isAuthRequired(model.SettingCommentAuthRequired) && ctx.Values().GetString("user_uuid") == "" {
			pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
			ctx.StopExecution()
			return
		}
		ctx.Next()
	}
}

// permissionChecker 抽象权限查询，便于测试和解耦。
type permissionChecker interface {
	HasPermission(roleName, permCode string) bool
}

var permChecker permissionChecker

// SetPermissionChecker 注入权限查询实现（启动时调用）。
func SetPermissionChecker(c permissionChecker) {
	permChecker = c
}

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

// RequirePermission 基于权限码的鉴权，替代硬编码角色名。
func RequirePermission(permCode string) iris.Handler {
	return func(ctx iris.Context) {
		role := ctx.Values().GetString("user_role")
		if role == "" {
			pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
			ctx.StopExecution()
			return
		}
		if permChecker == nil || !permChecker.HasPermission(role, permCode) {
			pkg.Fail(ctx, pkg.FORBIDDEN)
			ctx.StopExecution()
			return
		}
		ctx.Next()
	}
}

// RequireOwnerOrPermission 资源归属校验：拥有资源归属 或 拥有指定权限码 即可放行。
func RequireOwnerOrPermission(ownerContextKey, permCode string) iris.Handler {
	return func(ctx iris.Context) {
		// 先尝试权限码放行
		role := ctx.Values().GetString("user_role")
		if permChecker != nil && permChecker.HasPermission(role, permCode) {
			ctx.Next()
			return
		}
		// 再尝试归属放行
		owner := ctx.Values().GetString(ownerContextKey)
		username := ctx.Values().GetString("username")
		if owner != "" && username != "" && owner == username {
			ctx.Next()
			return
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
