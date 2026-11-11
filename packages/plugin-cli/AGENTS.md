# 插件开发 CLI（`packages/plugin-cli/`）

`fbp` 命令：脚手架、本地 mock 宿主、打包。**不发布运行时 SDK**——生成的协议代码内联在插件工程里，
插件打包产物不依赖任何 `@feedchamber/*` 包。

> 上级：[packages/AGENTS.md](../AGENTS.md) · 规范：[plugin-sdk](../../openspec/changes/2026-11-04-plugin-hooks/specs/plugin-hooks/spec.md) · 插件运行时：[internal/plugin](../../app/server/internal/plugin/AGENTS.md)

## 三个子命令

| 命令 | 作用 | 产物 |
|------|------|------|
| `fbp init <name>` | 生成 `<name>/` 插件工程 | `plugin.json` + `index.js` + `README.md` |
| `fbp dev` | 启动内置 mock 宿主并拉起插件 | 控制台输出钩子往返 |
| `fbp dev --hook <name>` | 指定要触发的钩子 | 同上 |
| `fbp build` | 打包 | `<name>-<version>.tgz` |

**没有 `publish`**：插件分发走「仓库地址 + 人工审批」，由站长在后台登记来源并审批。
CLI 侧不提供上传通道，这是刻意的——一个能从开发机直推插件到生产站点的命令，
等于把「谁可以装什么」的控制权从管理员手里拿走了。

## 为什么生成代码而不是发 SDK

`fbp init` 把协议客户端**内联**进 `index.js`，不装任何包。三条理由：

1. 插件作者不必等 CLI 发版就能改协议（自己改生成物）；
2. 插件打出的包里没有我们的代码，`fbp` 停更不会让存量插件失效；
3. 协议升级可以按插件逐个迁移，而不是一刀切。

代价：协议升级后老插件不会自动跟上。因此 `plugin.json` 带 `protocol_version`，
站点侧有兼容窗口。

## mock 宿主与真宿主的差异

`fbp dev` 的 mock 宿主**用同一个帧格式**（NDJSON + `id` 关联），但只做到「能跑通」：

| 方面 | mock 宿主 | 真宿主 |
|------|-----------|--------|
| 帧格式 | 一致 | 一致 |
| manifest 校验 | 不做 | 严格（协议版本、钩子白名单、entry 路径） |
| 鉴权 | 不做 | 插件专属 token |
| 宿主 API | 固定返回 `{ok:true}` | 真实 service + capability 校验 + 审计 |
| 事件来源 | 手动触发一次 | eventbus 广播 |

因此 **mock 能跑通不等于线上能跑通**——`capabilities` 漏申请在 mock 下看不出来
（mock 不校验），到真站点会被拒。这是 mock 的已知边界，不是缺陷。

## 两个踩过的坑

1. **shebang 只能有一份**。`tsup.config.ts` 的 `banner.js` 会加 `#!/usr/bin/env node`，
   所以 `src/index.ts` 里**不能**再写一条——重复的 shebang 会让产物第二行是
   `#!/usr/bin/env node` 而不是代码，Node 直接 `SyntaxError: Invalid or unexpected token`。
2. **`fbp build` 排除 `node_modules`**。依赖由站点在安装时按 `package.json` 自行安装
   （那里有超时与清理窗口）；把 `node_modules` 塞进包会让产物随依赖膨胀，且与站点的依赖树冲突。

## 改这里要注意

- 改协议帧格式 → 必须同步 `app/server/internal/plugin/ipc.go` 与
  `app/server/internal/plugin/AGENTS.md` 的 IPC 一节。CLI 的模板与真宿主是**两份实现**，
  这里没有共享代码（模板是字符串，宿主编解码是 Go）——两边靠文档对齐，靠 e2e 验收兜底。
- 新增钩子 → 同步模板里的 `hooks` 示例、`README.md` 的钩子清单、后端 `knownHooks`。
- 新增 capability → 同步模板的 `capabilities` 注释清单、`README.md`、后端 `knownCapabilities`。
- 新增子命令 → 本文件的上表 + `src/index.ts` 的 switch + `USAGE` 字符串。三者漏一处会让
  命令不可达或帮助信息过期。
- **不要给 `init` 生成物加 `package.json`**，除非确实需要 npm 依赖：空 `package.json` 会诱导
  作者 `npm install` 一堆用不上的包，而插件产物本该零依赖。

## 本文件何时更新

- 子命令增减或参数变化
- 协议帧格式变化（与后端 `ipc.go` 成对改）
- 钩子清单或 capability 清单变化
- mock 宿主与真宿主的行为差异变化（差异缩小是好消息，但仍要写清楚边界）
