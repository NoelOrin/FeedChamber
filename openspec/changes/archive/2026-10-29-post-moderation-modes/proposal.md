# 文章审核模式：不审核 / 发布后异步 / 外部自动化 / 全部人工

## Why

当前「发帖要不要审核」只有一个布尔开关 `post_approval`，但它表达不了运营真正需要的三种节奏：

- 全站免审（小站点、内部论坛）；
- **先发布、后台异步扫一遍**（大多数社区的默认解：作者立刻看到帖子，命中规则再转人工）——这是现状最缺的一档，因为现在的规则引擎是在发帖请求里**同步**跑的，要么当场拦（作者体验差），要么只能全人工；
- 接入外部审核服务（自建 AI 审核、云内容安全 API），在**发布前**拿一个结论；
- 全部人工（强监管场景）。

`moderation_configs` 里的 `moderation_ai_provider` / `moderation_ai_endpoint` / `moderation_ai_api_key` 三个 key 一直是死配置（无任何读取点），外部审核这一档无处落地。

## What Changes

- 新增系统设置 `moderation_mode`（四选一，默认 `async`）：`off` / `async` / `external` / `manual`，与 `moderation_enabled`（规则总开关）、`post_approval`（旧布尔，降级为 `manual` 的兼容别名）分层。
- `postService.Create` 按模式分支，不再无条件同步跑规则：
  - `off`：`approved`，不跑规则、不写审核记录，TL0 强制待审门一并跳过；
  - `async`（默认）：`approved` 立即公开 → 发帖事件携带 `moderation_async=true` → 事件总线消费者在后台跑规则，命中即转 `pending` 并建单（**交人工判断**，不自动删帖/拒稿）；
  - `external`：发布前同步 `POST` 外部服务（3s 超时，`Authorization: Bearer` 由 `moderation_ai_api_key` 提供），`allow` 放行、`flag`/`block`/`replace` 转待审并留痕；超时/非 2xx/未知结论一律回退 `async`；
  - `manual`：一律 `pending` + 必留痕。
- 审核记录新增判定来源（`strategy`）：`async` / `external` / `manual` / `tl0`，审核队列可据此区分「谁判的」。
- 后台两个入口共用同一个设置 key：`设置 → 内容 → 文章审核模式`（下拉）与 `/admin/moderation` 顶部的模式卡片（四选一，直接切换）。
- 写入校验：非法模式值返回 `2xxx`；读取侧对缺失/非法值回落默认 `async`。

## Non-goals

- **不动评论与编辑**：评论仍走 `CheckContent` 的关键词/正则规则，模式开关只作用于文章；编辑不重审。
- **不引入异步任务持久化**：异步审核与 webhook 一样走 NATS core pub/sub，进程重启会丢在途事件；内容本身不受影响（只是少了一次自动复查）。
- **不做外部服务的重试/排队**：外部审核是同步调用 + 超时回退，不做退避重试。
- **不新增审核端点**：模式经现有 `POST /admin/settings/{key}/update` 与 `POST /admin/settings/batch` 读写。
