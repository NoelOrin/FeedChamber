# NATS 部署指南

## 端口

| 端口 | 用途 | 启用场景 |
|------|------|---------|
| 4222 | 客户端连接 (Go/JS SDK) | 所有部署 |
| 8222 | HTTP 监控仪表盘 | 所有部署 |
| 6222 | 集群路由 | cluster.yml |
| 8443 | WebSocket（前端直连） | 可选 |

## 部署方案

项目提供 2 档 docker-compose，覆盖单机到集群：

| 档位 | 配置 | 文件 |
|------|------|------|
| 默认（一键） | PostgreSQL + NATS（JetStream）+ nginx，依赖服务由本档一并拉起 | `deploy/docker-compose.yml` |

仓库只保留这一个编排档；原集群档（PostgreSQL + NATS 三节点 + 监控栈）已删除，
需要多实例时自行接入外部 PostgreSQL 与 NATS 集群，指标走容器网络内的 9091。

运行：

```bash
# 在仓库根执行
docker compose -f deploy/docker-compose.yml up -d --build
```

原 `lite` / `basic` / `full` 三档与 `cluster` 档均已删除：`lite` 与默认档一致，`basic` / `full` / `cluster`
只在前半段重复或叠加外部依赖。Redis 与 `KV_PG_DSN` 一并删除——后端从未读取它们（KV 唯一实现是 NATS JetStream）。

## 开发环境

### 方式 A：Docker 单容器运行（推荐）

```bash
docker run -d --name orin-nats \
  -p 4222:4222 -p 8222:8222 \
  nats:latest -js -m=8222
```

验证：

```bash
open http://localhost:8222    # 监控页面
```

### 方式 B：Docker Compose 开发专用

```yaml
# deploy/dev.yml
services:
  nats:
    image: nats:latest
    command: -js -m=8222
    ports:
      - "4222:4222"
      - "8222:8222"
    volumes:
      - nats_data:/data
volumes:
  nats_data:
```

```bash
docker compose -f deploy/dev.yml up -d
```

### 环境变量

在 `.env` 或 docker-compose environment 中配置：

```bash
# 必填：外部 NATS，必须启用 JetStream（-js）。
# 为空时后端启动即失败（config.Validate 拦截），不再回退内嵌。
NATS_URL=nats://localhost:4222
NATS_BUCKET=feedchamber_kv
```

## 生产部署

### NATS 配置文件

推荐使用配置文件而非纯 CLI 参数：

```conf
# deploy/nats.conf
port: 4222
http_port: 8222
jetstream: true
store_dir: /data

cluster {
  port: 6222
  name: orin
  routes: [
    nats://nats-2:6222
    nats://nats-3:6222
  ]
}

# 可选：WebSocket 支持
websocket {
  port: 8443
  no_tls: true
}
```

### 集群部署

仓库不再提供集群编排档。多实例部署需自备：

- NATS 3 节点集群（各自独立持久化卷，`--routes` 互相指向）
- 外部 PostgreSQL（`DB_TYPE=postgres`），替代默认档的 SQLite
- 监控：backend 在容器网络内暴露 `METRICS_PORT`(9091)；`deploy/prometheus.yml` 与 `deploy/grafana/` 是现成的采集/面板素材，挂到外部实例即可

### K8s 部署

推荐 NATS Helm Chart 或 NATS Operator：

```bash
helm repo add nats https://nats-io.github.io/helm-charts
helm install nats nats/nats \
  --set jetstream.enabled=true \
  --set replicaCount=3
```

Operator 方式支持自动集群和故障恢复：

```bash
kubectl apply -f https://github.com/nats-io/nats-operator/releases/latest/operator.yaml
```

## 外部 NATS 模式

- `NATS_URL` **必填**：连接外部 NATS（需 `-js`）；为空时启动失败，不回退内嵌。
- 连接失败时进程直接报错，不做任何降级。
- 多副本必须配置同一个 `NATS_URL`，共享 KV（会话缓存 / 限流窗口）与事件队列。

进程内 nats-server 已于 2026-10-28 剥离：KV 与 EventBus 的生命周期不该绑在应用
进程上——进程重启会丢 JetStream 队列，多实例也无法共享会话缓存与限流窗口。
`internal/pkg/natsembedded` 包保留仅供单元测试（`internal/pkg/testnats`）。

## 调试工具

```bash
# 安装 NATS CLI
brew install nats-io/nats-tools/nats

# 常用命令
nats stream ls                    # 查看 JetStream streams
nats kv ls                        # 查看 KV buckets
nats kv get feedchamber_kv <key>         # 读 KV 值
nats server check connection      # 连接健康检查
nats consumer ls                  # 查看消费者
```

## 注意事项

- NATS 必须启用 `-js`（JetStream），否则 KV store 不可用
- 大消息（>1MB）会降低吞吐，不适合传 blob
- 集群使用 gossip 协议，大规模集群建议 super-cluster 拓扑
- 生产环境必须配置 NATS 认证（token/JWT）
- WebSocket 端口 8443 仅在前端需要直连 NATS 时开启
