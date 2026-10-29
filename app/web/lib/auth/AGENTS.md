# Web Auth（`lib/auth/`）

Web 前端的会话状态机与服务端鉴权门。**本目录有一条不可逾越的导入边界**。

> 上级：[lib/AGENTS.md](../AGENTS.md) · 消费方：`components/providers.tsx`、`app/(app)/**`、`app/(auth)/layout.tsx`

## 文件表

| 文件 | 职责 | 运行环境 |
|------|------|----------|
| `session.ts` | zustand 状态机：`unknown` / `anonymous` / `loading` / `authenticated` / `refreshing` / `expired`；命令式门面 `authSession`（`get status` / `get user` / `get isAuthenticated` / `setAuthenticated` …） | 浏览器（+ SSR 首帧注入） |
| `client.ts` | 浏览器侧认证动作（登录/登出/刷新后的状态迁移） | 浏览器 |
| `sync.ts` | BroadcastChannel + localStorage 跨标签页同步（`AUTH_LOGIN` / `AUTH_LOGOUT` / `AUTH_EXPIRED`），启动点在 `components/providers.tsx` | 浏览器 |
| `guards.ts` | 服务端门：`requireAuth` / `requireGuest` / `optionalAuth` / `requireAuthBySetting` | **仅服务端**（`next/headers`） |
| `index.ts` | barrel：导出 `client` / `session` / `sync`，**刻意不导出 `guards`** | 浏览器安全 |

## 不可逾越的导入边界

```ts
// lib/auth/index.ts
export * from './client';
export * from './session';
export * from './sync';
// guards.ts is intentionally NOT re-exported: it imports `next/headers`
// and must not end up in a client bundle.
```

`guards.ts` 一旦被 barrel re-export，`next/headers` 就会进浏览器包，构建直接失败。**服务端组件要显式 `import { requireAuth } from '~/lib/auth/guards'`。**

## 状态机语义

| 状态 | 含义 | 谁设置 |
|------|------|--------|
| `unknown` | 尚未判定（首帧） | 初始值 |
| `anonymous` | 已确认未登录 | `setAnonymous` / 登录失败 / 登出 |
| `loading` | 正在登录 | `setLoading` |
| `authenticated` | 已登录且有 user | `setAuthenticated`（可带 `broadcast`） |
| `refreshing` | 正在刷新凭证 | `setRefreshing` |
| `expired` | 会话已过期（`401` 且刷新失败） | `setExpired` |

`authSession.isAuthenticated` 要求 `status === 'authenticated' && user !== null`——只看 status 会在 user 尚未落地时误判。

跨标签同步只在**显式传 `broadcast: true`** 时发出（避免 SSR 首帧注入触发广播回环）。

## 服务端门（guards.ts）

| 函数 | 语义 |
|------|------|
| `requireAuth(pathname)` | 无会话 → `redirect('/login?redirect=<pathname>')`；有则返回 `User` |
| `optionalAuth()` | 有会话返回 `User`，否则 `null`，**不重定向** |
| `requireGuest()` | 已登录 → 弹回（`(auth)/layout.tsx` 用） |
| `requireAuthBySetting(settingKey, pathname)` | 设置非 `'true'` 放行（返回可能为 null 的 `User`）；为 `'true'` 时等价 `requireAuth` |

`getSessionUser()` 先做**便宜的 cookie 标记检查**（`master_auth=1`），再 `loadProfile()`——后者与根布局的外壳取数经 `cache()` 合并，同一次请求只发一次后端调用。

**登录墙分层不可合并**：`proxy.ts` 只挡结构性私有路径（读不到后端设置），内容页是否要登录由 `requireAuthBySetting` 在 RSC 层判定。详见 [app/AGENTS.md](../../app/AGENTS.md)。

## 凭据形态

会话凭据是 **HttpOnly cookie**（`master_session`，后端 `pkg/cookie.go`）。JS 只读非 HttpOnly 的 `master_auth` 标志位——**前端拿不到 sid 本身**，也没有 localStorage token。

旧文档里的 `master_access_token` / `master_refresh_token` 是会话迁移前的名字，现在只剩 `master_session`。

## 改这里要注意

- 加任何新导出到 `index.ts` 前确认它不 import `next/headers` / `next/navigation` 之外的服务端专属模块。
- `?redirect=` 只接受站内相对路径（拒绝 `//host` 与 `/\host`），这是防开放重定向的唯一关口。
- 状态机新增状态要同时改：`AuthState` 接口、`authSession` 门面、`providers.tsx` 的首帧注入、以及消费 `status` 的组件（`components/auth/*`、`components/layout/*`）。
- `sync.ts` 的广播是跨标签一致性的唯一机制；删掉它会让「一个标签登出、另一个标签仍显示已登录」。

## 本文件何时更新

- 状态机状态集合或 `authSession` 门面变化
- `guards.ts` 新增/修改服务端门
- 导入边界（barrel 导出）变化
- 凭据形态变化（cookie 名、是否 HttpOnly）
