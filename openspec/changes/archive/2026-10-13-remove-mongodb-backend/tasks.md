# Tasks

## 1. Repository 层收敛

- [x] 1.1 38 个资源工厂函数收敛为 `return &sqlXxxRepo{db: store.SQL()}`（不再保留 mongo 分支）
- [x] 1.2 删除 18 个独立 `*_mongo.go` 文件（user/post/content/follow/favorite/like/notification/group/role/badge/banned/user_mfa/user_preference/space_permission/user_block/watch/webhook/conversation）
- [x] 1.3 删除 20 个同文件资源末尾的 `mongoXxxRepo` 段
- [x] 1.4 删除 `checker_mongo.go`，去掉 `checker_sql.go` 的 `//go:build !mongo` tag
- [x] 1.5 删除 `softdelete.go` 的 `MongoSoftDeleteFilter`/`SoftDeleteMerge`、`post_filter.go` 的 `buildPostListFilterMongo`
- [x] 1.6 清理 mongo/bson import 与残留段注释、死代码（`mongoEscapeRegex` 等）

## 2. db 层瘦身

- [x] 2.1 `db.go` 移除 `Mongo()`/`BackendMongo` 与 `case BackendMongo`，删除 `context` import
- [x] 2.2 删除 `db/mongo.go`、`db/tx_mongo.go`、`db/collections.go`
- [x] 2.3 `sql.go` 移除 `Mongo()` 方法；`tx.go` 的 `NewTransactionManager` 直连 SQL
- [x] 2.4 `poolstats_test.go` 移除 `mockPoolStatsStore.Mongo()`

## 3. 装配 / 监控 / 命令层

- [x] 3.1 `server/seed.go` 去 mongo 分支与 mongo-only 函数，SQL 逻辑无条件化
- [x] 3.2 `internal/service/backup_snapshot.go` 直连 SQL，保留 `snap.Backend != store.Type()` 兼容校验
- [x] 3.3 `internal/handler/handler.go` 健康检查移除 `case db.BackendMongo`
- [x] 3.4 `internal/metrics/collector.go` 仅改注释，保留 `stats == nil` 守卫
- [x] 3.5 `cmd/backup.go` 提示文案去 `mongodump`
- [x] 3.6 `internal/config/config.go` 类型注释与 `Validate()` 去 mongo

## 4. Model 层

- [x] 4.1 删除全部 `bson:"..."` 标签（保留 gorm/json/swaggertype）

## 5. 构建与依赖

- [x] 5.1 `go.mod` 移除 mongo-driver，`go mod tidy` 清理传递依赖
- [x] 5.2 `Makefile` 删除 `linux-sql`/`darwin-sql`/`linux-mongo`/`darwin-mongo` 及 `.PHONY` 项

## 6. 测试

- [x] 6.1 `config_test.go` 类型列表去 mongo
- [x] 6.2 `errors_test.go` 删除 Mongo 用例
- [x] 6.3 `post_filter_test.go` 删除 Mongo 用例与 bson import

## 7. 文档与规范

- [x] 7.1 新建 openspec change（本目录）
- [x] 7.2 更新根 `AGENTS.md`、`app/server/AGENTS.md`、各层 `AGENTS.md`
- [x] 7.3 更新 `docs/ARCHITECTURE.md`、`docs/backend-overview.md`

## 8. 验证

- [x] 8.1 `go build ./...` / `go vet ./...` / `go test ./...` 全绿
- [x] 8.2 `grep -rni mongo app/server --include=*.go` = 0；`grep -rn 'bson:"' internal/model` = 0
- [x] 8.3 `grep -rn "MongoStore\|BackendMongo\|mongo-driver" app/server` = 0
- [x] 8.4 sqlite 冒烟启动成功
- [x] 8.5 `make clean && make darwin` 目标收敛后可用