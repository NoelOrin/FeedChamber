# Shared Utils 跨前端共享工具

Web 和 Dashboard 共同依赖的工具函数包。

## 函数

| 函数 | 说明 |
|------|------|
| `formatSmartDate(date)` | 相对时间格式化（"3 分钟前"、"2 小时前"、"昨天"） |
| `formatTime(date)` | `YY-MM-DD HH:mm:ss` 格式化 |
| `cx(...classes)` | Tailwind CSS class 合并（clsx + tailwind-merge） |
| `sortCx(...classes)` | `cx()` 的排序版本，合并 + 排序 |
| `isAuthenticated()` | 检查 localStorage 中是否有 access_token |
| `getAccessToken()` | 获取 access_token |
| `setTokens(access, refresh)` | 设置 access + refresh token |
| `clearAuthTokens()` | 清除所有 token |

## 设计

（FE-14 修复）Web 和 Dashboard 原本各自维护一套认证逻辑，现统一提取到 `shared-utils` 作为纯函数共享。
