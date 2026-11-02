---
title: OpenAPI
description: 规格从哪来到哪去：swag 注解、make swag、运行时 /swagger/、文档站下载、生成客户端与 CI 检查
---

接口规格是**生成物**，不是手写文档：唯一真相源是 handler 上的 swag 注解，`make swag`
把它编译成 JSON，仓库根的 `openapi.json` 是这份产物的副本。当前规模是
**313 个 path / 314 个操作**（278 个 `POST`、36 个 `GET`）、66 个定义、49 个 tag。

## 前置条件

- Go 工具链可用；`swag` 按 `go.mod` 版本安装：

```bash
cd app/server
make swag-install        # go install github.com/swaggo/swag/cmd/swag@latest
```

## 生成

```bash
cd app/server
make swag
```

这一条命令做两件事：

1. `swag init -g main.go -o docs --parseInternal` 扫描全部 handler 注解，产出
   `app/server/docs/`（`swagger.json`、`swagger.yaml`、`docs.go`）。
2. 把 `docs/swagger.json` 复制到仓库根 `openapi.json`。

`app/server/docs/` 与 `openapi.json` 都是**提交进仓库**的产物。改完接口只改代码不
提交产物，会让文档与实现长期脱节且没有任何编译错误提醒——所以有 CI 检查（见文末）。

## 规格内容

| 字段 | 值 |
|------|----|
| `swagger` | `"2.0"`（Swagger 2.0，不是 OpenAPI 3） |
| `info.title` | `Feed Chamber API` |
| `info.version` | `1.0` |
| `host` | `localhost:8080` |
| `basePath` | `/api/v1` |
| 安全定义 | `BearerAuth`：`apiKey`，位置 `header`，名字 `Authorization` |

注意 `basePath` 是 `/api/v1`，所以规格里的路径是**去掉前缀**的（`/posts/list` 而不是
`/api/v1/posts/list`）。生成的客户端默认会拼上前缀，`/health` 也正确落在
`/api/v1/health`。

一个例外要留意：爬虫与订阅端点（`/sitemap*.xml`、`/robots.txt`、`/latest.rss`）也带
注解，但它们实际挂在**应用根**。生成客户端后这七条路径的 URL 会多出 `/api/v1`，
直接抓会 404——用的时候单独处理，或者干脆不让客户端生成这几条。

`BearerAuth` 描述的是"往 `Authorization` 头里塞 `Bearer <sid 或 ak_...>`"。会话 sid
与 API Key 都用这一个头，区别只在值的前缀，详见 [认证与权限](/api/authentication/)。

## 三个出口

| 出口 | 位置 | 用途 |
|------|------|------|
| 仓库根文件 | `openapi.json` | 生成客户端、导入 API 工具、做 diff |
| 运行时 UI | `http://localhost:8080/swagger/` | 手工试接口，读的是 `/swagger/doc.json` |
| 文档站下载 | 站内 `/openapi.json` | 给读者直接下载当前版本的规格 |

运行时 UI 由服务自身注册（`/swagger/{any:path}`），不需要额外起服务，也不依赖构建
产物之外的步骤。文档站的副本由 `prebuild` / `predev` 跑
`scripts/sync-openapi.mjs` 从仓库根复制到 `app/docs/public/openapi.json`（该文件
不入库）——所以它永远与仓库根那份同版本，不会各自漂移。

## 生成客户端

规格是 Swagger 2.0，主流生成器都吃这个格式：

```bash
npx @openapitools/openapi-generator-cli generate \
  -i openapi.json -g typescript-fetch -o ./src/api
```

生成后必须改两处，否则本地能跑、线上全错：

- **base URL**：`host` 是 `localhost:8080`，生产要换成自己的域名（浏览器里通常用同源
  相对路径 `/api/v1`）。
- **认证**：`BearerAuth` 是 `apiKey` 类型，生成器会给你一个"设置 API key"的方法，
  传值时要自己带上 `Bearer ` 前缀（描述里写的就是这个形式）。

另外，生成出来的模型只覆盖 `definitions` 里的 66 个结构；分页响应统一是
`{items,total}`，错误统一是 `{code,msg,data}`，这两层信封生成器不会替你处理。

## CI 检查

```bash
cd app/server
make swag-check
```

它是 `make swag` + `git diff --quiet`：重新生成一遍，如果 `docs/` 或 `openapi.json`
有变化就失败，并打印差异。CI 的 `quality.yml` 先按锁定版本安装 `swag`，再跑这条
命令——所以"改了注解没重新生成"会在 PR 上被拦住，而不是等到线上文档出错。

## 什么时候要重新生成

| 改动 | 需要 `make swag` |
|------|------------------|
| 新增或删除路由 | 是 |
| 改 `@Param` / `@Success` / `@Router` / `@Tags` / `@Security` | 是 |
| 改请求体或响应结构体的字段与 JSON tag | 是 |
| 只改 service 内部实现 | 否 |
| 改注释文案但不影响注解 | 否 |
| 新增 `/mcp` 这类协议端点 | 否——它不套信封，也没有 swag 注解 |

`/mcp` 刻意不在规格里：JSON-RPC 的方法与工具清单由协议自身在运行时发现，写进
OpenAPI 只会造出一份必然过期的副本。见 [MCP 集成](/api/mcp/)。

## 验证

```bash
# 规格与仓库同步
cd app/server && make swag-check

# 运行时 UI 能打开，且 doc.json 可解析
curl -s http://localhost:8080/swagger/doc.json | jq '.info, (.paths | length)'
```

文档站构建后应能在产物根取到 `openapi.json`；`prebuild` 找不到仓库根那份时会直接
失败并提示先跑 `make swag`。

## 常见失败

| 现象 | 原因 | 处理 |
|------|------|------|
| `make swag: swag: command not found` | 没装 CLI | `make swag-install` |
| CI 报 "swagger docs are stale" | 改了注解没提交产物 | 本地 `make swag` 后提交 `docs/` 与 `openapi.json` |
| `/swagger/` 页面空白 | 静态资源路径被代理吃掉 | 确认 `/swagger/` 转发到后端 |
| 生成客户端的请求全 404 | 没拼 `basePath`，或把根级 SEO 路径也拼了前缀 | 业务接口加 `/api/v1`，SEO 路径不加 |
| 生成的类型缺字段 | 字段没导出或没有 JSON tag | 补 tag 后重新生成 |
