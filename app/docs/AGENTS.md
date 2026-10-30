# 文档站（`app/docs/`）

对外文档站：Astro + [Starlight](https://starlight.astro.build/)，静态输出。
面向**使用者**（项目介绍 / 快速开始 / API / MCP / 架构 / 部署 / 二开 / FAQ），
不是内部工程材料。

> 上级：[根 AGENTS.md](../../AGENTS.md) · 相邻：[Web](../web/AGENTS.md)

## 内容边界（最重要的一条）

| 写进本站 | 留在仓库 `docs/`（不进站） |
|----------|---------------------------|
| 项目介绍、功能总览、快速开始 | plans / reviews / ADR / superpowers 计划 |
| 架构与分层（对外可读的版本） | 内部代号（P1/P2/P3、R-04、H-04） |
| API 约定、错误码、按域端点、openapi 下载 | 未实现能力的「预告」 |
| MCP 双向能力 | 任何密钥、示例密码、内网地址 |
| 部署安装、环境变量、常见故障 | 性能调优细节（无实测数据） |
| 二开 checklist 与扩展点 | 内部评审结论与争议过程 |

**对外内容必须能自证**：写进站的每个端点、错误码、环境变量都要能在源码里找到。
不写「据说」「大约」「未来会」。

## 结构

```
app/docs/
├── astro.config.mjs          站点标题、侧栏（8 组，手工排）、locale
├── src/content.config.ts     collection = docs，loader 用 Starlight 的 docsLoader
├── src/content/docs/         全部内容（本包内部，与仓库 docs/ 隔离）
│   ├── index.mdx             首页（splash 模板 + 入口卡片）
│   ├── getting-started.md    快速开始
│   ├── capabilities.md       功能总览
│   ├── glossary.md           术语表
│   ├── faq.md                常见问题
│   ├── roadmap.md            路线图与状态
│   ├── architecture/         架构总览 / 分层 / 可见性 / 事件
│   ├── api/                  约定 / 错误码 / 按域端点 / 下载
│   ├── mcp/                  概览 / Server / Client
│   ├── deploy/               compose / 本地 / 环境变量
│   └── extend/               新增模块 / 扩展点
├── scripts/sync-openapi.mjs  从仓库根同步 openapi.json 到 public/
└── public/openapi.json       构建产物（gitignore，不提交）
```

## 三条约定

1. **内容放本包内**。不要改成指向 `../docs` 的 glob loader：内部文档的改写节奏与
   对外导航无关，混在一起必然互相拖累。需要引用内部材料时，从站内链回仓库 URL。
2. **openapi.json 只同步不手放**。`prebuild`/`predev` 跑 `scripts/sync-openapi.mjs`，
   从仓库根复制（那份由 `make swag` 生成）。手放一份副本必然与真相源漂移。
3. **侧栏手工排**。`astro.config.mjs` 的 `sidebar` 显式列出全部条目——顺序、中文标题
   都受控。新增页面必须同时在这里登记，否则页面存在但不可导航（只有搜索能找到）。

## 写作约定

- 每页 frontmatter 至少给 `title` 与 `description`（`description` 进搜索结果与 SEO）
- 侧栏顺序用 `sidebar.order`（同目录内）
- 用 Starlight 的 aside 语法表达重点：`:::note` / `:::tip` / `:::caution` / `:::danger`
- 代码块标语言；命令给可直接粘贴的完整形式
- 中文正文与代码、英文术语之间加空格

## 命令

```bash
pnpm dev:docs      # 开发，默认 http://localhost:4321
pnpm build:docs    # 构建，产物在 app/docs/dist
pnpm --filter @feedchamber/docs check   # tsc --noEmit + biome check
```

> **搜索只在构建产物里有**：Pagefind 的索引是构建期生成的，`dist/pagefind/` 下有
> `pagefind.js` 与索引分片。dev 模式下 `/pagefind/*` 返回 404 是正常的，不是配置错误。

`check` 用 `tsc --noEmit` 而不是 `astro check`：后者在当前 pnpm 布局下解析
`@astrojs/check` 的传递依赖会失败（`@emnapi/runtime` 缺失）。tsc 已能覆盖类型错误。

## 多版本（预留，未启用）

现在是单版本（当前分支即唯一版本）。需要并行版本时：

1. 装 `starlight-versions` 插件
2. 把当前内容移入版本目录（如 `src/content/docs/v1/`）
3. 在 `astro.config.mjs` 的 integrations 里加插件配置

**本配置的侧栏结构不需要改**——这是当初把侧栏写成显式条目的原因之一。

## 本文件何时更新

- 新增/删除一级内容分组（即侧栏的 8 组之一）
- 内容边界规则变化
- openapi 同步方式变化（比如改成构建期渲染）
- 启用多版本
