---
title: 本地开发
description: 在宿主机直接跑 Go 后端、React 前端与一个外部 NATS
---

本地模式和 Compose 的区别只有两点：**没有 nginx**（前后端各用各的端口），
以及**数据库默认 SQLite**（不用起 Postgres）。其余依赖一样，尤其是 NATS 不能省。

## 前置条件

| 需要 | 版本 | 检查 |
|------|------|------|
| Go | 1.25+ | `go version` |
| Node.js | 22+ | `node -v` |
| pnpm | 10+ | `pnpm -v` |
| NATS Server | 2.10+，**带 JetStream** | `nats-server --version` |

NATS 用 Docker 起也一样：

```bash
docker run --rm -p 4222:4222 -p 8222:8222 nats:2.10-alpine -js -m 8222
```

## 1. 装依赖

```bash
# 仓库根：前端 workspace 与后端没有共享依赖，但 pnpm 必须在根执行
pnpm install
```

## 2. 配置后端

后端按 `-e` 读取 `.env` 与 `.env.<env>`（后者覆盖前者），文件放在 `app/server/` 下。

```bash
cd app/server
cp .env.example .env.dev   # 没有 .env.example 时直接新建
```

最小配置：

```dotenv
PORT=8080
DB_TYPE=sqlite
NATS_URL=nats://127.0.0.1:4222
LOG_LEVEL=debug
LOG_FORMAT=text
```

- `DB_DSN` 留空 → SQLite 落在 `app/server/db/app.db`（相对后端工作目录）
- 想用 Postgres：`DB_TYPE=postgres` + `DB_DSN="host=127.0.0.1 user=… dbname=… port=5432 sslmode=disable"`
- 首次启动会 seed 管理员；想看固定密码就设 `ADMIN_PASSWORD=admin123`

## 3. 启后端

```bash
cd app/server
go run main.go -e dev
```

启动成功的标志：

```text
level=info msg="seeded 61 permissions"
level=info msg="permission cache loaded"
level=info msg="server listening" port=8080
```

首次启动还会创建表、seed 角色/权限/版面/标签与示例数据（dev 环境）。
`go run main.go dev` 会报 `unknown command`——环境只经 `-e` 选择。

## 4. 启前端

另开一个终端：

```bash
# 仓库根
pnpm dev:web
```

前端默认连 `http://localhost:8080`。打开终端里打印的地址（通常是
<http://localhost:3000>），登录 `admin` + 你在第 2 步设的密码。

## 5. 验证

| 检查 | 期望 |
|------|------|
| `curl -s localhost:8080/api/v1/health` | `code: 0` |
| 打开前端 → 时间线 | 帖子列表能加载 |
| 登录 → `/admin` | 运营概览可见 |
| 发一条帖子 | 时间线立刻出现（默认 `async` 审核） |

## 常用命令

```bash
# 后端（仓库根）
pnpm dev:server

# 前端 + 依赖包
pnpm dev:web

# 文档站
pnpm dev:docs

# 全量 API 回归（需要后端已在 8080 跑着）
bash feedchamber_test.sh -all

# 格式化 / lint / 类型检查
pnpm check
```

## 重置本地数据

```bash
rm -f app/server/db/app.db          # SQLite
docker exec -it <postgres> psql -U feedchamber -c 'drop schema public cascade; create schema public;'
```

删库后下次启动会重新 seed，管理员密码回到 `ADMIN_PASSWORD` 或随机值。

## 常见失败

| 现象 | 原因 | 处理 |
|------|------|------|
| `NATS_URL is required` | 没设或 NATS 没起 | 先起 `nats-server -js`，再启后端 |
| `failed to initialize database` | DSN 错 / Postgres 没起 | 用 `psql` 先验证连通性 |
| 前端 401 循环 | 后端没起 / 端口不对 | 看浏览器 Network 里 `/api/v1` 的响应 |
| cookie 不生效 | 开发用 `localhost` 之外的主机名 | 用 `localhost`，或临时 `MASTER_COOKIE_SECURE=false` |
| 端口冲突 | 8080 / 3000 被占用 | `PORT=8081`；前端端口看 vinext 输出 |

更多见 [排障](/operations/troubleshooting/)。
