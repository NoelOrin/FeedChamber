# 部署编排（`deploy/`）

容器编排与反向代理配置。**所有命令都在仓库根执行**（不是 `deploy/` 内）。

> 上级：[根 AGENTS.md](../AGENTS.md) · 运行说明：[README.md](README.md) · 后端：[app/server/AGENTS.md](../app/server/AGENTS.md)

## 文件表

| 文件 | 说明 |
|------|------|
| `docker-compose.yml` | 默认档：postgres 16 + nats 2.10(`-js`) + nats-exporter + backend + frontend + nginx；named volumes `feedchamber_pg` / `feedchamber_nats` / `feedchamber_uploads` |
| `docker-compose.dokploy.yml` | Dokploy 档：入口交给 Traefik，nginx 用 `expose` 而非 `ports`、接入 `dokploy-network`，且 backend/frontend 用 `image:` 而非 `build:`（镜像由 CI 推 GHCR）。**与默认档只差这三处，改基础档时必须同步这里** |
| `nginx/nginx.conf` | 反向代理：`/api/v1/` → backend，`/` → web SSR（匿名页 60s 共享缓存），另有 6 条**精确匹配**的爬虫路由 |
| `prometheus.yml` + `prometheus/rules/feedchamber.yml` | 抓取配置与告警规则 |
| `grafana/` | 数据源与仪表盘 provisioning |
| `blackbox/blackbox.yml` | 探针（外部可达性） |
| `.env.example` | 环境变量样例（生产必配项用 ⚠ 标出） |

## 三条硬约束

1. **`NATS_URL` 必填**。后端已剥离进程内 nats-server，指向外部 NATS（需 `-js`）；为空时后端**启动即失败**（fail fast），不再静默降级成只有本进程可见的存储。
2. **`METRICS_PORT`(9091) 只能 `expose`，不能 `ports`**。`/metrics` 无鉴权，安全全靠「不发布到宿主机 + nginx 无上游指向它」。`scripts/check-metrics-exposure.sh` 是这条的 CI 护栏（`docker-build.yml` 里在镜像构建之前跑）。
3. **6 条爬虫路由必须是 nginx 的精确 `location =`**：

```
/sitemap.xml  /sitemap-posts.xml  /sitemap-spaces.xml  /sitemap-tags.xml
/robots.txt   /latest.rss
```

   少了它们会落到 `location /` 交给前端并 404。**爬虫面由后端独占，前端不生成任何爬虫文件。**

## 匿名 SSR 页面缓存（`proxy_cache`）

前端 SSR 实测只有约 10 req/s，是整站第一道天花板；nginx 因此对**匿名公开页面**做
60s 共享缓存（`keys_zone=feedchamber_html`，`max_size=1g`，落在容器内
`/var/cache/nginx/feedchamber`）。

不变量（改配置前必须保住）：

1. **带会话的请求不读不写缓存**。`master_session=` / `master_auth=` 命中
   `$feedchamber_has_session` → `BYPASS`。SSR 的 HTML 里嵌着登录态外壳（头像、未读角标），
   缓存它就是**跨用户泄露**。
2. **RSC / 预取请求不读不写缓存**。`RSC`、`Next-Router-{Prefetch,State-Tree,Segment-Prefetch}`、
   `X-Vinext-*` 任一出现即 `BYPASS`——同一 URL 上是 flight payload 而不是文档，
   混进同一个 key 会互相投毒。
3. **只缓存公开内容路径**：`/`（307 → `/timeline`）、`/timeline` `/spaces` `/t/` `/user/`
   `/search` `/leaderboard` `/motions`。`/login`、`/admin`、`/api/`、爬虫 6 条一律不走缓存。
   3xx 也要缓存（`proxy_cache_valid 301 302 307`）：`/` 不缓存时每个请求都会排在 SSR
   队列末尾，千级并发下实测 44% 超时。
4. **`Vary` 被显式忽略**（nginx 无法表达这些变体），补偿手段就是第 2 条；
   `Accept-Encoding` 折进缓存 key，避免把 gzip 响应发给不支持的客户端。
5. **不忽略 `Set-Cookie`**：上游一旦回 `Set-Cookie`，nginx 默认拒绝存储，这条必须保留。

验证：`curl -sD- -o /dev/null https://<host>/timeline | grep -i x-cache` 连续两次应为
`MISS` → `HIT`；带 cookie 应为 `BYPASS`。

## 常用命令

```bash
docker compose -f deploy/docker-compose.yml up -d --build     # 一键起
cp deploy/.env.example deploy/.env                            # 要覆盖默认值时
docker compose -f deploy/docker-compose.yml logs backend | grep -i admin   # 初始管理员密码
bash scripts/check-metrics-exposure.sh                        # 本地跑护栏
```

初始管理员密码：设了 `ADMIN_PASSWORD` 就用它，否则随机生成并打印到日志。生产至少要配 `POSTGRES_PASSWORD`、`SITE_URL`、`MASTER_COOKIE_SECURE`。

## 改这里要注意

- **Dokploy 档绝不能有 `build:`**：镜像由 `.github/workflows/docker-build.yml` 构建推 GHCR（`ghcr.io/<owner>/feedchamber-{server,web}`），线上 `docker compose up` 只做 pull。这台机器 2 vCPU / 3.9G 且 swap 常满，服务器上跑 `go build` + `vinext build` 会打穿内存（实测 load 131、全站不可用）。回滚靠改 `IMAGE_TAG=sha-<短哈希>` 而不是重构建。
- **改 `docker-compose.yml` 的服务定义时同步 `docker-compose.dokploy.yml`**。两份是整份复制而非 `include`（Compose 的 include 会报 `services.nginx conflicts`，`!override` 会丢 `image`/`volumes`/`depends_on`，两者实测都不可行）。
- **不要给 backend 加 `ports: 9091`**，也不要在 nginx 加指向 9091 的 `location`。
- **nginx 改路径规则时同步检查 6 条爬虫 location 与 `/swagger/`、`/health`**。
- 镜像构建上下文是**仓库根**（两个 Dockerfile 的 `COPY` 按仓库根书写），改 Dockerfile 时注意路径。
- 告警阈值是经验值，用 `scripts/calibrate-alert-thresholds.sh` 在真实流量环境校准，不要凭感觉调。

## 本文件何时更新

- 新增/删除一个编排服务或 named volume
- nginx 路由规则变化（尤其爬虫 6 条与 `/api/v1/`、`/swagger/`）
- 页面缓存策略变化（TTL、白名单、bypass 条件、`proxy_ignore_headers`）
- 两份 compose 的差异项变化
- 必需环境变量变化
