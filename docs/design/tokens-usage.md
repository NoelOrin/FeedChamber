# @orinforum/tokens 使用指南

日期：2026-06-13

## 概览

`@orinforum/tokens` 是 OrinForum monorepo 的共享设计令牌包，输出三类产物：

| 产物 | 用途 | 消费方 |
|------|------|--------|
| `tokens.css` | Tailwind v4 `@theme` 语义变量 | web + dashboard |
| `daisyui.css` | daisyUI v5 主题块 | web |
| `antd.ts` | AntD `ThemeConfig` 对象 | dashboard |

**核心原则：** 改 `tokens.config.ts` → 跑构建 → 两端同步生效。

---

## 基础用法

### 1. 在 CSS 中使用（web + dashboard）

```css
/* 在任意 .css 文件中 import */
@import "@orinforum/tokens/tokens.css";

/* 使用语义变量 */
.my-card {
  background-color: var(--color-card);
  color: var(--color-text);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-md);
}
```

### 2. 在 Tailwind 工具类中使用

语义变量已注册为 `@theme` CSS 变量，可直接在 Tailwind 工具类中引用：

```html
<div class="bg-card text-text border-border rounded-md shadow-md">
  内容
</div>
```

### 3. 在 AntD 组件中使用（dashboard）

```tsx
import { lightTheme, darkTheme, getAntdTheme } from '@orinforum/tokens/antd';
import { ConfigProvider } from 'antd';

// 静态导入
<ConfigProvider theme={lightTheme}>
  <App />
</ConfigProvider>

// 动态根据暗色模式
<ConfigProvider theme={getAntdTheme(isDark)}>
  <App />
</ConfigProvider>
```

### 4. 在 daisyUI 组件中使用（web）

`daisyui.css` 已定义 `orinlight` / `orindark` 主题，daisyUI 组件自动消费：

```html
<!-- 切换主题（useThemeMode hook 已自动处理） -->
<button class="btn btn-primary">按钮</button>
```

---

## 可用 CSS 变量

### 语义颜色

```css
/* 主色 */
--color-primary         /* 主色（默认主色） */
--color-primary-hover   /* hover 状态 */
--color-primary-active  /* active 状态 */
--color-primary-content /* 主色上的文字色 */

/* 背景 */
--color-bg              /* 页面背景 */
--color-bg-elevated     /* 卡片/面板背景 */
--color-card            /* 卡片背景 */
--color-item            /* 列表项背景 */

/* 文字 */
--color-text            /* 主文字色 */
--color-text-secondary  /* 次要文字 */
--color-text-muted      /* 弱化文字 */
--color-brand-text      /* 品牌文字色（亮/暗自动切换） */

/* 边框 */
--color-border          /* 边框色 */
```

### 原色阶（用于自定义派生）

```css
/* 亮色主题色阶 brand-1..10 */
--color-brand-1: #faf0ff;   /* 最浅 */
...
--color-brand-6: #8c2de4;   /* 原始种子色 */
...
--color-brand-10: #1e034a;  /* 最深 */

/* 暗色主题色阶 brand-dark-1..10 */
--color-brand-dark-1: #221830;  /* 最浅 */
...
--color-brand-dark-6: #7b2ac6;
...
--color-brand-dark-10: #edd2fb; /* 最深 */
```

### 圆角

```css
--radius-sm: 0.25rem;
--radius-md: 0.5rem;   /* 默认 */
--radius-lg: 0.75rem;
--radius-xl: 1rem;
--radius-2xl: 1.5rem;
--radius-full: 9999px;
```

### 阴影

```css
--shadow-sm: 0 1px 2px 0 rgb(0 0 0 / 0.05);
--shadow-md: 0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1);
--shadow-lg: 0 10px 15px -3px rgb(0 0 0 / 0.1), 0 4px 6px -4px rgb(0 0 0 / 0.1);
--shadow-xl: 0 20px 25px -5px rgb(0 0 0 / 0.1), 0 8px 10px -6px rgb(0 0 0 / 0.1);
--shadow-2xl: 0 25px 50px -12px rgb(0 0 0 / 0.25);
```

### 层级

```css
--z-dropdown: 1000;
--z-sticky: 1020;
--z-fixed: 1030;
--z-modal-backdrop: 1040;
--z-modal: 1050;
--z-popover: 1060;
--z-tooltip: 1070;
```

### 动画时长

```css
--duration-fast: 150ms;
--duration-normal: 300ms;
--duration-slow: 500ms;
```

---

## 暗色模式

语义变量在 `.dark` 类下自动切换：

```css
.dark {
  --color-primary: #64269f;    /* 暗色主色 */
  --color-bg: #1a1a1e;          /* 暗色背景 */
  --color-text: #f0f4f6;        /* 暗色文字 */
  /* ... */
}
```

**web：** `useThemeMode` hook 已自动设置 `.dark` class + `data-theme="orindark"/"orinlight"`。

**dashboard：** `useThemeMode` hook 同理，`getAntdTheme(isDark)` 返回对应 AntD 主题配置。

---

## 修改令牌

### 流程

1. 编辑 `packages/tokens/src/tokens.config.ts`（修改种子色）
2. 编辑 `packages/tokens/src/semantic.ts`（修改语义映射）
3. 构建：

```bash
pnpm --filter @orinforum/tokens tokens:build
```

4. 提交 `dist/` 变更：

```bash
git add packages/tokens/dist/
git commit -m "chore(tokens): update design tokens"
```

### 修改种子色（推荐）

种子色从 web 包 `styles.css` 的 `@property initial-value` 提取，新增颜色也应从 web 提取：

```ts
// packages/tokens/src/tokens.config.ts
export const seedColors: SeedColors = {
  brandPrimary: '#8c2de4',  // ← 改这里
  // ...
};
```

### 修改语义映射

```ts
// packages/tokens/src/semantic.ts
export function mapLightSemantic(brand, seed): SemanticColors {
  return {
    primary: brand[5],  // ← 改这里（使用 brand 色阶的哪个层级）
    primaryHover: brand[4],
    // ...
  };
}
```

---

## 包导出对照

| 导出路径 | 内容 |
|---------|------|
| `@orinforum/tokens/tokens.css` | CSS 变量（`@theme` 块 + `.dark` 覆盖） |
| `@orinforum/tokens/daisyui.css` | daisyUI 主题定义（`orinlight` + `orindark`） |
| `@orinforum/tokens/antd` | `lightTheme`、`darkTheme`、`getAntdTheme(isDark)` |

---

## 当前接入状态

| 包 | 接入 | 说明 |
|----|------|------|
| `web` | ✅ 完整 | `tokens.css` + `daisyui.css` 已 import，主色/语义变量已使用 |
| `dashboard` | ✅ 完整 | `tokens.css` 已 import，`lightTheme`/`darkTheme` 已接入 `ProShell` |

### dashboard 暗色模式

`useThemeMode` hook 已设置 `data-theme` 属性，`ProShell` 中已接入：

```tsx
// packages/dashboard/src/layout/ProShell.tsx
import { lightTheme, darkTheme } from '@orinforum/tokens/antd';
import { useThemeMode } from '@/hooks/useThemeMode';

const { isDark } = useThemeMode();
const antdThemeConfig = isDark
  ? { ...darkTheme, algorithm: antdTheme.darkAlgorithm }
  : lightTheme;

<ConfigProvider theme={antdThemeConfig}>
```

---

## 注意事项

- **`@ant-design/colors` 仅 devDependency**，不进入任何前端 bundle
- `dist/` 目录提交入库，避免每个 app 构建都跑生成脚本
- AntD 组件内部颜色（如 `colorSuccess`）使用 AntD 默认值，暂未注入完整色阶
- web 的历史 `@property` 迁移是分批进行，不要求一次清空