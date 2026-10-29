# Packages 前端共享包

pnpm workspace 里的 4 个前端共享包，被 `app/web` 与（历史遗留的）Dashboard 消费。**单一职责、可独立构建**。

另有 1 个**非前端**的工具包 `loadtest/`（k6 压测脚本），不进构建链、不被任何应用 import——它只是被 `pnpm --filter @orinforum/loadtest` 调用的运行脚本。

> 上级：[根 AGENTS.md](../AGENTS.md) · 消费方：[app/web](../app/web/AGENTS.md)

## 包一览

| 包 | 目录 | 职责 | 文档 |
|----|------|------|------|
| `@orinforum/api-client` | `api-client/` | 唯一请求层：`authFetch` + 401 单飞刷新 + 全部 service + 类型 + 常量 + SSR adapter | [api-client/AGENTS.md](api-client/AGENTS.md) |
| `@orinforum/sse-client` | `sse-client/` | 框架无关 SSE 客户端（Bearer 鉴权、自动重连、401 刷新） | [sse-client/AGENTS.md](sse-client/AGENTS.md) |
| `@orinforum/shared-utils` | `shared-utils/` | 时间格式化 + `cx()` class 合并 | [shared-utils/AGENTS.md](shared-utils/AGENTS.md) |
| `@orinforum/tokens` | `tokens/` | 视觉 token 生成（DTCG → CSS / JS） | [tokens/AGENTS.md](tokens/AGENTS.md) |
| `@orinforum/loadtest` | `loadtest/` | k6 匿名只读压测：端点目录 / smoke 自检 / 千级阶梯 / Markdown 报告（**工具包，无 build、无 dist**） | [loadtest/AGENTS.md](loadtest/AGENTS.md) |

## 构建顺序（turbo 的 `^build` 依赖）

```
tokens / shared-utils / sse-client   （无内部依赖，可并行）
        ↓
   api-client                        （无内部依赖，但 Web 依赖它）
        ↓
     app/web
```

`turbo run build` 的 `dependsOn: ["^build"]` 保证顺序。**CI 的 typecheck job 必须先 `turbo run build --filter='./packages/*'`**，否则 `app/web` 的 tsconfig 解析不到 `@orinforum/*` 的类型（它们指向 `dist/*.d.ts`）。

## 通用约定

- **包名统一 `@orinforum/<name>`**，全部 `private: true`，不发布。
- **`exports` 显式声明**（不用隐式 main）：`api-client` 有 `.`、`./adapters/web`、`./adapters/dashboard` 三个入口。
- **产物在 `dist/`，被 `.gitignore` 忽略**——唯一例外是 `packages/tokens/dist/`，它是**提交进仓库的生成产物**（见 `tokens/AGENTS.md`）。
- **每个包有自己的 `biome check ./src`**，根 `pnpm lint` 经 turbo 全量跑；`loadtest` 没有 `src`，它只被 `biome check packages/loadtest` 覆盖，不挂 lint 脚本。
- **不要在包里 import `app/web`**：依赖方向单向，包不知道宿主的存在。需要宿主提供的能力用 adapter 注入（见 `api-client/src/adapter.ts`）。

## 改这里要注意

- **改 `api-client` 的公开导出**会同时影响 `app/web` 的所有页面与后台：先 `pnpm --filter @orinforum/api-client build`，再 `cd app/web && pnpm exec tsc -p tsconfig.check.json`。
- **`tokens/dist/` 是提交的产物**：改 `w3c-tokens.json` 后必须重跑构建并提交 `dist/`，否则 CI 的 biome 与使用方会看到旧值。
- **`sse-client` 必须保持框架无关**：它不认识 React/vinext，宿主经 `lib/hooks/use-notification-sse.ts` 包一层。
- 新增包要在 `pnpm-workspace.yaml`（已覆盖 `packages/*`）与 turbo 的 `--filter` 里生效，通常无需改动；但要建自己的 `AGENTS.md` 并在本文件加一行。

## 本文件何时更新

- 新增/删除一个共享包
- 包之间的构建依赖顺序变化
- 通用约定（命名、exports、产物提交策略）变化
