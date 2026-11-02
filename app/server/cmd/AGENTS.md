# Cmd CLI 入口

Cobra 命令行根命令，负责解析子命令并启动服务。

## 子命令

| 命令 | 说明 |
|------|------|
| `main.go -e dev` | 开发模式（env=Dev，reseed 等可执行）。**没有 `dev` / `prod` 子命令** |
| `main.go -e prod` | 生产模式（env=Prod，跳过 destructive 操作） |
| `main.go backup --out DIR` | 导出 DB 快照 + 上传清单 |
| `main.go import-discourse` | 导入 Discourse 数据 |
| `main.go recompute-xp` | 全量重算 XP / 等级（行为聚合，可重复执行；迁移与改规则后对账用） |

`recompute-xp` 直接构造 `repository.NewTrustRepository` + `service.NewTrustService`（SettingService 与 KV 都传 nil：Recompute 不读设置，CLI 也不需要广播 `trust:sync`），不需要 NATS 或 HTTP 服务。

## 配置加载

支持多环境 `.env` 文件：
- `.env` — 基础配置（始终加载）
- `.env.dev` — 开发环境覆盖
- `.env.prod` — 生产环境覆盖

> 2026-10-29：项目更名为 Feed Chamber（Go module `feedchamber`、npm `@feedchamber/*`、镜像 `feedchamber-*`）。
