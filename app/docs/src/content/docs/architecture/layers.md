---
title: 分层职责
description: 每一层做什么、不做什么，以及为什么这样切
sidebar:
  order: 2
---

## Handler

**做**：解析请求（路径参数 / body / 分页）、调 service、用统一信封写响应。

**不做**：业务规则、所有权判断、跨资源编排。

```go
func (h *postHandler) Create(ctx iris.Context) {
	var in service.PostInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	post, err := h.svc.Create(ctx.Request().Context(), in, currentUserID(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, post)
}
```

约定：
- 列表接口**必须容忍空 body**（`ctx.ReadJSON` 在无 body 时返回 `io.EOF`，
  直接判 400 会把「合法的空筛选」打成错误）
- 每个方法上方必须有 swag 注解，`make swag` 依赖它生成 `docs/` 与 `openapi.json`

## Service

**做**：业务规则、事务、所有权与可见性判定、发事件、返回 `*pkg.AppError`。

**不做**：直接碰 HTTP、直接拼 SQL、直接写 `ctx.JSON`。

所有权检查在这里（不在中间件）：`post.AuthorID == actorID || hasPerm`。
理由：中间件只有路径和角色，判不了「这是不是你的帖子」。

## Repository

**做**：数据访问，一个资源一个文件（接口 + SQL 实现），统一用 `WrapDBErr` 翻译错误。

**不做**：业务规则、事务边界（事务在 service 层用 `db.TransactionManager`）。

```go
type UserRepository interface { ... }

func NewUserRepository(store db.Store) UserRepository {
	return &sqlUserRepo{db: store.SQL()}
}
```

`WrapDBErr` 把「记录不存在」翻成 `ErrNotFound`、「唯一冲突」翻成 `ErrDuplicate`。
service 只判断这两个哨兵，不要字符串匹配错误文案——三家后端文案各不相同。

## Gateway / Middleware

网关负责**凭证解析与授权**（谁在调用、能干什么）；
中间件负责**身份已确定之后**的门控与请求基础设施（限流、超时、body 上限、维护期）。

流水线是**可组合的两段**：

- `v1.Use(gw.Preflight())`：trace + 全局 IP 限流 + 可选凭证解析
- `protected.Use(gw.Enforce())`：必需凭证 → banCheck（被封返回 404）→ 权限 → 安全风控 → 会话跟踪

两段各自幂等：一个 protected 请求的限流与凭证解析都只发生一次。

:::caution[挂在应用根的端点必须自带保护]
`/mcp` 不在 `/api/v1` 之下，拿不到上面两段。因此它自带 `mcp.Enforcer`，
四项检查一个都不能省：凭证、封禁、权限码、限流。
:::

## 为什么这样切

- **可测试**：service 只依赖接口，用内存替身就能测业务规则
- **可审计**：`server/wiring.go` 一个文件读完全部 HTTP 依赖
- **可替换**：换存储只改 repository 实现；换凭证方式只改 gateway
