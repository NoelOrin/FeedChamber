## Why

`deploy/` 下并存三份编排档，其中两份没有独立存在价值：

1. **`docker-compose.dokploy.yml` 是纯孤儿**：全仓零引用（README、docs、CI、脚本都没有提到它），内容与标准档几乎一致，唯一差别是 nginx 额外接入 `dokploy-network`、不发布宿主机端口。它记录的是一个特定 PaaS 的接入姿势，放在仓库里既不会被读到，也会随标准档演进慢慢腐坏。
2. **`docker-compose.cluster.yml` 的维护成本高于收益**：171 行，含 PostgreSQL、三节点 NATS 集群、Prometheus、blackbox、Grafana 五个附加服务，以及 6 个绑定挂载源。它是两个 openspec requirement 的强制对象（`operations` 的「集群监控栈运行与校验」、`observability` 的「监控栈可部署」），因此每次改动监控配置都必须同步改它，而实际上从来没有按这份档跑过生产。同时它把「多实例部署」与「监控栈」两件正交的事捆在一个文件里——想要 PG 就必须一并拉起 Grafana。
3. **档位语义已经漂移**：更早的 `lite`/`basic`/`full` 三档已合并删除，`docs/backend-overview.md` 的编排表仍列着它们，说明多档并存的文档维护本身就在失守。

结论：只保留一个「一键可运行、零外部依赖」的标准档，把监控配置降级为**可选素材**（文件保留，接入方式变成挂到外部 Prometheus/Grafana），多实例部署交给使用者自备外部服务。

## What Changes

- **删除 `deploy/docker-compose.dokploy.yml`**（零引用，无需迁移说明）。
- **删除 `deploy/docker-compose.cluster.yml`**：PG + nats×3 + 监控栈不再随仓库发布。
- **保留监控素材**：`deploy/prometheus.yml`、`deploy/prometheus/rules/`、`deploy/blackbox/`、`deploy/grafana/` 全部保留，改为「挂到外部实例使用」的定位；其中 `rules/orinforum.yml` 的 13 条告警规则、Grafana provisioning 与 dashboard 都不动。
- **规范删除**（不是改写）：
  - `operations` 删除「集群监控栈运行与校验」整条 requirement——它要求的「一条命令拉起监控栈」与「`docker-compose.cluster.yml config` MUST 成功」都锚定在已删除的文件上。
  - `observability` 删除「监控栈可部署」整条 requirement——同理，它通篇以 cluster compose 的挂载文件为约束对象。
  - 删除后不再用 MUST 规定部署档的数量与形态：「只保留一个 compose」是本轮实现选择，不上升为规范。`METRICS_PORT` 的暴露方式、「告警规则最低集」等其他 requirement 不受影响，继续约束。
- **文档同步**：`deploy/README.md`、`docs/system-map.md`、`docs/backend-overview.md`、`docs/design/nats-deploy.md`、以及 `deploy/docker-compose.yml` / `prometheus.yml` / `prometheus/rules/orinforum.yml` 三处文件头注释；`deploy/.env.example` 裁掉 cluster 专用的 `POSTGRES_PASSWORD` / `GRAFANA_*`（这些变量已无人读取）。

## Capabilities

### Modified Capabilities

- `operations` — **删除**「集群监控栈运行与校验」requirement（REMOVED）。该 capability 其余 requirement（环境变量配置、健康检查、全量接口回归等）不变。
- `observability` — **删除**「监控栈可部署」requirement（REMOVED）。「告警规则最低集」「指标暴露与对外不可达」保持不变。

## Impact

- **文件删除**：`deploy/docker-compose.dokploy.yml`、`deploy/docker-compose.cluster.yml`。
- **破坏性变更（对使用者）**：原先用 `docker compose -f docker-compose.cluster.yml up -d` 拉起 PG + NATS 集群 + 监控栈的人，需要自行编写编排或改用外部服务。仓库不再提供该路径。
- **不受影响**：`app/server/Dockerfile`、`app/web/Dockerfile`、标准档 `docker-compose.yml` 的行为完全不变；后端 `METRICS_PORT` 暴露方式不变；告警规则与 dashboard 内容不变。
- **历史记录不改写**：`openspec/changes/archive/2026-10-13-build-observability/` 与 `docs/reviews/observability-findings-2026-10-13.md` 保留原文（它们是当时的评审与提案记录，改写会变成伪造历史）。
- **不在本轮范围**：不新建「多实例部署」替代档；不删除监控素材文件；不改 docker-compose.yml 的服务拓扑。
