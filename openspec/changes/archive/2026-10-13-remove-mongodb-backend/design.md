# Design

## 背景

`db.Store` 抽象同时服务 SQL（GORM）与 MongoDB（官方驱动）两套后端。每资源在 `repository/` 下提供 `sqlXxxRepo` 与 `mongoXxxRepo` 两个实现，工厂函数按 `store.SQL() != nil` 分派。

## 决策

1. **保留 `Store` + `SQL()`，删除 `Mongo()` 与 `BackendMongo`。**
   `SQL()` 被工厂、seed、handler、backup_snapshot 与测试广泛使用；把 `Store` 压平成具体 `sqlStore` 纯属改动噪音，收益低。

2. **工厂只删分支、不重命名。**
   `func NewXxxRepository(store db.Store) XxxRepository { return &sqlXxxRepo{db: store.SQL()} }`。保留 `sqlXxxRepo` 命名与 `db` 字段名，避免测试与引用连锁改动。

3. **build tag 收敛。**
   删除 `checker_mongo.go`，去掉 `checker_sql.go` 的 `//go:build !mongo`，共享声明（`notFoundChecker`/`registerNotFoundChecker`/`WrapDBErr`）继续留在无 tag 的 `errors.go`。不再需要按 tag 选择实现。

4. **`TransactionManager` 保留抽象、简化实现。**
   `NewTransactionManager(store)` 直接 `return &sqlTxManager{db: store.SQL()}`；`txContextKey` 与 `WithTx` 语义不变。

5. **备份兼容性校验保留。**
   `restoreBackupSnapshot` 保留 `snap.Backend != store.Type()` 校验：老 mongo 备份的 `backend` 字段为 `mongo`，恢复时会显式报错而非静默误操作。

6. **构建产物收敛为无 tag。**
   移除 `-tags sql`/`-tags mongo` 变体 target；无 tag 即 SQL 版。

## 备选方案

- **压平 `Store` 为具体 `sqlStore`**：改动面大、收益小，放弃。
- **保留 mongo 代码仅移除接线**：未达成「消除维护成本」目标，放弃。

## 风险与缓解

| 风险 | 缓解 |
| --- | --- |
| 删除 mongo 段误删相邻 SQL 代码 | 先核验每个文件 `type mongoXxxRepo` 至 EOF 段内 receiver 全为该 mongo 结构 |
| 模型去 tag 误删 gorm/json | 仅匹配 ` bson:"..."` 片段，抽样复核 `user.go`/`post.go` |
| 文档漂移 | 同步 4 个 spec 与各层文档，纳入本 change |