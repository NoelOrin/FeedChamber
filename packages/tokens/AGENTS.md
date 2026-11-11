# Tokens 视觉设计 Token

视觉设计 Token 系统。单源真理，生成 CSS 和 JS 产物。**现状消费者只有一个：`app/web`（前台 + 内嵌 `/admin` 后台），经 `globals.css` 引用 `shadcn.css`（Tailwind v4）。** `antd.js` 产物是已删除的独立 Dashboard 的历史兼容残留，2026-11-11 实测无消费方。

## 技术栈

- W3C Design Tokens (DTCG) — `w3c-tokens.json` 为唯一可信来源
- TypeScript + tsx — 构建脚本运行环境
- `pnpm --filter @feedchamber/tokens tokens:build` — 生成 dist/ 输出

## 结构

```
w3c-tokens.json      — W3C DTCG 单一来源：全部 token 的 $value/$type (color/dimension/shadow/duration/number/fontFamily/typography)
scripts/
  build.ts           — 构建脚本
dist/
  tokens.css         — Tailwind v4 @theme tokens + .dark 覆盖
  daisyui.css        — daisyUI v5 orinlight/orindark 主题
  antd.js + .d.ts    — Ant Design ThemeConfig（JS + TS 声明）
  layout.js + .d.ts  — 布局 token
  dashboard.js+.d.ts — Dashboard 专用 token
```

## 产物消费方

| 产物 | 消费方 |
|------|--------|
| `tokens.css` | Web 前端（Tailwind CSS） |
| `daisyui.css` | Web 前端（daisyUI 主题） |
| `antd` | **无消费方**（历史兼容产物；旧独立 Dashboard 已删除，新代码不要再接） |
| `layout` | Web 前端布局 token |
| `dashboard` | Dashboard 专用 token |

## 构建

```bash
cd packages/tokens && pnpm build
```
