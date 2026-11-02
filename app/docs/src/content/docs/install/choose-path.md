---
title: 选择部署方式
description: Docker Compose、本地开发与自建反代三条路径的取舍
---

Feed Chamber 是一个后端 + 前端 + 两个有状态依赖（数据库、NATS）的组合。
部署方式只影响"这些进程怎么跑"，不影响功能——审核、门控、XP、MCP 在哪种部署下都可用。

## 三条路径

| 路径 | 适合 | 你负责 | 入口文档 |
|------|------|--------|----------|
| **Docker Compose** | 单机、NAS、虚拟机、想最快跑通 | 一台装了 Docker 的机器 | [Docker Compose 安装](/install/docker-compose/) |
| **本地开发** | 改代码、调试接口、写前端 | Go / Node / pnpm 工具链 + 一个 NATS | [本地开发](/install/local/) |
| **自建编排** | 已有 Kubernetes、Traefik、Cloudflare 或公司反代 | 数据库、NATS、TLS、进程守护 | [生产部署清单](/install/production/) |

三条路径共用同一份后端二进制与同一个前端构建产物，不存在"精简版"或"企业版"。

## 决策建议

**先用 Compose。** 它把 Postgres、NATS（JetStream）、后端、前端、nginx 一次配好，
包含健康检查、named volume 与内部指标端口。跑通之后再决定要不要拆。

**需要改代码才用本地开发。** 本地模式不经过 nginx，前后端各自起端口，CORS 与 cookie
行为与生产不同；不要用本地模式验证"上线后登录失效"这类问题。

**已有集群再自建。** 后端是无状态进程（会话在数据库 + NATS KV），可以水平扩多副本；
但 NATS 与数据库必须是你自己运维的外部服务，后端不会替你启动它们。

## 不管走哪条路，这三件事都一样

1. **NATS 必须启用 JetStream。** 后端启动时校验 `NATS_URL`，为空直接退出；
   没有 JetStream，KV bucket 建不出来，会话缓存与限流会全部失效。
2. **首次启动会 seed 管理员。** 用户名固定 `admin`，密码取 `ADMIN_PASSWORD`，
   没设就随机生成并打印到日志；首次登录强制改密。
3. **上传默认落本地磁盘。** 多副本部署要么挂共享卷，要么切到 S3
   （后台 `/admin/settings?tab=storage`，改完重启）。

## 资源估算

小型社区（几十个日活、以文本为主）从 **2 vCPU / 2 GB 内存 / 20 GB 磁盘**起步通常够用；
图片多、备份频繁时优先加磁盘。真正该看的是指标而不是拍脑袋：
`feedchamber_http_request_duration_seconds` 的 p95、`feedchamber_db_pool_wait_count_total`、
`feedchamber_eventbus_queue_depth`——见 [观测与指标](/operations/observability/)。

## 下一步

- 单机一把梭 → [Docker Compose 安装](/install/docker-compose/)
- 想改代码 → [本地开发](/install/local/)
- 准备对外 → [生产部署清单](/install/production/)
