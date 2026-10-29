# Markdown 编辑器（`components/markdown-editor/`）

tiptap（ProseMirror）富文本编辑器 + 双向 Markdown 桥 + **XSS 防线**。发帖、评论、私信共用。

> 上级：[components/AGENTS.md](../AGENTS.md) · 消费方：`components/post/composer.tsx`、`components/content/comment-list.tsx`、`components/pm/`

实测规模：11 个文件（含唯一一个 CSS 例外 `editor.css`）。

## 文件表

| 文件 | 职责 |
|------|------|
| `index.tsx` | `'use client'` 编辑器入口：`useEditor` + 工具栏 + 图片上传 + 草稿自动保存 |
| `extensions.ts` | tiptap 扩展集（StarterKit、Image、Link、Placeholder、TaskList/TaskItem） |
| `toolbar.tsx` | 工具栏按钮（加粗/斜体/标题/引用/代码/链接/图片/任务列表） |
| `link-popover.tsx` | 链接编辑浮层（走 `safe-url` 校验） |
| `image-upload.ts` | 图片上传：**魔数校验**（前几字节匹配声明的 MIME）→ `uploadService` → 插入节点 |
| `doc-to-md.ts` | ProseMirror doc → Markdown（取代旧 turndown 的 html→md） |
| `markdown-bridge.ts` | md → html 管线（remark-parse → remark-gfm → remark-rehype → **rehype-sanitize** → rehype-stringify）+ 防抖桥 |
| `sanitize.ts` | `markdownSanitizeSchema`（协议白名单 href: http/https/mailto、src: http/https）+ DOMPurify 兜底 |
| `safe-url.ts` | URL 白名单：协议集合 + 控制字符拒绝 + **百分号解码后再校验**（防 `%6a%61%76%61%73%63%72%69%70%74:` 这类混淆） |
| `draft.ts` | localStorage 草稿（前缀 `orinforum:draft:`，与旧版键语义一致，用户已有草稿可续读） |
| `editor.css` | tiptap 必需的样式（**本目录是 CSS module 禁令的唯一例外**） |

## 安全边界（本目录存在的核心理由）

三层，缺一层就破防：

1. **输入时**：`safe-url.ts` 白名单——链接只允许 `http/https/mailto`，图片只允许 `http/https`；先剥控制字符，再 `decodeURIComponent` 后重校验（否则 `%6a%61...` 绕过）。
2. **渲染前**：`markdown-bridge.ts` 的 `rehype-sanitize` + `sanitize.ts` 的 DOMPurify 兜底。
3. **展示时**：`components/content/markdown-renderer.tsx` 同样走 `rehype-sanitize`（详情页不经过编辑器，但走同一份 schema）。

`safe-url.ts` 与 `sanitize.ts` 是从 Solid 版**原样平移**的，逻辑一字未动——它们是 XSS 防线，**不要「顺手重构」**。改动必须有针对性测试或明确的安全论证。

## 编辑器 ↔ Markdown 的双向契约

- **存储格式是 Markdown**（后端只存 md）。编辑器内部是 ProseMirror doc，进入时 `mdToHtml` → 挂载，离开时 `docToMd` 回写。
- **两个方向的管线必须对称**：`doc-to-md.ts` 的节点映射与 `markdown-bridge.ts` 的 html 处理要覆盖同一组节点（标题/列表/任务列表/代码块/图片/链接/引用）。加一种节点要同时改两边，否则会出现「编辑后内容变形」。
- **草稿键语义是兼容契约**：`orinforum:draft:<key>` + `{ v, t }` 结构。改前缀或结构会让用户本地草稿读不出来。

## 改这里要注意

- **不要直接 `dangerouslySetInnerHTML`**：所有渲染经 `markdown-bridge` 或 `markdown-renderer`。
- **图片上传的魔数校验不要省**：只信 `file.type` 会被伪造的扩展名绕过。
- **`editor.css` 只放 tiptap 必需样式**，业务样式用 Tailwind。
- 编辑器依赖 `uploadService`（`@orinforum/api-client`）——后端上传接口变化时这里要同步。
- 加扩展先确认后端 Markdown 渲染端（`markdown-renderer.tsx` + `sanitize.ts` schema）也认这种节点，否则「编辑器里能看、详情页被剥掉」。

## 本文件何时更新

- 新增/删除编辑器扩展或节点类型（双向管线都要提）
- `safe-url` / `sanitize` 白名单调整（安全变更，必须说明理由）
- 草稿键或结构变化（破坏用户本地草稿）
- 上传链路变化
