## ADDED Requirements

### Requirement: 插件以实现语义化钩子的方式扩展站点

站点的扩展点 MUST 以核心预定义的具名钩子表达，插件 MUST 通过声明实现哪些钩子来扩展功能。

v1 钩子集合 MUST 至少包含：`post.created`、`post.updated`、`post.closed`、`comment.created`、
`user.registered`、`user.followed`、`report.created`、`moderation.decided`、`cron.daily`。

插件 MUST NOT 自行定义钩子名；未在核心钩子表内的钩子名 MUST 被忽略。

#### Scenario: 插件实现已定义钩子

- WHEN 一个插件的 `plugin.json` 声明实现 `post.created`
- THEN 每当有新帖子创建成功，该插件的对应处理函数 MUST 被调用

#### Scenario: 未定义的钩子名被忽略

- WHEN 一个插件的 `plugin.json` 声明了一个不在核心钩子表中的钩子名
- THEN 站点 MUST 忽略该声明
- AND MUST NOT 因此拒绝加载该插件

### Requirement: v1 只提供异步钩子，不得阻塞或回滚业务动作

v1 的钩子 MUST 在对应业务动作完成之后触发，MUST NOT 位于写路径的前置环节。

钩子执行失败、超时或插件崩溃 MUST NOT 影响触发它的业务动作：该动作 MUST 已经成功返回。

系统 MUST NOT 在 v1 提供任何 `before_create` 形态的写路径同步钩子。

需要前置拦截的场景 MUST 通过新增审核模式档位实现，MUST NOT 插入现有审核链中间。

#### Scenario: 钩子失败不影响发帖

- WHEN 新帖创建成功，但处理 `post.created` 的插件进程崩溃
- THEN 发帖请求 MUST 已成功返回
- AND 该帖 MUST 已持久化且状态不受影响
- AND 系统 MUST 记录一条插件失败日志

#### Scenario: 钩子超时不影响主流程

- WHEN 一个 `post.created` 钩子的处理时长超过该钩子声明的超时上限
- THEN 系统 MUST 终止本次钩子调用
- AND MUST NOT 回滚已创建的帖子

#### Scenario: 不存在前置同步钩子

- WHEN 检查 v1 的钩子清单
- THEN 清单中 MUST NOT 出现任何 `*.before_*` 形态的钩子

### Requirement: 插件以独立 Node 子进程运行

插件 MUST 以独立子进程运行（`node`），MUST NOT 加载进宿主 Go 进程。

子进程 MUST 由宿主通过标准输入输出上的 JSON-RPC 通信。
插件进程的崩溃 MUST NOT 导致宿主进程退出。

#### Scenario: 插件崩溃不影响宿主

- WHEN 某个插件进程异常退出
- THEN 宿主进程 MUST 继续运行
- AND 其余插件 MUST 继续工作
- AND 该插件状态 MUST 被置为异常

#### Scenario: 不做进程内执行

- WHEN 检查宿主的插件加载实现
- THEN 实现 MUST NOT 使用 `plugin.Open` 或任何进程内脚本求值来运行插件代码

### Requirement: 插件生命周期状态机可观测且可恢复

插件的状态 MUST 落库，状态集合 MUST 为 `installed` / `starting` / `running` / `crashed` / `stopped`。

插件崩溃后 MUST 指数退避重启，退避序列 MUST 从 1s 起并以 2 倍递增。
重启次数 MUST 有上限；超限后 MUST 停止重启并置为 `crashed`，MUST NOT 无限重启。

宿主重启后 MUST 依据落库状态恢复插件：先前为 `running` 的插件 MUST 被重新拉起。

#### Scenario: 崩溃后退避重启

- WHEN 一个运行中的插件进程崩溃
- THEN 系统 MUST 在 1s 后尝试重启
- AND 再次崩溃时退避时长 MUST 翻倍

#### Scenario: 超过重启上限停止

- WHEN 同一插件连续崩溃达到最大重启次数
- THEN 系统 MUST 停止重启
- AND 该插件状态 MUST 为 `crashed`

#### Scenario: 宿主重启恢复插件

- WHEN 宿主进程重启，且某插件重启前状态为 `running`
- THEN 宿主 MUST 重新拉起该插件进程
- AND 其状态 MUST 回到 `running`

### Requirement: 插件事件订阅为广播语义

插件对事件的订阅 MUST 为广播：同一事件 MUST 送达所有订阅了该事件的插件，MUST NOT 只送达其中一个。

该广播订阅 MUST 与现有 `eventbus` 的队列订阅语义并存且互不影响；
现有事件消费路径的「集群内每事件只消费一次」语义 MUST NOT 被改变。

#### Scenario: 多个插件收到同一事件

- WHEN 两个插件都声明订阅 `post.created` 且新帖创建成功
- THEN 两个插件 MUST 都收到该事件

#### Scenario: 不影响现有消费语义

- WHEN 新增广播订阅后，一个既有事件（如 `content.created`）被发布
- THEN 通知服务 MUST 仍然只消费一次该事件

### Requirement: 插件写数据必须经宿主 API 且受能力约束

插件 MUST NOT 直连站点数据库，MUST NOT 持有数据库连接串。

插件的数据写入 MUST 经宿主暴露的结构化 API，由宿主在业务层执行。

插件可调用的 API MUST 限于其在 `plugin.json` 中声明的 capability；
调用未声明的 capability MUST 在运行时被拒绝。

所有写类 API 调用 MUST 记录审计日志。

#### Scenario: 未声明能力被拒绝

- WHEN 一个仅声明了 `posts:read` 的插件尝试调用写类 API
- THEN 系统 MUST 拒绝该调用
- AND MUST 返回权限错误给插件

#### Scenario: 写操作留审计

- WHEN 插件经宿主 API 成功执行一次写操作
- THEN MUST 存在一条审计记录，包含插件标识、API 名与目标对象

### Requirement: 插件在钩子外不得继续调用宿主 API

一次钩子调用提供的上下文 MUST 仅在该次调用期间有效；
钩子返回后，插件对该上下文的使用 MUST 被拒绝。

#### Scenario: 钩子返回后上下文失效

- WHEN 插件在钩子处理函数返回之后仍尝试使用该次调用的上下文调用宿主 API
- THEN 系统 MUST 拒绝该调用
## ADDED Requirements

### Requirement: 插件安装必须经来源审批且展示能力清单

插件来源 MUST 登记于白名单并处于已审批状态方可安装；待审批或已拒绝的来源 MUST NOT 可安装。

安装前 MUST 向管理员展示该插件申请的 capability 清单与 npm 依赖清单，MUST 经管理员显式确认后才执行安装。

#### Scenario: 未审批来源不可安装

- WHEN 安装请求引用的来源处于待审批状态
- THEN 系统 MUST 拒绝安装

#### Scenario: 安装前展示能力

- WHEN 管理员发起一次插件安装
- THEN 系统 MUST 返回该插件申请的 capability 清单与依赖清单供确认
- AND 未经确认 MUST NOT 开始安装

### Requirement: 插件获取 npm 依赖限于插件目录

插件 MAY 声明 npm 依赖；安装时 MUST 在插件自身目录内执行依赖安装，依赖 MUST NOT 安装到宿主目录或全局作用域。

依赖安装失败 MUST 导致该次安装失败并回滚，MUST NOT 留下半安装状态。

#### Scenario: 依赖安装隔离

- WHEN 一个插件声明了 npm 依赖并执行安装
- THEN 其依赖 MUST 只存在于该插件目录内
- AND 宿主进程的依赖 MUST NOT 被修改

#### Scenario: 依赖失败回滚

- WHEN 插件的依赖安装失败
- THEN 该插件 MUST NOT 被标记为已安装
- AND 其残留文件 MUST 被清理

### Requirement: 插件进程不得继承站点的敏感配置

插件子进程 MUST NOT 继承宿主进程的环境变量，MUST 只接收白名单内的变量与 manifest 声明的插件配置。

白名单 MUST NOT 包含数据库连接串、管理员密码、会话密钥、OAuth 密钥等敏感值。

这是本方案下唯一可执行的数据访问防线，实现 MUST NOT 以「先继承再删除」的方式近似——
子进程启动时 `cmd.Env` MUST 由白名单重新构造，MUST NOT 基于 `os.Environ()` 构造。

#### Scenario: 敏感环境变量不下发

- WHEN 宿主拉起一个插件子进程
- THEN 该子进程的环境变量中 MUST NOT 出现 `DB_DSN`、`ADMIN_PASSWORD` 与任何 `*_SECRET` / `*_KEY`

#### Scenario: 不基于宿主环境构造

- WHEN 检查插件子进程的启动实现
- THEN `cmd.Env` MUST NOT 以 `os.Environ()` 为基底构造

### Requirement: 同用户的边界必须如实声明

插件子进程以与后端相同的系统用户运行。因此「插件 MUST NOT 直连数据库」在本版本中
MUST 被表述为**约定**，MUST NOT 在文档或界面中表述为强制隔离。

系统 MUST NOT 基于「插件已被限制无法访问数据库」这一前提做任何安全假设。

#### Scenario: 文档如实声明边界

- WHEN 查阅插件安全文档
- THEN 文档 MUST 明确说明插件与后端同用户、可自行连接数据库
- AND MUST NOT 声称插件被强制隔离
EOF
echo done