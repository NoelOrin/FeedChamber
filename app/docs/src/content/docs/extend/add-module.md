---
title: 新增一个模块
description: 从 model 到路由的完整 checklist，含最小代码骨架
sidebar:
  order: 1
---

照着这个顺序做，每一步都能编译、能测。

## 后端 checklist

| # | 文件 | 做什么 |
|---|------|--------|
| 1 | `internal/model/xxx.go` | 数据模型（GORM 标签，无业务逻辑） |
| 2 | `internal/repository/xxx.go` | 接口 + SQL 实现，统一用 `WrapDBErr` |
| 3 | `internal/service/xxx_iface.go` | 服务接口 |
| 4 | `internal/service/xxx.go` | 业务逻辑，返回 `*pkg.AppError` |
| 5 | `internal/handler/xxx.go` | HTTP 层 + swag 注解 |
| 6 | `internal/handler/handler.go` | `Deps` 加字段，`New()` 里接线 |
| 7 | `internal/router/routes_<域>.go` | 挂路由 |
| 8 | `server/iris.go` | AutoMigrate 加模型 |
| 9 | `server/seed.go` | 需要的话加种子数据 |
| 10 | `internal/container/container.go` | 构造 repo/service（注意顺序） |
| 11 | `server/wiring.go` | 把服务填进 `handler.Deps` |
| 12 | `internal/gateway/permission.go` | 受保护路由必须在 path→权限映射里登记 |

漏了第 12 步的表现是：路由返回 403，日志里有 `unmapped protected route`（debug 级）。

## 最小骨架

### model

```go
// internal/model/widget.go
package model

import "time"

type Widget struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Widget) TableName() string { return "widgets" }
```

### repository

```go
// internal/repository/widget.go
package repository

type WidgetRepository interface {
	List(ctx context.Context) ([]model.Widget, error)
	Create(ctx context.Context, w *model.Widget) error
}

func NewWidgetRepository(store db.Store) WidgetRepository {
	return &sqlWidgetRepo{db: store.SQL()}
}
```

```go
// internal/repository/widget_sql.go
func (r *sqlWidgetRepo) Create(ctx context.Context, w *model.Widget) error {
	return WrapDBErr(r.db.DB().WithContext(ctx).Create(w).Error)
}
```

### service

```go
// internal/service/widget.go
func (s *widgetService) Create(ctx context.Context, in WidgetInput) (*model.Widget, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "name 不能为空")
	}
	w := &model.Widget{ID: pkg.NewID(), Name: in.Name, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.repo.Create(ctx, w); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, pkg.NewAppError(pkg.ALREADY_EXISTS)
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return w, nil
}
```

### handler

```go
// 每个方法上方必须有 swag 注解
// @Summary      创建 widget
// @Tags         Widget
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        payload body WidgetInput true "widget 内容"
// @Success      200  {object}  pkg.Response
// @Router       /admin/widgets/create [post]
func (h *widgetHandler) Create(ctx iris.Context) {
	var in service.WidgetInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	w, err := h.svc.Create(ctx.Request().Context(), in)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, w)
}
```

:::danger[swag 注解必须在 @Router 之前]
`@Param` / `@Success` 等写在 `@Router` 之后会被忽略，生成出来的规格里没有 body schema。
:::

### 路由 + 权限

```go
// internal/router/routes_admin.go
a.PartyFunc("/widgets", func(aw iris.Party) {
	aw.Post("/list", h.Widget.List)
	aw.Post("/create", h.Widget.Create)
})

// internal/gateway/permission.go —— 漏了这行就是 403
p.addPrefix("/api/v1/admin/widgets/", model.PermSettingManage)
```

## 前端 checklist

| # | 文件 | 做什么 |
|---|------|--------|
| 1 | `packages/api-client/src/types.ts` | 类型 |
| 2 | `packages/api-client/src/services/xxx.ts` | service 模块（写请求 `POST` + 动词后缀） |
| 3 | `app/web/app/admin/<route>/page.tsx` | 后台页（DataTable + useQuery） |
| 4 | `app/web/components/admin/layout/admin-shell.tsx` | 菜单登记 |
| 5 | `app/web/lib/admin/auth/route-access.ts` | 权限码登记 |

```ts
// packages/api-client/src/services/widget.ts
export const widgetService = {
	list(body: { page?: number; pageSize?: number } = {}) {
		return post<Widget[]>('/admin/widgets/list', body);
	},
	create(body: { name: string }) {
		return post<Widget>('/admin/widgets/create', body);
	},
};
```

```tsx
// app/web/app/admin/widgets/page.tsx
'use client';
const query = useQuery({
	queryKey: ['admin', 'widgets'],
	queryFn: () => widgetService.list(),
});
const data = query.data ?? [];
```

:::caution[列表页三态]
`query.isError` 必须有分支。用 `const { data = [] } = useQuery()` 会让 pending/error 期
每次渲染产生新数组——喂给 `useEffect` 依赖就是无限渲染。
:::

## 别忘了

```bash
cd app/server && make swag        # 刷新 docs/ 与 openapi.json
node scripts/check-agents-docs.mjs --write-index   # 新增模块目录要建 AGENTS.md
```

CI 会拦「改了模块代码但没更新该模块 AGENTS.md」。
