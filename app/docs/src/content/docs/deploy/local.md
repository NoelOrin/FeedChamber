---
title: 本地开发
description: 不用 Docker 起后端与前端，含 NATS 依赖
sidebar:
  order: 2
---

## 前置

| 需要 | 版本 |
|------|------|
| Go | 1.25+ |
| Node | 22+ |
| pnpm | 10+ |
| NATS Server | 任意近期版本（需支持 JetStream） |

## 起 NATS

```bash
nats-server -js
```

没有的话装一个：`brew install nats-server`（macOS）。

:::caution[NATS 必填]
后端启动时 `NATS_URL` 为空会直接失败。这是刻意的 fail fast——
进程内嵌的 nats-server 已剥离，KV 与事件总线的生命周期不该绑在应用进程上。
:::

## 起后端

```bash
cd app/server
NATS_URL=nats://127.0.0.1:4222 go run main.go -e dev
```

`-e dev` 选择环境，读取 `.env` 与 `.env.dev`（后者覆盖前者）。
本地默认 SQLite，库文件在 `app/server/db/app.db`。

热重载（可选）：

```bash
pnpm dev:server     # 走 air，改代码自动重启
```

## 起前端

```bash
pnpm dev:web        # 含 /admin
```

默认连 `http://localhost:8080`。

## 起文档站

```bash
pnpm dev:docs       # http://localhost:4321
```

## 常见操作

```bash
# 全量 API 测试（建议在新库上跑）
cd app/server && DB_TYPE=sqlite DB_DSN=/tmp/of_test.db go run main.go -e dev &
sleep 4
bash ../feedchamber_test.sh -all

# 重新生成 swagger 与 openapi.json
cd app/server && make swag

# 手动重算全量 XP / 等级
cd app/server && go run main.go -e prod recompute-xp
```

:::tip[环境选择没有子命令]
`go run main.go dev` 会报 `unknown command`。环境只经 `-e` 选，
真正的子命令只有 `backup` / `import-discourse` / `recompute-xp`。
:::
