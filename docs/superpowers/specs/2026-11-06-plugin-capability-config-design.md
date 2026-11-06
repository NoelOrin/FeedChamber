# 插件能力配置化设计（`internal/plugin`）

- 日期：2026-11-06
- 范围：`app/server/internal/plugin` 宿主 API 规格表（`api.go`）
- 状态：待评审
- 前置：用户裁定「先做插件化能力，不执着于 MCP」；HTTP/MCP 投影本轮**不做**

## 1. 问题：同一份参数形状被写了两遍

`api.go` 现有 902 行，`apiSpecs` 一节（229–556 行）占 328 行，服务 10 个宿主 API。实测重复：

| 指标 | 现值 | 说明 |
|------|------|------|
| `var in struct { ... }` 匿名参数结构 | **17 处** | 每个 build 一份，多数 audit 又抄一份 |
| `audit` 闭包 | **7 个** | 其中 6 个是对同一份 JSON 的**第二次** `json.Unmarshal` |
| `requireFields(...)` 调用 | **9 处** | 字段名散落在调用点，与 struct tag 是两份数据 |
| 参数形状的声明处 | **2 处/方法** | build 的 struct + audit 的 struct |

结构性后果（不是风格问题）：

1. **改一个字段名要改两处，且没有任何东西会报错。** build 改了 `post_id` → `postId`，audit 仍按 `post_id` 解析，`json.Unmarshal` 静默返回零值，审计记录里 target 变成空串——**写操作照常生效，审计目标丢失**。
2. **没有机器可读的参数说明。** 插件作者要调 `api.posts.setStatus`，只能读 Go 源码或仓库文档；`apiMethodNames()` 只给名字，不给参数。
3. **反向校验缺失。** `TestEveryAPIMethodDeclaresACapability` 只查「每个 API 都登记了已知能力」，不查「每个已知能力都被某个 API 使用」。没有 API 使用的能力照样出现在装机确认页，管理员批了一条用不上的权限。
4. **`RegisterAPIs` 每次调用多扫一遍表。** 闭包已经捕获了 `spec`，却仍调 `lookupAPI(spec.method, ...)` 线性重扫 `apiSpecs`（api.go:721–731）。

## 2. 非目标（本轮明确不做）

- **不建新的共享包。** 原计划抽 `internal/capability` 是为了让 MCP 与插件共用；用户裁定 MCP 不在本轮，唯一消费者的抽象是投机性的。全部改动留在 `internal/plugin`。
- **不做运行时能力开关。** 见 §7。
- **不做 HTTP / MCP 投影。** 数据结构预留 `HTTPPath` 的位置，但不填、不校验、不生成路由。
- **不碰 `service` 层的能力判定、审计口径、KV 隔离规则。** 保持既有语义。

## 3. 核心设计：参数即数据

把「参数形状」从 17 处匿名 struct 收敛成每条规格的一行数据。

```go
// Field 声明一个参数。同一份声明同时产出三样东西：
//   - 校验器（是否必填、能否为空串）
//   - 取值器（a.String("post_id") 的合法字段名集合）
//   - JSON Schema（给插件作者与安装确认页）
type Field struct {
    Name     string  // 参数名，必须与 Handler 里的取值一致
    Type     FieldType // string | int | object | bool
    Required bool
    AllowEmpty bool  // 仅 api.kv.set 的 value 需要
    Enum     []string // 只写进 schema，运行期不校验（理由见 §5.2）
    Desc     string
}
```

规格行：

```go
type apiSpec struct {
    method string
    cap    string
    write  bool
    params []Field
    audit  AuditSpec        // 零值 = 无审计（读类）
    build  func(pluginID string, deps APIDeps) (Func, error)
}

// Func 是解码并校验之后的处理函数。**不再是 ipc.Handler**：
// 参数已经解好，handler 不再接触 json.RawMessage。
type Func func(ctx context.Context, a *Args) (any, error)

// AuditSpec 把审计落点声明成字段引用，而不是第二个 Unmarshal 闭包。
type AuditSpec struct {
    Action string
    Target []string      // 字段名，按序拼接
    Detail []string      // 字段名，按序拼接
    Scope  AuditScope    // 0 = 原样；AuditScopePluginKV → target 过 pluginKVKey
}
```

`api.kv.set` 是唯一需要派生值的一条：审计 target 要落**加过前缀的全键** `plugin:<id>:<key>`。用 `AuditSpec.Scope = AuditScopePluginKV` 表达，由统一的提取器调用既有的 `pluginKVKey`，而不是再开一个闭包。

### 3.1 解码与校验只发生一次

`lookupAPI` 变成「查表 → 构造适配器」，适配器把 `ipc.Handler` 的 `json.RawMessage` 变成 `*Args`：

```
RegisterAPIs（每方法一次）
  └─ lookupAPI(method, pluginID, caps, deps) → ipc.Handler, bool
       ├─ 查表得到 spec（**不再重扫**：由调用方传入或预建索引）
       ├─ !caps.grants(spec.cap) → denyHandler（不持有 deps，行为不变）
       ├─ build 失败            → 报错 handler（errAPINotWired，行为不变）
       └─ 成功 → 返回 func(ctx, raw):
            1. 解码一次 → *Args
            2. 按 spec.params 校验（必填 / 空串 / 类型）
            3. 调 spec.build 产出的 Func
            4. 成功且 write → 按 spec.audit 提取 action/target/detail 落审计
```

**保持不变的既有语义（每条都有现存测试）：**

- `ipc.Handler` 类型不动（`ipc.go:44`），`Conn.Handle` 不动——适配在 `api.go` 内部完成。
- 容忍未知字段（不 `DisallowUnknownFields`）。
- 缺字段时**点名**字段、错误按名排序（`requireFields` 现有行为，测试可逐字断言）。
- 未授予能力时是 `denyHandler`，**不持有 deps**，删掉内部判定也碰不到业务代码。
- 审计失败**不翻转结果**，只记 error 日志。
- 参数非法时 service / KV / 审计**零调用**。

## 4. 收益与可验证的验收点

| # | 验收点 | 怎么验 |
|---|--------|--------|
| 1 | `api.go` 中 `var in struct {` 出现次数 = **0** | `grep -c` 归零 |
| 2 | `audit:` 闭包数量 = **0**（改成 `AuditSpec` 数据） | `grep -c "audit: func"` 归零 |
| 3 | `json.Unmarshal(p, &in)` 在 `api.go` = **0** | `grep -c` 归零 |
| 4 | 每个已知能力都被 ≥1 个 API 使用 | **不能直接加断言**：今天就有 2 条违反，见 §5.3。先裁定再定测试 |
| 5 | `plugin.json` 的 `capabilities` 清单与 API 需求集一致 | 同上测试断言 |
| 6 | 能导出每个 API 的 JSON Schema | 新增 `APISchemas()`，后台安装确认页可读 |
| 7 | **`internal/plugin` 生产依赖图 gorm 仍为 0** | `go list -deps ./internal/plugin \| grep -c gorm.io` = 0 |
| 8 | 现有 60+ 条 `api_test.go` 全部**原样通过** | 不改断言，只允许改 mock 接线 |

第 8 条是本轮的主回归网：**不新增业务行为，只换实现形态**。任何需要改断言的改动都说明重构越界了。

## 5. 两处必须如实处理的点

### 5.1 取值域校验不做（因为已经有了）

初版设计打算在插件层加 `Enum` 强校验。读代码后推翻：

```go
// internal/service/content.go:480 与 post.go:911
if !isValidPostStatus(status) {
    return pkg.NewAppError(pkg.INVALID_PARAMS, "status 必须为 approved、pending、rejected 或 closed")
}
```

`SetStatusForPlugin` 与 `SetStatus` **已经**校验取值域。在插件层再校验一遍会产生第二个事实来源——将来加一个 `post_status` 枚举值，改 service 忘了改插件层，症状是「API 文档说支持、实际被拒」。

因此 `Enum` **只写进 schema 供文档使用，运行期不校验**。这条要在代码注释里写明，否则后来者会「顺手补上校验」并制造漂移。

### 5.2 枚举字面量放哪：internal 测试镜像 `model` 常量

`Enum` 的值（`approved/pending/rejected/closed`）不能从 `internal/model` 取——生产代码 import 它会把 gorm 带进 `internal/plugin`，违反硬约束 3。

**实测确认（本轮已验证）**：`package plugin` 的内部测试（`_test.go`，`package plugin` 而非 `plugin_test`）import `internal/model` 后，

```
go list -deps ./internal/plugin | grep -c gorm.io   →  0
go build ./internal/plugin/                          →  OK
```

——`go list -deps` 统计的是**非测试**构建图。因此：

- 生产代码里 `Enum` 写字符串字面量；
- 内部测试 import `model`，断言「字面量 == `model.PostStatusApproved` 等常量」。

这不破坏 gorm=0，却把「插件侧枚举」与「服务端权威枚举」钉在一起。这是本轮唯一保留的字符串重复，且它有了回归网。

### 5.3 两条空能力：`posts:read` 与 `comments:read` 从未被任何 API 使用

实测（本轮已验证）：

```
CapPostsRead     → 0 apiSpec(s)   # "posts:read"
CapCommentsRead  → 0 apiSpec(s)   # "comments:read"
CapPostsWrite → 2   CapCommentsWrite → 1   CapUsersRead → 2
CapNotifySend → 1   CapKVRead → 1   CapKVWrite → 1   CapSlotsWrite → 2
```

7 条有人使用，2 条是**空的**。后果是真实的：管理员在安装确认页批准 `posts:read` 时，批准的是一条**不存在的能力**——插件拿不到任何对应 API，却占用了信任决策的注意力。

这正好是 §1.3 那个「反向校验缺失」的活证据：现有 `TestEveryAPIMethodDeclaresACapability` 只查「API → 能力」，漏了「能力 → API」，所以两条空能力一直没被发现。

**这不能在本轮的重构里顺手修掉，因为它不是重构。** 两条路都要改行为：

| 方案 | 行为变化 | 风险 |
|------|----------|------|
| **A. 删除两条常量** | 已声明 `posts:read` 的插件在**下次安装/预览**时被 `Validate` 拒绝（未知能力报错） | 存量已装插件不受影响（`Capabilities` 是安装快照，运行期不重校验）；但**重新安装或升级会失败**，对作者是破坏性变更 |
| **B. 给它们接上真实 API** | 新增 `api.posts.get` / `api.comments.get` 宿主 API | 这是**新能力**，超出本轮「减重复」范围；且要新增接缝接口、审计口径、测试 |

我的建议：**本轮不修，只把它变成可见的**——加一条测试断言「已知能力必须被 ≥1 个 API 使用」，但以**显式白名单**放行这两条，白名单里写明「待裁定」并引用本节。这样：

- 回归网立即生效：将来新增空能力会变红；
- 存量两条不被静默遗忘，`grep` 白名单就能找到；
- 修复动作（A 或 B）留给一个明确决定，而不是夹在一个重构提交里。

需要你选 A / B / 本轮不修（建议）。

## 6. 改动清单

| 文件 | 改动 |
|------|------|
| `internal/plugin/api.go` | 新增 `Field`/`FieldType`/`Args`/`AuditSpec`/`AuditScope`/`Func`；`apiSpec` 换形；`build` 签名收窄到 `Func`；`lookupAPI` 返回带解码校验的适配器；删 `decodeParams` 之外的双重 Unmarshal 与全部 `audit` 闭包；`RegisterAPIs` 去掉重扫 |
| `internal/plugin/schema.go`（新增） | `APISchemas()`：由 `params []Field` 生成 JSON Schema（含 `Enum`/`Desc`/`required`），供安装确认页与插件作者 |
| `internal/plugin/api_test.go` | **不改既有断言**；新增 `TestEveryCapabilityUsedByAnAPI`（带 §5.3 白名单）、`TestAPISchemasCoverAllMethods`、`TestFieldLiteralsMatchModelConstants`（internal 测试，import `model`） |
| `internal/plugin/AGENTS.md` | 更新宿主 API 一节：规格表形态、`Args` 取值、`AuditSpec`、§5.1/§5.2 两条裁定、新增测试 |
| `internal/container/plugin_api_deps.go` | 无需改动（7 个接缝接口与 `APIDeps` 保持不变） |

`APIDeps` 的 7 个接缝接口（`PostWriter` 等）**一律不动**：它们是 Go 代码，不该假装成数据。

## 7. 为什么不做运行时能力开关

MCP 侧有 `mcp_disabled_tools`（负号 = 显式开启写工具，误清空回到全关）。照搬给插件看起来对称，但语义不同：

- MCP 的工具是**站点自己的能力**，开关是站长的偏好；
- 插件的能力是**安装时授予的快照**（`model.Plugin.Capabilities` 注释明写「不是当前 manifest 声明」），改权必须**重走安装审批**。

给插件能力加运行时开关，等于给一条绕过安装审批的窄门——而且它必须自带一套 fail-closed 编码与后台界面才能不重蹈覆辙，成本远超本轮「减重复」的目标。**明确不做**；真要做得单独立项。

## 8. 实施顺序

1. 引入 `Field`/`Args`/`AuditSpec`，保持 `apiSpecs` 原样可编译（两种形态并存一提交）。
2. 迁移 10 条规格逐条到新形态；每迁一条跑一次 `api_test.go`。
3. 全部迁完后删除旧 `decodeParams`/匿名 struct/audit 闭包，`grep` 归零。
4. 加 `schema.go` 与 3 条新测试（§5.3 白名单先按「本轮不修」写）。
5. 更新 `AGENTS.md`，跑 `go test ./internal/plugin/ -race` 与 gorm=0 检查。

第 1、2 步可以拆成两个提交，第 3 步是「删旧形态」的独立提交——让 diff 可读，也让回退点清晰。

## 9. 风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| 重构改变错误文案 | 插件侧依赖 `unknown method` / `capability denied` 文案 | 断言 `errors.Is(err, ErrCapabilityDenied)` 的测试不动；文案保持不变 |
| `requireFields` 排序行为被改 | 现有测试逐字断言 | 新校验器复用同一排序与「参数缺失: a, b」格式 |
| 参数校验提前，掩盖 service 层错误 | 某些依赖 service 报错的路径 | 只校验「必填 / 空串 / 类型」，取值域仍交 service（§5.1） |
| 内部测试 import model 被误加进生产文件 | gorm 进依赖图，硬约束 3 失效 | 验收点 7 每次改动都跑；建议同时加进 CI |
