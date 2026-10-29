# deploy/

容器编排。**在仓库根目录**执行本节所有命令。

## 一键启动（默认档）

```bash
docker compose -f deploy/docker-compose.yml up -d --build
```

打开 <http://localhost>。编排档自带两个依赖服务：

| 组成 | 说明 |
|------|------|
| 数据库 | PostgreSQL 16，数据在 named volume `orin_pg` |
| KV / 事件 | NATS 2.10（`-js` 启用 JetStream），数据在 named volume `orin_nats` |
| 上传 | named volume `orin_uploads` |
| 入口 | nginx，`/` → web SSR（匿名页面带 60s 共享缓存），`/api/v1/` → backend |

后端进程内嵌的 nats-server 已剥离：`NATS_URL` 必须指向外部 NATS，
为空时后端启动即失败（fail fast），不再静默降级成只有本进程可见的存储。

数据库默认账号/库名可用环境变量覆盖：
`POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB`（默认均为 `orinforum`）。

所有变量都有默认值，不建 `.env` 也能起。要覆盖时先复制样例：

```bash
cp deploy/.env.example deploy/.env    # 然后按注释改
```

生产至少要配 `POSTGRES_PASSWORD`、`SITE_URL`、`MASTER_COOKIE_SECURE` 三项，
样例里用 ⚠ 标出。

初始管理员密码：设了 `ADMIN_PASSWORD` 就用它，否则随机生成并打印：

```bash
docker compose -f deploy/docker-compose.yml logs backend | grep -i admin
```

改端口：`HTTP_PORT=8088 docker compose -f deploy/docker-compose.yml up -d`

## 集群 / 外部监控

仓库只维护 `docker-compose.yml` 这一个标准档。需要完整监控栈时，自行接入外部服务：

- **指标**：backend 在容器网络内暴露 `METRICS_PORT`（默认 9091），刻意不映射到宿主机。
- **采集**：把 `prometheus.yml` 与 `prometheus/rules/` 挂到外部 Prometheus 即可，
  抓取目标为 `backend:9091`，告警规则随文件提供。
- **面板**：`grafana/` 下是 provisioning + 数据源/dashboard 自动装配，挂到外部 Grafana。

这些文件是可选素材，仓库里没有任何编排档会自动拉起它们。

## 文件

```
deploy/
├── docker-compose.yml           # 唯一的编排档：PostgreSQL + NATS + nginx，一键可起
├── nginx/nginx.conf             # 统一入口；透传 X-Request-ID 便于日志关联
├── prometheus.yml               # 可选：抓 backend:9091，并经 blackbox 探 health
├── prometheus/rules/            # 可选：告警规则
├── blackbox/blackbox.yml        # 可选：health 探针模块
├── grafana/                     # 可选：provisioning + dashboard
└── .env.example                 # 环境变量样例
```

## 已知约束

- **两个镜像的构建上下文都是仓库根**（`context: ..`）。`app/server/Dockerfile`
  与 `app/web/Dockerfile` 的 COPY 路径、根 `.dockerignore` 都按仓库根书写；
  web 镜像还需要根部的 `pnpm-lock.yaml` / `pnpm-workspace.yaml` / `packages/`。
- **9091（指标）不映射到宿主机**，nginx 也不代理它（端点无鉴权）。外部 Prometheus
  需在同一容器网络内直连。
- **80 必须用 `ports` 映射到宿主机**，不能改成 `expose`（那样没人监听，站点打不开）。
  前面加反代时改绑 `127.0.0.1:${HTTP_PORT:-80}:80`。
- 后端镜像的 `WORKDIR` 固定为 `/app/server`，`db` 与 `uploads` 因此稳定落在
  被挂载的两个 volume 上。
- 前端 SSR 用 `API_BASE_URL` 直连 `backend:8080`；浏览器侧走同源 `/api/v1`
  由 nginx 转发，两条链路不共用同一个地址。
- **线上镜像来自 CI，不在服务器构建**：push 到 `develop` 后
  `.github/workflows/docker-build.yml` 构建 `orinforum-server` / `orinforum-web`
  并推 GHCR，随后调 Dokploy 部署 hook；Dokploy 档 compose 只有 `image:`，
  部署等价于 `docker compose pull && up -d`。回滚：`.env` 里设
  `IMAGE_TAG=sha-<短哈希>` 再部署一次。
- **匿名 SSR 页面在 nginx 里缓 60s**（`proxy_cache_path` → `/var/cache/nginx/orinforum`）。
  响应头 `X-Cache-Status` 直接暴露命中情况：`HIT` / `MISS` / `BYPASS`。
  带会话 cookie、RSC/Next-Router-* 预取请求、`/login` `/admin` 等非公开路径一律
  `BYPASS`，不读也不写缓存——登录态页面绝不进共享缓存。改这块前先读
  `nginx/nginx.conf` 顶部的注释与 `AGENTS.md` 的缓存约束。
