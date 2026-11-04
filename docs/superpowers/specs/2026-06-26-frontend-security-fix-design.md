# Feed Chamber Frontend Security & Bug Fix — 2026-06-26

## Summary

修复代码审查发现的 CRITICAL/HIGH 安全漏洞与缺陷，选择性修复中危问题。

## Changes

### Batch 1: XSS 防护加固

| File | 修复内容 |
|---|---|
| `utils/safe-url.ts` | `normalizeUrl()` 增加 `decodeURIComponent` 反混淆，阻断 percent-encoded scheme 绕过（如 `%6a%61%76%61%73%63%72%69%70%74:`） |
| `utils/sanitize.ts` | DOMPurify 添加 `afterSanitizeAttributes` hook，二次检查 percent-encoded `javascript:`/`data:`/`vbscript:`/`blob:` scheme |

### Batch 2: 路由守卫

| File | 修复内容 |
|---|---|
| `pages/(app)/route.tsx` | 添加 `beforeLoad: () => requireAuth()`，所有 `(app)` 子路由统一受认证保护 |
| `pages/(app)/user/follow.tsx` | `throw new Error('Not authenticated')` → `throw redirect({ to: '/login', ... })` |
| `pages/(app)/user/follow.tsx` | `page_size: 1000` → `DEFAULT_PAGE_SIZE`，避免大关注列表拖慢 TTFB |

### Batch 3: SolidJS 反模式修复

| File | 修复内容 |
|---|---|
| `pages/(auth)/oauth/route.tsx` | `createEffect(async () => ...)` → `onMount(() => { (async () => ...)() })`，避免 Promise 被当作 cleanup 函数 |
| `pages/(app)/user/notifications.tsx` | `markRead()` 添加错误回滚，与 `markAllRead`/`deleteNotification` 行为一致 |
| `pages/(app)/timeline/index.tsx` | Loader 包裹 `try/catch`，API 故障返回空数据而非触发 ErrorBoundary |

### Batch 4: SSR Hydration 修复

| File | 修复内容 |
|---|---|
| `components/auth/AuthShell.tsx` | `new Date().getFullYear()` → 模块级 `CURRENT_YEAR` 常量 |
| `pages/(auth)/login/route.tsx` | 同上 |
| `pages/(auth)/register/route.tsx` | 同上 |

### Batch 5: 中危修复

| File | 修复内容 |
|---|---|
| `components/auth/Oauth.tsx` | `.catch(() => ({}))` → `.catch((err) => { console.error(...); return {} })`，网络故障可观测 |

## Not Fixed (deferred)

| 问题 | 原因 |
|---|---|
| 表单提交防重 | 需组件级重构，独立 PR |
| 上传无客户端大小校验 | 需配合后端限制 |
| ProfileEditForm 非 `<form>` | 独立 UI 改动 |
| `as any` 类型修复 | 需 API 类型完善配合 |
