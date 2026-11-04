# 插件钩子：子进程插件、语义化钩子与壳层插槽

提案状态：**草案（待评审）** · 创建日期：2026-11-04
取代：`2026-11-04-plugin-platform`（已删除）

## Why

`plugins` 表目前只存元数据（`model/plugin.go` 注释明写 *never executes plugin code*），
`docs/design/extension-points.md` 把「不支持第三方热加载、不提供 `.so` / 脚本沙箱 / 任意插件运行时」写成原则。
站点每一次功能扩展都必须改 Go 源码并重新构建，社区无法自带功能。

同时，站点自定义层（`site-customization`）只覆盖导航项：侧边栏的版面树、标签云、`MotionPanel`，
页脚状态区全部硬编码在 `components/aside/sidebar-content.tsx` 与 `components/layout/footer.tsx` 里。

参考对象是 Discourse 的插件语义（`lib/plugin/instance.rb`，183 个 `register_*` / `add_*` 钩子）：
**扩展点由核心预先定义，插件按名字填充**。这比让插件自造协议、站长再手工接线要好写得多。

但 Discourse 的同进程 `eval` 依赖 Ruby 动态特性，Go 没有等价物；
`plugin.Open` 又要求插件与宿主用完全相同的 Go 版本和依赖编译，且不支持交叉编译（会废掉 Makefile 的 windows/darwin 目标）。
因此本项目采用 **Node 子进程 + stdio IPC**：保留 npm 生态与热重启能力，不用 CGO，不影响交叉编译。

## What Changes

### 钩子模型

- 扩展点改为**语义化钩子**：核心定义钩子名，插件在 `plugin.json` 中声明实现哪些。
- **v1 只做异步钩子**：钩子在对应业务动作完成之后触发，MUST NOT 阻塞或回滚该动作。
- v1 钩子清单：`post.created`、`post.updated`、`post.closed`、`comment.created`、`user.registered`、
  `user.followed`、`report.created`、`moderation.decided`、`cron.daily`。
- **v1 MUST NOT 提供写路径同步钩子**（如 `post.before_create`）。需要前置拦截的场景，
  MUST 通过新增 `moderation_mode=plugin` 作为审核链的一个具名档位实现，MUST NOT 插入审核链中间。

### 子进程宿主

- 新增 `internal/plugin/`：`host.go`（生命周期状态机）、`supervisor.go`（`os/exec` 与退避重启）、
  `ipc.go`（stdio JSON-RPC 双向）、`hooks.go`（注册与分发）、`api.go`（绑定给插件的宿主 API）、`trust.go`（校验与能力授予）。
- 插件运行形态：`node plugins/<name>/index.js`，工作目录为插件目录。
- 插件状态机：`installed → starting → running → crashed → stopped`，MUST 落库。
- 崩溃重启 MUST 指数退避（1s → 2s → 4s → 8s → 16s），MUST 有最大重启次数，超限 MUST 置 `crashed` 并停止重启。

### 事件扇出

- 现有 `eventbus.Subscribe` 使用 `QueueSubscribe`（`internal/pkg/eventbus/eventbus.go`），
  集群内每个事件只有一个实例消费；插件需要**广播**。
- 新增 `EventBus.SubscribeBroadcast`（不带 queue group），`internal/plugin` 用它订阅后在本机 fan-out。
- 该改动 MUST 为纯增量，MUST NOT 修改现有 14 条事件消费路径的语义。

### 宿主 API（结构化写库）

- 插件 MUST NOT 直连数据库。写库 MUST 经宿主 API，由宿主在 `service` 层执行。
- 插件 MUST 获得 npm 依赖能力（安装时执行 `npm install`，在插件目录内隔离）。
- 宿主 API 清单：`posts.*`、`comments.*`、`users.*`、`notifications.send`、`kv.*`、`log.*`。
- 每个 API MUST 受 manifest 声明的 capability 限制，未被声明的调用 MUST 在运行时被拒绝。
- 所有写类 API 调用 MUST 落 `audit_logs`。

### 可信模型

- 插件来源 MUST 经白名单与人工审批；安装前 MUST 向管理员展示该插件申请的能力清单。
- 插件包 SHOULD 带签名，站点 SHOULD 验签。
- v1 不做插件市场前端，分发以「仓库地址 + 人工审批」进行。
- **插件子进程以与后端相同的系统用户运行**（v1 决策）。由此产生的边界 MUST 在文档中如实说明：
  「插件 MUST NOT 直连数据库」是**约定而非强制**，同用户进程有能力自行连接数据库。
- 唯一可执行的防线是**环境变量白名单**：拉起插件时 MUST 只下发白名单内的变量，
  MUST NOT 继承宿主环境（`DB_DSN`、`ADMIN_PASSWORD`、OAuth 密钥等 MUST NOT 出现）。

### 壳层插槽

- 新增 `slot_entries` 表，承载声明式条目；条目类型限于 `text` / `link` / `image` / `divider` / `widget`。
- 11 个固定插槽覆盖侧边栏、顶栏、页脚。
- **v1 的 `widget` 由插件异步写入**（插件在钩子中调用 `ctx.slots.set()` 写入条目），
  MUST NOT 在渲染时同步调用插件；渲染 MUST 只读库。
- 配置为空时 MUST 回退既有硬编码布局。

## Non-goals

- **不做同进程执行**：不引入 `plugin.Open`、不在宿主进程内跑插件代码。
- **不做写路径同步钩子**：v1 无 `*.before_*` 钩子。
- **不做容器编排**：不用 Docker，不引入 docker socket 代理。
- **不做 WebSocket 网关与 MCP 桥**：子进程用 stdio IPC，不需要网络协议栈。
- **不做渲染时同步取数**：插槽渲染 MUST NOT 调用插件；插件内容经异步钩子写入。
- **不做插件市场**：无浏览、评分、一键安装页。
- **不做插件间直接通信**：只经宿主中转。
- **不迁移 AstrBot / Discourse 插件**：协议与钩子名均不兼容。
- **不做裸 HTML 插槽**：插槽只接受结构化条目。

## 影响面

| 区域 | 变更 |
|---|---|
| `app/server/internal/plugin/` | 新增，约 1.5–2k 行 |
| `app/server/internal/pkg/eventbus/` | 新增 `SubscribeBroadcast`（增量） |
| `app/server/internal/model/` | 新增 `PluginInstall` / `PluginSource` / `SlotEntry`；`plugin.go` 扩字段 |
| `app/server/internal/service/plugin.go` | 重写（现仅元数据 CRUD） |
| `app/server/internal/router/routes_admin.go` | 补权限码（现为零）；新增插槽路由 |
| `app/server/internal/container/` | 装配 `Host`，`Start` 里拉起插件 |
| `app/web/components/slot/` | 新增插槽宿主与渲染器 |
| `app/web/components/aside/sidebar-content.tsx` 等 | 拆插槽 |
| `packages/plugin-cli/` | 新增（`fbp init` / `dev` / `build`） |
| `docs/design/extension-points.md` | 改写原则 |
| `openspec/specs/` | 新增 `plugin-hooks` / `shell-slots` capability |

## 风险

1. **插件与站点同机同用户（已确认接受）**：子进程与后端同用户，能读后端可读的文件系统，
   理论上可自行连接数据库。「插件不直连数据库」在本方案下是约定，不是强制边界。
   缓解：环境变量白名单（唯一可执行防线）、能力校验、审计。若将来需要强制隔离，须改用独立系统用户或容器。
2. **npm 供应链**：插件带 `package.json`，`npm install` 会拉任意依赖。缓解：安装时审计依赖清单并展示给管理员。
3. **异步钩子丢事件**：进程崩溃时在途钩子丢失。与现有 `eventbus` 的 fire-and-forget 语义一致，可接受；
   如需可靠投递须另行提案。
4. **热路径泄漏**：插件持有 `ctx` 并在钩子外继续调用宿主 API，可能形成越权路径。缓解：`ctx` 仅在钩子执行期间有效。
