# @feedchamber/tokens

Feed Chamber 视觉设计 Token 包。Web 端（`@feedchamber/web`）为视觉权威来源；Dashboard（`@feedchamber/dashboard`）消费同一套 token 以保持跨端视觉一致。构建产出静态 CSS + JS 产物，运行时零颜色算法依赖。

**单一可信来源：`w3c-tokens.json`**（W3C Design Tokens / DTCG 格式）。所有 token 值都以 `$value` / `$type` 声明，其余产物全部由构建脚本推导。修改 token 只需编辑 `w3c-tokens.json` 后重新构建，两个前端自动生效。

## 架构

```
w3c-tokens.json (W3C DTCG: color / dimension / shadow / duration / number / fontFamily / typography)
       |
       v
scripts/build.ts (tsx, 纯函数映射，无运行时依赖)
       |
       +-- dist/tokens.css      Tailwind v4 @theme + .dark 覆盖
       +-- dist/daisyui.css     daisyUI v5 orinlight/orindark 主题
       +-- dist/antd.js/.d.ts   Ant Design ThemeConfig（light/dark + getAntdTheme）
       +-- dist/layout.js/.d.ts Dashboard 布局 token（getLayoutTokens）
       +-- dist/dashboard.js/.d.ts Dashboard 专用 token
```

## 文件结构

```
packages/tokens/
├── package.json
├── tsconfig.json
├── w3c-tokens.json        # 单一来源：全部 token 的 $value/$type 声明
├── scripts/
│   └── build.ts           # 构建脚本：w3c-tokens.json → 7 个 dist 产物
└── dist/                  # 提交进仓库的生成产物（勿手改）
    ├── tokens.css
    ├── daisyui.css
    ├── antd.js / antd.d.ts
    ├── layout.js / layout.d.ts
    └── dashboard.js / dashboard.d.ts
```

## 构建

```bash
pnpm --filter @feedchamber/tokens tokens:build
```

## 修改 token 的流程

1. 编辑 `w3c-tokens.json`（保持 W3C DTCG 结构）。
2. 运行 `pnpm --filter @feedchamber/tokens tokens:build`。
3. 提交 `w3c-tokens.json` 与 `dist/` 产物。
