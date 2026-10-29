# DB 数据库抽象层

`Store` 接口抽象 SQL 后端（GORM），工厂函数根据配置自动选择。

## 核心接口

```go
type Store interface {
    SQL() SQLStore      // 返回底层 GORM 句柄
    Type() BackendType  // sqlite/mysql/postgres
    Close() error
}
```

## 实现

| 后端 | 驱动 | 文件 |
|------|------|------|
| sqlite | GORM (go-sqlite3) | sql.go |
| mysql | GORM (mysql driver) | sql.go |
| postgres | GORM (postgres driver) | sql.go |

## 事务

`TransactionManager` 接口 + `WithTx(ctx, fn)` 统一事务抽象，基于 GORM `Transaction()`。

## 关键约定

- 实现通过 `r.db.DB().WithContext(ctx)` 操作
- Repository 工厂函数直接返回基于 `store.SQL()` 的 SQL 实现
- 不对外暴露存储细节，service 层只依赖接口
