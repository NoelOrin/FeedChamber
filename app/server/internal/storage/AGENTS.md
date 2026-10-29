# Storage 文件存储抽象

Provider 接口抽象文件存储，支持本地和 S3 两种实现。

## 接口

```go
type Provider interface {
    Save(ctx, path, contentType string, reader io.Reader) error
    Delete(ctx, path string) error
    URL(ctx, path string) (string, error)
    PresignURL(ctx, path string, expiry time.Duration) (string, error)
}
```

## 实现

| 实现 | 驱动 | 选择条件 |
|------|------|---------|
| local | os 文件操作 | `STORAGE_TYPE=local` 或为空 |
| s3 | AWS SDK | `STORAGE_TYPE=s3` |

## 配置

Storage config 由 `pkg.BuildStorageConfig()` 按 settings 表 → env → 默认优先级构建（H-04 修复）。不支持热重载。
