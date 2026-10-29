# Loadtest 压测（`packages/loadtest/`）

对已部署实例做**匿名只读**压测的 k6 脚本集。诊断"这台机器 + 这条链路 + 这个源站"能扛多少并发，产出可对比的报告。

> 上级：[packages/AGENTS.md](../AGENTS.md) · 用法与参数：[README.md](README.md)

## 职责边界

- **只读**：只发匿名读请求，不登录、不写库；需要认证或写场景时另开场景文件，不要往现有目录里塞。
- **不是前端共享包**：不被 `app/web` 消费，没有 `build`、没有 `dist`，不在 turbo 构建链里；`pnpm build` 不会碰它。
- **端点目录单一来源**：`k6/lib/endpoints.js` 的 `ENDPOINTS` 数组描述压测面与权重，`smoke` / `baseline` / `stress` 三个场景共用同一份，报告也由它驱动。

## 文件

| 文件 | 作用 |
|------|------|
| `k6/lib/config.js` | 目标地址、超时、端点类型过滤 `KIND_FILTER`（`-e KINDS=`）、阶梯曲线 `rampStages()`、中止阈值 `ABORT_THRESHOLDS`（全部 `-e` 可覆盖） |
| `k6/lib/endpoints.js` | 端点目录 + 形状断言 `EXPECT` + 分端点指标 `createMetrics()` + `hit()` + `seedPostIds()` |
| `k6/lib/workload.js` | 一次"阅读"迭代：按权重选端点 + 思考时间，baseline / stress 共用 |
| `k6/lib/summary.js` | `handleSummary` → `reports/<timestamp>-<profile>.md` 与 `.json` |
| `k6/smoke.js` | 1 VU 全端点自检，`checks rate == 1`，压测前的门 |
| `k6/reads.js` | 固定并发基线（`VUS` / `DURATION`） |
| `k6/stress.js` | 千级阶梯（`MAX_VUS=1000` 默认）+ `abortOnFail` |

## 跑法

```bash
pnpm --filter @orinforum/loadtest smoke     # 先过自检
pnpm --filter @orinforum/loadtest baseline  # 50 VU × 60s
pnpm --filter @orinforum/loadtest stress    # 0 → 1000 VU 阶梯
```

`reports/` 是生成产物，已 gitignore。报告里的 `429`（限流）与 `403`（Cloudflare/WAF）要和源站 `5xx` 分开读。

## 改这里要注意

- **加端点**：只改 `ENDPOINTS`，并给 `id`（指标前缀，全局唯一）、`weight`、`kind`（`json` / `html` / `redirect`）、`method`、`path`、`body`。改完先跑 `smoke`。
- **改断言**：动 `EXPECT` 会改变"什么算失败"，报告与阈值口径一起变——先确认后端信封语义（`pkg.Success` → `code=0`）。
- **改阶梯/阈值**：在 `config.js`，不要在场景文件里复制一份曲线。
- **混合场景一次只给一个结论**：API 面与 SSR 面容量差一个量级，定位瓶颈时用 `-e KINDS=json|html` 拆开各压一轮，别用混合报告下结论。
- **不要提高默认 `abortOnFail`**：那是防止源站已挂还继续加压的唯一保险。

## 本文件何时更新

- 新增/删除场景文件或端点目录结构变化
- 阶梯曲线、阈值、报告字段变化
- 换压测工具（k6 → 其它）或运行方式变化
