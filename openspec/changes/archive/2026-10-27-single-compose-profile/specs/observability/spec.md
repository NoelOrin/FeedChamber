# observability Specification (delta)

## REMOVED Requirements

### Requirement: 监控栈可部署

移除原因：该 requirement 通篇以 `deploy/docker-compose.cluster.yml` 为锚点（「其引用的每个挂载文件 MUST 在仓库中存在」「`docker compose -f deploy/docker-compose.cluster.yml config` MUST 无错误」）。cluster 档已删除，锚点不存在。

监控素材文件本身保留（`deploy/prometheus.yml`、`deploy/prometheus/rules/feedchamber.yml`、`deploy/grafana/provisioning/`），只是不再由规范强制其存在与挂载方式。相关约束由以下 requirement 继续承载：

- 「告警规则最低集」：仍要求 `deploy/prometheus/rules/feedchamber.yml` 至少 8 条告警（文件保留，未改动）。
- 指标暴露与对外不可达：仍由 `/metrics` 独立 listener 与 nginx 404 的既有 requirement 约束。
