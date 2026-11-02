---
title: 生产部署清单
description: 对外提供服务前必须确认的 TLS、密钥、数据、观测与内容治理项
---

这份清单按"上线前一次做完"的顺序排：先让流量安全地到达，再保证数据可恢复，
最后才是观测与运营。每一项都能在仓库里找到对应配置，不是泛泛而谈的注意事项。

## 一页版

| # | 项目 | 完成标准 |
|---|------|----------|
| 1 | TLS | 全站 HTTPS，HTTP 301 到 HTTPS |
| 2 | 密钥 | `ADMIN_PASSWORD`、`POSTGRES_PASSWORD` 显式设置且足够强 |
| 3 | Cookie | `MASTER_COOKIE_SECURE=true`（默认），只经 HTTPS 下发 |
| 4 | 依赖隔离 | Postgres / NATS 不暴露到公网，仅内网或 compose 网络可达 |
| 5 | 备份 | 数据库每日 dump + 上传目录同步，且**演练过一次恢复** |
| 6 | 观测 | Prometheus 抓 9091，日志进采集，告警接上 |
| 7 | 站点地址 | 后台填 `site_url`，sitemap / canonical / OG 用它 |
| 8 | 账号安全 | 管理员改密完成，建议开 MFA，删除/禁用测试账号 |
| 9 | 内容治理 | `moderation_mode`、`register_mode`、门控规则按运营策略设置 |
| 10 | 升级路径 | 知道怎么备份、怎么回滚、怎么重算 XP |

## TLS 与反向代理

后端与前端都只监听容器内端口，**唯一入口是 nginx**。仓库自带 `deploy/nginx/nginx.conf`
已经处理了这些容易漏的点，自建反代时逐条对照：

| 路径 | 必须转到 | 原因 |
|------|----------|------|
| `/api/v1/` | 后端 8080 | 业务接口 |
| `~ ^/api/v1/(notifications\|realtime)/stream$` | 后端，`proxy_buffering off`、`proxy_read_timeout 3600s` | SSE 长连接；默认 60s 会被切断 |
| `/swagger/` | 后端 | 运行时 API 文档 |
| `= /mcp` | 后端，`proxy_read_timeout 120s` | JSON-RPC 协议端点，不在 `/api/v1` 下 |
| `= /sitemap.xml` 等 6 条 | 后端 | 爬虫文件由后端独占生成，前端没有 |
| `/` | 前端 3000 | SSR 页面与静态资源 |
| `/health` | nginx 本地返回 | 负载均衡健康检查，不经过后端 |

另外两件必须显式做的事：

```nginx
# 上传上限：仓库自带配置没有设置，nginx 默认 1 MiB 会先于后端拒绝
client_max_body_size 10m;   # 不小于 upload_max_size（默认 10485760）

# 传递关联 id，让 nginx 访问日志与后端 trace 能对上
proxy_set_header X-Request-ID $request_id;
```

:::caution[SSE 不能走缓冲]
只要反代开了 `proxy_buffering`，通知流就会攒到缓冲区满才下发，实时性消失；
再叠加默认 60s 读超时，空闲流会被直接切断。两条都要按上表处理。
:::

## 密钥与账号

| 项 | 做法 |
|----|------|
| `ADMIN_PASSWORD` | 首次启动前设置；空值会随机生成并打印日志，事后很难找 |
| `POSTGRES_PASSWORD` / `POSTGRES_USER` | 不要用默认的 `feedchamber` |
| `MASTER_COOKIE_SECURE` | 保持 `true`；只有纯 HTTP 本地调试才设 `false` |
| MFA | `/admin/settings?tab=user` 开启 `mfa_totp_enabled`，给自己绑定 TOTP |
| 会话 | 改密/改角色/封号会自动吊销该用户全部会话；换设备排障用 `/admin/sessions` |
| API Key | 默认关闭（`api_key_enabled=false`）；开启后按 scope 最小授权 |
| 测试账号 | seed 的示例用户只在 dev 环境创建，prod 不会写入 |

## 数据与备份

| 数据 | 位置 | 备份 |
|------|------|------|
| 业务数据 | Postgres（compose 卷 `feedchamber_pg`） | `pg_dump` 每日一次，保留 7–30 天 |
| 上传文件 | 本地卷或 S3 | 卷快照 / 对象存储版本化 |
| NATS JetStream | compose 卷 `feedchamber_nats` | 可重建；丢失会丢会话缓存、限流窗口与未消费事件 |

```bash
# 每日 dump 示例
docker compose -f deploy/docker-compose.yml exec -T postgres \
  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" | gzip > "/backup/fc-$(date +%F).sql.gz"
```

后台也能创建备份（`/admin/backups`），保留天数由 `backup_retention_days` 控制（默认 30）。
**备份只有恢复过才算数**：每季度把 dump 导入一个临时库，确认能启动、能登录、帖子还在。

## 观测

| 项 | 配置 |
|----|------|
| 指标 | `METRICS_ENABLED=true`，`METRICS_PORT=9091`；**不要**把它映射到公网 |
| Prometheus | `deploy/prometheus.yml` 可直接用；规则在 `deploy/prometheus/rules/` |
| 日志 | `LOG_FORMAT=json`、`LOG_OUTPUT=stderr`，交给容器日志驱动采集 |
| 关联 | 日志字段 `trace_id` / `span_id` / `route` / `latency_ms`；入站 `traceparent` 会被沿用 |
| 健康 | 后端 `/api/v1/health`（依赖异常时非 2xx）、nginx `/health` |
| 建议告警 | 5xx 比例、p95 延迟、`feedchamber_db_pool_wait_count_total`、`feedchamber_ratelimit_rejections_total`、`feedchamber_nats_connected` |

指标全表见 [观测与指标](/operations/observability/)。

## 站点地址与 SEO

1. 后台「站点信息 → 网站 URL」填 `https://你的域名`（或设环境变量 `SITE_URL` 兜底）
2. 确认 `GET /sitemap.xml` 输出的是绝对地址，而不是 `localhost:8080`
3. 到 Google Search Console / 百度搜索资源平台提交 sitemap，填对应验证码
4. 不需要索引他人主页时保持 `user_profile_indexable=false`
5. 分享图设置 `og_image`，否则回退 Logo

缺少 `site_url` 时站点功能正常，但 sitemap 会输出不可达地址，表现为"一切正常、收录为零"。

## 内容治理

| 设置 | 建议 |
|------|------|
| `register_mode` | 公开运营用 `all` / `email`；小圈子用 `invite`；需要人工把关用 `approve` |
| `moderation_mode` | 默认 `async`；风险高用 `manual`；接了外部审核服务用 `external` |
| 门控规则 | `/admin/gates`：新用户发外链、TL0 发帖等按需建规则，先试算再启用 |
| 限流 | `rate_limit_enabled` 默认关闭；公开站点建议开启全局每 IP 上限 |
| MCP | `mcp_enabled` 默认关闭；只为可信客户端开，写工具逐个启用 |

## 扩容与回滚

- 后端无状态：会话在数据库 + NATS KV，不需要 sticky session；多副本直接加
- 上传要么共享卷，要么切 S3；否则多副本会读到"一半的图片"
- 升级前：备份 → 拉镜像 → `up -d --build`；`AutoMigrate` 只加不删，回滚镜像即可
- 破坏性变更与手工迁移见 [升级与迁移](/operations/upgrades/)

## 上线后 10 分钟验收

```bash
curl -sI https://你的域名/ | head -1                 # 200
curl -s  https://你的域名/api/v1/health               # code: 0
curl -s  https://你的域名/sitemap.xml | head -c 200   # 绝对地址
curl -sI https://你的域名/api/v1/notifications/stream  # 200，且不立即断开
```

再手动走一遍：注册 → 登录 → 发帖 → 评论 → 通知到达 → 后台审核 → 退出登录。
六步都通，才算上线完成。
