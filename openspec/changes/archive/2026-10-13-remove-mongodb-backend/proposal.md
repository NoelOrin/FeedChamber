## Why

`app/server` 目前是「`db.Store` 抽象 + 每资源 SQL/Mongo 双实现」，支持 `DB_TYPE = sqlite | mysql | postgres | mongo`。双实现带来持续的维护成本：新增字段/查询需同步两套实现，模型层携带大量 `bson` 标签噪音，构建脚本与文档也需维护带 `-tags mongo` 的分支产物。

MongoDB 后端在实践中未被使用，团队决定**彻底移除 Mongo 兼容层**，只保留 SQL（sqlite/mysql/postgres），以降低认知与维护负担。

## What Changes

- **Repository 层**：38 个资源的工厂函数收敛为 SQL-only（`return &sqlXxxRepo{db: store.SQL()}`）；删除 18 个独立 `*_mongo.go` 与 20 个同文件共存的 `mongoXxxRepo` 实现；删除 `checker_mongo.go`、`MongoSoftDeleteFilter`/`SoftDeleteMerge`、`buildPostListFilterMongo`。
- **db 层**：`Store` 接口移除 `Mongo()`，删除 `BackendMongo`、`db/mongo.go`、`db/tx_mongo.go`、`db/collections.go`；`NewTransactionManager` 简化直连 SQL。
- **装配/监控/命令层**：`seed.go`、`backup_snapshot.go`、`handler.go`（health）、`metrics/collector.go`（注释）、`cmd/backup.go`、`config/config.go` 去除 mongo 分支。
- **Model 层**：删除全部 359+ 处 `bson:"..."` 标签（保留 `gorm:`/`json:`/`swaggertype:`）。
- **构建与依赖**：`go.mod` 移除 `go.mongodb.org/mongo-driver/v2` 及其独有传递依赖；`Makefile` 删除 `linux-sql`/`darwin-sql`/`linux-mongo`/`darwin-mongo` 四个 target（无 tag 即 SQL 版）。
- **规范同步**：更新 `backend-service-design`、`layered-architecture`、`module-conventions`、`operations` 四个 spec 中的「Repository 双实现」「SQL/Mongo」「DB_TYPE 取值域」表述。

## Capabilities

### New Capabilities

<!-- 无新增 capability -->

### Modified Capabilities

- `backend-service-design` — Repository 由「双实现」改为「SQL-only 单实现」，`DB_TYPE` 收敛为 sqlite/mysql/postgres。
- `layered-architecture` — 依赖链末端由「SQL via GORM / Mongo via 官方驱动」改为「SQL via GORM」；Repository 层职责表述去双实现。
- `module-conventions` — 模块文件组织约定中 repository 层由「接口 + SQL + Mongo」改为「接口 + SQL 实现」。
- `operations` — `DB_TYPE` 环境变量取值域移除 mongo。

## Impact

- 代码：`app/server/internal/{repository,db,service,handler,metrics,config,model,container}`、`app/server/server/seed.go`、`app/server/cmd/backup.go`、`app/server/Makefile`、`app/server/go.mod`。
- 行为：不再支持 `DB_TYPE=mongo`；`DB_TYPE` 传入 mongo 将返回「unknown database backend」错误；老 mongo 备份文件因 `backend` 字段校验会显式报错（不再静默尝试恢复）。
- 文档：根 `AGENTS.md`、`app/server/AGENTS.md`、各层 `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/backend-overview.md`。
- 不改动：`server/iris.go`、`Dockerfile`、`deploy/*.yml`（本就无 mongo 依赖）、历史归档文档。