# operations Specification (delta)

## REMOVED Requirements

### Requirement: 集群监控栈运行与校验

移除原因：`deploy/docker-compose.cluster.yml` 已删除，仓库不再发布集群/监控编排档，该 requirement 失去对象。

原先它要求「cluster 部署 MUST 可一条命令拉起可用的监控栈（Prometheus 9090 / Grafana 可访问）」，以及「`docker compose -f docker-compose.cluster.yml config` MUST 成功」的离线校验路径。两者都绑定在一个不再存在的文件上。

删除后不再以规范形式约束部署档数量与形态：`deploy/` 只保留 `docker-compose.yml` 是当前实现选择，而非 MUST。backend 的 `METRICS_PORT`（默认 9091）暴露方式由「可观测性」capability 的指标 requirement 继续约束，不受影响。
