# 文档站（`app/docs/`）

对外文档站：Astro 7 + [Starlight](https://starlight.astro.build/)，静态输出，中英双语。
面向**使用者**：怎么装、怎么配、怎么管、怎么接、怎么改。

> 上级：[根 AGENTS.md](../../AGENTS.md) · 相邻：[Web](../web/AGENTS.md)

## 内容边界

| 写进本站 | 留在仓库 `docs/` |
|----------|------------------|
| 安装部署、配置、后台操作、API、MCP、运维、贡献指南 | plans / reviews / ADR / superpowers |
| 对外可读的架构与取舍 | 内部代号（R-04、H-04、P2/P3、M-xx） |
| 生成的端点索引、配置字段、指标目录 | 未实现能力的预告 |
| 排障与常见失败 | 密钥、示例密码、内网地址 |

**对外内容必须能自证**：端点、错误码、设置键、环境变量都要能在源码里找到。
不写「据说」「大约」「未来会」。

## 结构

```
app/docs/
├── astro.config.mjs            标题（按 locale）、logo、i18n、侧栏（7 组，手工排）
├── src/content.config.ts       collection = docs，Starlight docsLoader
├── src/content/docs/           中文（根 locale）
│   ├── index.mdx               首页（普通页 + LinkGroups / FeatureGrid）
│   ├── getting-started.md
│   ├── install/                choose-path / docker-compose / local / production
│   ├── configuration/          site / authentication / infrastructure / content / growth / customization
│   ├── administration/         overview / users / content / system
│   ├── operations/             architecture / data-and-events / observability / troubleshooting / upgrades
│   ├── api/                    conventions / authentication / examples / realtime / mcp / openapi
│   ├── reference/              endpoints / configuration / environment / errors / cli
│   └── faq.md roadmap.md contributing.md license.md glossary.md
├── src/content/docs/en/        英文，路径与中文一一对应（含 en/index.mdx）
├── src/components/home/        HomeLead / LinkGroups / FeatureGrid
├── src/components/reference/   EndpointIndex / SettingsReference / MetricsReference
├── src/data/reference.json     生成物（提交进仓库，--check 校验新鲜度）
├── src/styles/custom.css       品牌 token + Shell 覆写；组件样式写在各 .astro 内
├── scripts/sync-openapi.mjs    仓库根 openapi.json → public/
├── scripts/gen-reference.mjs   源码 → src/data/reference.json（端点/设置/指标）
├── scripts/rehype-base-links.mjs  正文站内绝对链接补 base
└── scripts/check-base-links.mjs   产物里的链接前缀与可达性检查
```

## 四条约定

1. **内容放本包内**：不要用 glob loader 指到 `../docs`。内部材料的节奏与对外导航无关。
2. **openapi.json 只同步不手放**：`prebuild`/`predev` 从仓库根复制。
3. **参考页是生成物**：端点索引、配置字段、指标目录读 `src/data/reference.json`，
   由 `scripts/gen-reference.mjs` 从 `openapi.json`、`model/setting.go`、`internal/metrics/*.go`
   生成。改了路由/设置/指标就重跑；JSON 已提交，CI 用 `--check` 拦漂移。
4. **侧栏手工排**：`astro.config.mjs` 显式列出全部条目，每个条目带 `translations.en`。
   新增页面必须同时登记中文与英文标题，否则页面存在但不可导航。

## 双语

- 中文在 `src/content/docs/`，英文在 `src/content/docs/en/`，**slug 必须一一对应**。
- 首页也必须两套：`index.mdx` 与 `en/index.mdx`。
- 英文不是逐字翻译：短句、主动语态，术语保留英文原名。
- 组件通过 `locale` prop 取文案（`EndpointIndex locale="en"`），不依赖页面语言。

## 设计层

- 品牌 token 在 `src/styles/custom.css`（`:root[data-theme='light']` 与默认深色两套），
  包括 `--fc-*` 与 Starlight 的 `--sl-color-*` 覆写。
- 组件样式写在 `.astro` 的 scoped `<style>` 里；需要压过 Markdown 全局样式时用
  `.组件根类 选择器` 提高特异性（见 `EndpointIndex.astro`）。
- 首页是普通文档页（`tableOfContents: false`），不是 splash：保留侧栏，靠
  `body:has(.fc-home)` 放宽正文宽度。
- `src/components/Head.astro` 只覆盖首页 `<title>`（避免「Feed Chamber · Feed Chamber 文档」），
  其余 head 条目原样透传；新增首页时同步这里与 `astro.config.mjs` 的 `components.Head`。
- biome 忽略 `**/*.astro`（它只解析 frontmatter，会误报未使用变量）；Astro 文件由
  `tsc --noEmit` 与构建负责。

## 写作约定

- 每页 frontmatter 必须给 `title` 与 `description`（进搜索与 SEO）。
- 页面骨架：一段话说明解决什么问题 → `## 前置条件` → `## 操作步骤` → `## 验证` → `## 常见失败`。
- 参考型内容用表格；列名具体，不超过 6 列。
- 代码块标语言；命令给完整可粘贴形式；中文与英文/代码之间加空格。
- 提示块用 `:::note` / `:::tip` / `:::caution` / `:::danger`，一页不超过 3 个。
- 站内链接以 `/` 开头、结尾带 `/`；正文链接由 rehype 插件补 base。
- MDX 里手写的 JSX 属性（组件 props 里的 `href`）**不走** rehype，必须用
  `import.meta.env.BASE_URL` 插值——组件里已经用 `withBase` 统一处理。

## 命令

```bash
pnpm dev:docs                                   # http://localhost:4321
pnpm build:docs                                 # 产物在 app/docs/dist
pnpm --filter @feedchamber/docs sync:reference  # 重新生成 reference.json
pnpm --filter @feedchamber/docs sync:reference:check  # CI：与源码是否一致
pnpm --filter @feedchamber/docs check           # astro sync + tsc + biome
```

> **搜索只在构建产物里有**：Pagefind 索引构建期生成，dev 下 `/pagefind/*` 404 属正常。

`check` 用 `tsc --noEmit` 而不是 `astro check`：后者在当前 pnpm 布局下解析
`@astrojs/check` 的传递依赖会失败（`@emnapi/runtime` 缺失）。

## GitHub Pages 部署（`.github/workflows/docs-pages.yml`）

推 `main` / `develop` 且改动命中 `app/docs/**`、`openapi.json`、该 workflow 时自动发布。

- **Node 版本独立**：Astro 7 要求 Node >= 22.23.4，`quality.yml` 用 Node 20（给 web/biome），
  不要为了省文件去抬它。
- **`site` / `base` 由 `DOCS_SITE` / `DOCS_BASE` 注入**：项目页挂在
  `https://<owner>.github.io/<repo>/`，缺 base 前缀会让站内跳转与 `_astro` 资源 404。
- 首次部署：仓库 Settings → Pages → Source 选 GitHub Actions；`github-pages` environment
  的分支策略要包含 `develop`。

### 三道护栏（都在 workflow 里，别删）

| 步骤 | 抓什么 |
|------|--------|
| `gen-reference.mjs --check` | 生成的参考数据是否与源码漂移 |
| `dist/openapi.json` 断言 | prebuild 的同步脚本是否漏跑 |
| `check-base-links.mjs` | 产物里站内绝对链接是否带 base、是否指向真实文件 |

### 正文链接为什么走 rehype

Starlight 只给侧栏与分页加 base，正文里手写的 `[x](/foo/)` 会原样保留。
`rehype-base-links.mjs` 在渲染管线统一补前缀；MDX 的 JSX 属性不经过管线，
所以组件内部用 `withBase()`。

## 本文件何时更新

- 新增/删除一级内容分组，或新增 `src/components/**` 下的类别
- 生成数据的来源或 `--check` 方式变化
- 部署方式（Pages / 自定义域名 / 多版本）变化
- 双语策略变化
