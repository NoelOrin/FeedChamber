---
title: CLI
description: 服务端二进制与环境标志，以及 backup、import-discourse、recompute-xp 三个子命令
---

后端入口是 Cobra 根命令，**没有 `dev` / `prod` 子命令**：环境只通过 `-e` 选择。

```bash
go run main.go -e dev      # 读取 .env + .env.dev
go run main.go -e prod     # 读取 .env + .env.prod
go run main.go dev         # ✗ unknown command
```

`.env` 先加载，`.env.<env>` 后加载并覆盖同名变量。参数在 `-e` 之后直接报错，
因为根命令不接受位置参数。

## 子命令

| 命令 | 用途 | 必填参数 |
|------|------|----------|
| `backup` | 导出本地站点备份（DB 快照 + 上传清单） | `--out` |
| `import-discourse` | 导入 Discourse JSON 导出目录 | `--dir` |
| `recompute-xp` | 按当前规则全量重算 XP 与等级 | 无 |

### `backup`

```bash
go run main.go -e prod backup --out /backup/2026-10-30
```

产物：

| 文件 | 说明 |
|------|------|
| `app.db` | **仅 SQLite**：数据库文件副本 |
| `db-note.txt` | MySQL / PostgreSQL：提示用 `mysqldump` / `pg_dump` 自行导出 |
| `uploads-manifest.json` | 上传目录的文件清单（相对路径、大小、mtime） |
| `meta.json` | 创建时间、DB 类型、Go 版本、上传文件数 |

:::caution[不是完整备份]
这个命令只复制 SQLite 文件并生成上传清单，**不打包上传文件本身**，也不会为
MySQL / PostgreSQL 生成 dump。生产环境请配合 volume 快照或引擎原生工具，
见 [备份与恢复](/administration/system/)。
:::

### `import-discourse`

```bash
go run main.go -e prod import-discourse \
  --dir /import/discourse-export \
  --source discourse
```

`--dir` 下需要四个文件：`users.json`、`categories.json`、`topics.json`、`posts.json`。
`--source` 写入 `import_maps` 表作为来源键，重复导入时用于去重；默认 `discourse`。

映射关系：Discourse 分类 → 版面，topic → 帖子，post → 评论；`reply_to_post_number`
还原嵌套回复。导入是**幂等**的（按 `import_maps` 跳过已导入对象），但不会覆盖
本地已修改的内容。

### `recompute-xp`

```bash
go run main.go -e prod recompute-xp
# recomputed 1234 users
```

XP 是行为聚合派生的，重算不会重复计分。用途：存量迁移、改过 XP 规则或等级定义后的对账、
发现某个用户等级异常时的手工修复。服务运行中也可以执行——它只写
`users.total_xp` / `trust_level` / `level_metrics`。

## 构建

```bash
cd app/server

make build          # 当前平台
make linux          # 交叉编译 linux/amd64
make darwin
make windows
make swag           # 重新生成 Swagger 文档并同步 openapi.json
make swag-check     # CI 用：swagger 是否过期
```

二进制默认输出到 `app/server/build/`。`make swag` 之后记得提交 `openapi.json`：
文档站的[端点索引](/reference/endpoints/)就是读它生成的。

## 退出行为

启动期的配置错误一律 `Fatal`，进程直接退出而不是降级运行：

| 场景 | 输出 |
|------|------|
| `NATS_URL` 为空 | `NATS_URL is required: KV Store and EventBus need an external NATS…` |
| `DB_TYPE` 不支持 | `unsupported DB_TYPE "…"` |
| `mysql`/`postgres` 缺 DSN | `DB_DSN is required when DB_TYPE=…` |
| 指标端口越界 | `METRICS_PORT must be between 1 and 65535…` |
| 管理员密码未设置 | 不退出，随机生成并 WARN 打印 |
