# ADR: 暂不引入 OpenTelemetry SDK

- 状态：已采纳
- 日期：2026-10-13（决策）／2026-10-28（补记）
- 相关：[openspec/specs/observability](../../openspec/specs/observability/spec.md)、[observability-findings-2026-10-13](../reviews/observability-findings-2026-10-13.md)

## 背景

2026-10-13 建设可观测性时，需要决定追踪（tracing）用什么。当时可选：

1. 引入 OpenTelemetry（OTel）SDK + OTLP exporter，再配一个 collector；
2. 自研最小实现，只做 W3C Trace Context 的解析与传播。

## 决策

**选择 2：自研 W3C `traceparent`，不引入 OpenTelemetry SDK。** 追踪只做到「一个关联 id 贯穿全链路」为止。

具体实现落在 `internal/pkg/tracing`：解析/生成 `traceparent`（仅 `00` 版本、小写十六进制），trace 上下文经 `context.Context` 传递，EventBus 经 NATS 消息头跨进程传播。trace id 同时作为 `X-Request-ID` 与日志的 `trace_id`，全链路只有一个关联 id。

## 理由

- **我们需要的只是关联，不是 span 树。** 实际排障动作是「拿到用户给的 id → grep 日志 → 串起网关/服务/后台 worker」。这只需要 id 传播，不需要 span 的父子关系、采样策略与属性标注。
- **OTel 的真实成本不在 SDK 本身。** 引入它意味着同时引入 collector 部署、采样配置、exporter 运维，以及一整套语义约定（span 命名、属性命名）。对一个自托管的小论坛，这些运维面比它解决的问题更重。
- **零依赖能让单二进制保持单二进制。** 后端刻意保持可 `go run` 启动、可交叉编译成单文件。OTel SDK 会带来一批依赖与初始化顺序约束。
- **指标侧已有 Prometheus，不需要 OTel 的 metrics。** 引入 OTel 会立刻面对「两套指标语义」的取舍，而这个取舍没有收益。

## 保留的兼容缝

这个决策是**可逆**的，而且已经为反转留了接口：

- 传播格式就是 W3C Trace Context 标准格式。任何 OTel 部署都能直接接收我们发出的 `traceparent`，不需要改我们的代码。
- 反向也成立：上游（网关、CDN、其他 OTel 服务）发来的 `traceparent` 我们已经在沿用其 trace id（`middleware.Trace` 的既有行为），并据此新建 span。
- 因此「将来接入 OTel」的动作是把 SDK 接到 `tracing` 包的解析结果上，而不是重写埋点。

## 代价（明确承认）

- 没有 span 树：只能知道「属于同一个请求」，不能知道「A 调 B 花了多久、B 内部哪一段慢」。跨服务耗时目前靠日志里的 `latency_ms` 分段推断。
- 没有采样与属性：无法按业务维度（用户等级、版面）聚合追踪数据。
- 没有标准化的 OTLP 导出：数据只进日志，不进追踪后端。

当出现「必须看跨服务调用树」或「必须按维度聚合追踪」的真实需求时，应重新评估本 ADR，而不是先加埋点再说。
