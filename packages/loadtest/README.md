# 压测（k6 · 匿名只读）

对已部署的 OrinForum 实例做**只读**负载压测：端点目录、可用性自检、固定并发基线、千级阶梯加压，产出 Markdown + JSON 报告。

- 目标默认 `https://dev-forum.noelorin.top`，可用 `-e BASE_URL=` 指向任意实例（含本地）。
- 全部场景**不写入任何数据**：只发匿名读请求（列表 / 详情 / 首页 SSR / sitemap / RSS）。
- 不进构建链：本包没有 `build`，turbo 的 `pnpm build` 不会碰它。

## 前置：k6

```bash
brew install k6          # macOS
k6 version               # 本仓库验证过 v2.x
```

没有 Homebrew 时用容器跑（脚本路径与 CWD 要一致，报告才会落到本包 `reports/`）：

```bash
docker run --rm -i -v "$PWD:/lt" -w /lt grafana/k6 run k6/stress.js
```

## 三条命令

```bash
pnpm --filter @orinforum/loadtest smoke      # ① 自检：1 VU 打全端点，必须 100% 通过
pnpm --filter @orinforum/loadtest baseline   # ② 基线：50 VU × 60s 稳态指标
pnpm --filter @orinforum/loadtest stress     # ③ 阶梯：10%→30%→50%→75%→100% × 1000 VU
```

报告写在 `reports/<时间戳>-<档位>.md`（同目录还有同名 `.json`，含原始指标）。
`pnpm --filter @orinforum/loadtest latest` 打印最近一份报告路径。

> **先跑 smoke。** 端点 404 / 信封 code≠0 属于"脚本或环境错了"，不是性能问题；smoke 不过就别跑压力场景。

## 可调参数（k6 `-e`，直接在命令后追加）

| 变量 | 默认 | 说明 |
|------|------|------|
| `BASE_URL` | `https://dev-forum.noelorin.top` | 被测实例 |
| `MAX_VUS` | `1000` | 阶梯顶点并发（= 用户说的"千级"） |
| `LEVEL_SECONDS` | `60s` | 每一级停留时长 |
| `HOLD_SECONDS` | `120s` | 顶点保持时长 |
| `VUS` / `DURATION` | `50` / `60s` | 仅 `baseline` 用的固定并发与时长 |
| `HTTP_TIMEOUT` | `15s` | 单请求超时 |
| `USER_AGENT` | `OrinForum-LoadTest/0.1 (k6)` | 服务端日志里可辨识压测流量 |
| `KINDS` | （空=全量） | 只压某类端点：`json`（API）/ `html`（SSR+静态）/ `redirect`，逗号分隔 |

例：先小规模验脚本（50 VU × 60s），再放开千级：

```bash
k6 run -e MAX_VUS=50 -e LEVEL_SECONDS=20s -e HOLD_SECONDS=20s k6/stress.js
k6 run -e MAX_VUS=1000 k6/stress.js
```

## 分层压测（找各自的拐点）

混合场景里一个慢端点会掩盖另一个的能力上限，所以支持按端点类型拆开压：

```bash
# 只看 API 面能扛多少并发（SSR 完全不参与）
k6 run -e KINDS=json -e PROFILE=api k6/stress.js

# 只看 SSR 页面面（前端服务端渲染的容量）
k6 run -e KINDS=html -e MAX_VUS=500 -e PROFILE=ssr k6/stress.js
```

`KINDS` 只裁剪"压哪些端点"，权重在剩余集合里重新归一化，断言与阈值口径不变；报告头部会写明本次压的是哪一面。

## 负载组成

| 端点 | 权重 | 类型 |
|------|------|------|
| `POST /api/v1/posts/list` | 35 | JSON |
| `GET /api/v1/posts/{id}` | 20 | JSON（ID 取自 setup 的真实列表） |
| `GET /timeline` | 10 | SSR 页面（实测最重） |
| `GET /spaces` | 5 | SSR 页面 |
| `GET /` | 5 | 307（不跟随重定向） |
| `POST /api/v1/tags/list` | 4 | JSON |
| `POST /api/v1/spaces/tree` / `flat` | 3 / 3 | JSON |
| `POST /api/v1/settings/public` | 3 | JSON |
| `GET /latest.rss` | 3 | 文件 |
| `POST /api/v1/themes/list` / `badges/list` / `leaderboard/list` | 2 / 2 / 2 | JSON |
| `POST /api/v1/announcements/published` | 2 | JSON |
| `GET /sitemap-posts.xml` | 1 | 文件 |

每次迭代另加 0.2~0.7s 思考时间，因此「1000 VU」对应的实际吞吐取决于源站延迟，不是固定 RPS。

## 判定与中止

- **断言**：JSON 端点要求 `200` **且** 信封 `code=0`（只判 200 会把 4xx 信封误判成功）；页面要求 `2xx` 且正文非空；根路径要求 `307/301/302/200`。
- **中止保护**：`http_req_failed > 2%` 或 `checks < 97%` 持续越界 → `abortOnFail` 立即停止（`delayAbortEval: 30s` 保证先拿到有效样本），避免源站已挂还继续轰。
- **状态码单独计数**：`429` 是策略性限流、`403` 可能来自 Cloudflare/WAF 挑战、`5xx` 才是源站错误，报告里分列，别混为一谈。

## 读报告

1. 看 `总览` 的 p95 / 失败率随并发的走势——某级之后陡升的那一级就是当前架构的拐点；
2. 看 `分端点` 谁在拖后腿（SSR 页面通常最贵，`posts/list` 通常是吞吐主体）；
3. 看 `状态码分类` 有没有 `403/429` 干扰结论（有就说明先撞的是 Cloudflare 而不是源站）。

## 基线

[`BASELINE.md`](BASELINE.md) 记录了 2026-10-29 的三轮实测（混合 / 仅 API / 仅 SSR）与结论：SSR 面约 10 req/s 先崩，API 面千级并发 0.02% 失败、平台期约 295 req/s。改动部署或缓存后跑同一组命令对照即可。

## 扩展

加/减压测面只改 [`k6/lib/endpoints.js`](k6/lib/endpoints.js) 的 `ENDPOINTS` 数组（`id` / `weight` / `kind` / `method` / `path` / `body`），指标与报告会自动带上新端点。新增端点后先跑 `smoke` 确认形状断言成立。

## 注意

- 目标站在 Cloudflare 后面：动态内容不缓存，流量会真实落到源站；单出口 IP 的千级并发也可能先触发 Cloudflare 侧的速率限制——报告里的 `403/429` 会体现这一点。
- 这是压测工具，不要对你不拥有的站点使用。
