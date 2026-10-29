# Tasks

## 1. 删除冗余编排档

- [x] 1.1 删除 `deploy/docker-compose.dokploy.yml`（全仓零引用）
- [x] 1.2 删除 `deploy/docker-compose.cluster.yml`
- [x] 1.3 确认 `deploy/` 只剩 `docker-compose.yml` 一个编排档

## 2. 规范删除

- [x] 2.1 `openspec/specs/operations/spec.md`：删除「集群监控栈运行与校验」整条 requirement（含两个 scenario）
- [x] 2.2 `openspec/specs/observability/spec.md`：删除「监控栈可部署」整条 requirement（含 scenario）
- [x] 2.3 确认「告警规则最低集」requirement 无需改动（`prometheus/rules/orinforum.yml` 仍在）

## 3. 文档与注释同步

- [x] 3.1 `deploy/README.md`：删「集群 / 生产档」章节，改「集群 / 外部监控」；文件树去掉 cluster 档并把监控目录标为可选
- [x] 3.2 `deploy/README.md` 已知约束段：去掉「集群档由 Prometheus 直连」措辞
- [x] 3.3 `docs/system-map.md`：编排表删 cluster 行，补「唯一编排档」说明
- [x] 3.4 `docs/backend-overview.md`：`expose` 说明改指 `docker-compose.yml`；编排表删陈旧档位（lite/basic/full/cluster）
- [x] 3.5 `docs/design/nats-deploy.md`：档位表、运行命令、「集群部署」小节改为自备外部服务
- [x] 3.6 `deploy/docker-compose.yml` 头注释：去掉「需要集群档时改用 …」
- [x] 3.7 `deploy/prometheus.yml`、`deploy/prometheus/rules/orinforum.yml` 头注释：改为「挂到外部实例」

## 4. 不动的部分（有意保留）

- [x] 4.1 监控素材文件全部保留：`prometheus.yml`、`prometheus/rules/`、`blackbox/`、`grafana/`
- [x] 4.2 `.env.example` 保留但裁剪：去掉 cluster 专用的 `POSTGRES_PASSWORD` / `GRAFANA_*`，只留标准档实际读取的运行时与构建期变量
- [x] 4.3 历史记录不改写：`openspec/changes/archive/2026-10-13-build-observability/**`、`docs/reviews/observability-findings-2026-10-13.md`

## 5. 验收

- [x] 5.1 `grep -r "docker-compose.cluster\|docker-compose.dokploy"` 仅剩历史归档与带日期的评审报告
- [x] 5.2 未实际执行 `docker compose up`（本轮不做运行时验证，仅静态一致性）
