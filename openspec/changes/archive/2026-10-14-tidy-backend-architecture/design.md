## Context

`app/server` 的三组问题（死码/重复、响应出口分裂、装配不可读）互为因果：网关自成一套错误写法，是因为没人规定出口唯一；网关自造号段，是因为共享码表没有把它当成约束；容器里能起协程，是因为没人区分"构造"与"启动"。因此本 change 同时收规范与收代码，且顺序上先删死码（减少需要改写的面积），再统一出口，最后动装配。

## Decisions

### D1 成功信封的 `msg` 取 `"success"`，而不是把规范改成 `"成功"`

代码输出 `"成功"`，规范与 `scripts/stub-api.mjs` 都写 `"success"`。取舍：改代码 vs 走 openspec 提案改规范。

选择改代码。`msg` 在成功路径上没有消费者（`client.ts` 只在失败分支读它），而 `"success"` 是协议字面量、不承载文案；把中文留在失败码文案里才是它真正的作用。改规范反而要让前端桩、e2e 与既有规范文本一起动，收益为零。

### D2 网关错误体统一到 `{code,msg,data}`，同时豁免 OAuth/OIDC 协议端点

不豁免的后果：`/oauth/token` 返回论坛信封，外部 relying party 的 SDK 直接解析失败——这是把内部一致性凌驾于互操作契约。豁免范围严格限定在 RFC 规定的响应（token / authorize 错误 / JWKS / Discovery 元数据），网关里"客户端注册列表""authorize 回调给自建前端的 code"这类论坛自有面仍走信封，它们本来也是 `{"code":0,"message":"success"}` 的错误键名。

### D3 私有号段折回共享表，而不是给网关另立一档

`1092–1099` 这类号在 `api-client` 的 `ErrorCode` 里根本不存在，即"没有消费者"。保留它们等于维护一份没人读的枚举。折叠映射按语义走：凭证无效→`1002`、过期→`1003`、撤销→`1014`、参数→`2001`、资源不存在→`3001`、限流→`4001`、服务端→`5001`。

代价：错误粒度下降（"API Key 不存在"与"令牌错误"同为 `1002`）。补偿：具体原因进 `msg`，粒度需求出现时再在 `1xxx` 内正式登记新码，而不是各子系统私开。

封禁 404 这类"状态不能用业务码推导"的情形另开 `FailStatus`，避免为了套信封把 404 改成 403（那会泄露资源存在性）。

### D4 panic 边界挂在 `Logger` 之后

Iris 先跑 app 级中间件再跑 per-party；恢复点若在 Logger 之前，panic 请求不会留下完成日志，而"哪个 trace 崩了"正是最需要日志的时候。响应已开始写出时只 `StopExecution`，不追加半截信封。

指标 `feedchamber_http_panics_total` 不带标签：panic 文本与路由是无界取值，带标签会打爆序列——这与既有基数护栏一致。

### D5 容器拆 `New` / `Start`，HTTP 层改为显式 `Deps`

被否方案一：给 container 加 provider/registry 或引入 DI 库——项目偏好"修接缝而非加层"，且 38 个服务的构造顺序本来就有真实依赖，声明式容器不会让顺序变简单。
被否方案二：`handler.New(c *container.Container)` 保持原样——它让 handler 任取容器任何字段，"这层依赖什么"要 grep 42 个文件才知道；`Shutdown()`（worker 生命周期）也只能挂在 HTTP 类型上。

选择：`New` 纯构造、`Start` 起后台并返回带名字 cancel 的 `Runtime`、`server/wiring.go` 做容器→`handler.Deps` 的显式映射。新增服务漏接线从"运行时才发现"变成编译错误。

### D6 `gateway.Deps.KV` 类型化，接受测试替身成本

改成 `kvstore.Store` 后三个测试 stub 都需满足完整接口。做法是 embed 一个 `baseKV` 提供未使用 verb 的错误返回，比 6 处运行时断言 + "接错线就静默关掉限流"划算。原 `any` 的理由（"避免直接依赖 kvstore"）在同包已经 import kvstore 的事实前不成立。

### D7 保留 middleware/gateway 两份同名窄接口

合并需要一个双方都 import 的共享 types 包，属于"加层"。consumer-side 小接口本身是 Go 惯例，且两份接口已由同一个 composition root 用同一批实例填装（不存在两套权限缓存）。故只记录，不合并。

## Consequences

- 对外契约变化集中在错误体：字段名 `message`→`msg`、部分业务码值收敛。仓库内消费者已核对无引用。
- 网关不再是"可以随意写响应"的例外，后续新增网关端点会受 `api-style` 约束。
- 装配面变宽（`handler.Deps` + `wiring.go` 两处要改），换来依赖清单可审计与副作用位置明确。
- 死码删除后，`middleware` 与 `gateway` 的分工第一次在代码和文档里一致。
