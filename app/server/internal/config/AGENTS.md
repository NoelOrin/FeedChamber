# Config 配置

环境变量加载，启动时执行 `Load()` 返回 `Config` 结构。

## 字段

| 字段 | 环境变量 | 默认值 | 说明 |
|------|----------|--------|------|
| Port | PORT | "8080" | 服务端口 |
| DBType | DB_TYPE | "sqlite" | 数据库类型 |
| DBDSN | DB_DSN | "" | 数据库连接字符串 |
| Storage.Provider | STORAGE_TYPE | "local" | 存储类型 |
| Storage.LocalDir | STORAGE_LOCAL_DIR | "uploads" | 本地存储目录 |
| Storage.Endpoint/Region/Bucket/AccessKey/SecretKey | S3_* | "" | S3 配置 |
| KV.NATSURL | NATS_URL | "" (**必填**) | 外部 NATS 地址，需启用 JetStream；为空则启动失败 |
| KV.NATSBucket | NATS_BUCKET | "feedchamber_kv" | JetStream KV bucket |
| PluginWatch | PLUGIN_WATCH | false | 插件子进程用 `node --watch` 拉起，改插件文件自动重启（IPC 不断）。仅开发期：watch 下崩溃不触发 supervisor 的退避重启 |

## 备注

- Storage 配置以 settings 表为来源，env 为回退（H-04 修复）。`cfg.Storage` 在启动时由 `pkg.BuildStorageConfig()` 填充。
- 不支持运行时热重载，修改 settings 后需重启服务。
