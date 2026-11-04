# Dashboard-v2 模板基座全量重建 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 以 `/Users/noelorin/template/apps/web` 模板为基座并行新建 `app/dashboard-v2`，一次性全量迁移旧 `app/dashboard` 的 30 条路由，视觉对齐前台 Offset Litho，迁完删旧改名。

**架构：** 模板骨架（文件路由 + useMenu 生成菜单 + 自研布局 + design-tokens + BaseTable）保留；删除模板自带 api/tenant/auth-token 引擎，认证换成旧 dashboard 的 cookie 会话模型（HttpOnly cookie + zustand 哨兵），数据层统一 `@feedchamber/api-client` + TanStack Query（staleTime 60s）；pro-components 不带入，难页重写为 BaseTable。

**技术栈：** React 19、TanStack Router（tsr 文件路由）、antd 6、antd-style、Tailwind 4、TanStack Query、zustand、biome、vitest、vite(rolldown)、pnpm workspace。

**规格：** `docs/superpowers/specs/2026-10-15-dashboard-v2-template-rebuild-design.md`（已批准，commit d87fb12）

---

## 工作者必读上下文

- 仓库：`/Users/noelorin/group/Feed Chamber`（工作区根 `/Users/noelorin/group` **不是** git 仓库，git 命令必须在 Feed Chamber 内跑）。
- 分支：从 `migrate/react-vinext` 切出 `rebuild/dashboard-template`（任务 0 创建）。该仓库**有另一个 agent 会话在并行写代码**——只允许写 `app/dashboard-v2/`、`docs/superpowers/plans/` 和收尾任务明确列出的文件；`app/dashboard/` 里有别人的未提交改动，**一个字节都不要碰**。
- 包管理是 **pnpm**（模板脚本是 yarn 风格，不照抄）；Feed Chamber 用 lefthook + biome（root `biome.json`）。
- 模板路径下文简称 `TPL=/Users/noelorin/template/apps/web`。模板的 `src` 别名：`@` → `src`，`#` → `types`。
- 旧 dashboard 认证模型（读 `app/dashboard/src/stores/auth-store.ts`）：服务端 Set-Cookie HttpOnly 会话，JS 只持有 `master_auth` flag cookie；`isAuthenticated()` 来自 `@feedchamber/api-client`。**没有可读 token、没有前端刷新逻辑**——模板的 `tokenRefreshUtils`/`authenticatedAxiosTransport`/BroadcastChannel 机制整条删除，不移植。
- api-client 关键 API：`authService.login({login, password}) → {user, must_change_password}`、`authService.firstChangePassword({new_password})`、`authService.logout()`、`isAuthenticated()`、`clearAuthTokens()`、各资源 `xxxService.list({page, page_size, ...}) → {data, total}`。所有业务接口都是 POST RPC（`/list` `/create` `/{id}/update`）。
- 权限模型：路由守卫只看 `user.role ∈ {admin, moderator}`；按钮级权限码本次不铺（`utils/permission` 机制保留、`permList` 置空备用）。
- 验证门命令（在 `app/dashboard-v2` 下）：`pnpm gen-route`（改路由文件后必须跑，或依赖 vite 插件自动生成）、`pnpm typecheck`（tsc --noEmit）、`pnpm lint`（biome）、`pnpm vitest run`、`pnpm build`。
- 旧页面行为基准：迁移每页前先读旧文件 `app/dashboard/src/pages/<x>/index.tsx`，迁移后与旧行为逐项对照（列、筛选、批量操作、弹窗文案）。

## 文件结构（新包 `app/dashboard-v2`）

```
app/dashboard-v2/
├── package.json / tsconfig.json / tsr.config.json / biome.json / vitest.config.ts / vite.config.ts / index.html
├── dev/                        # 任务 2 删除（模板 mock backend）
├── types/                      # 模板 `#` 别名（enum/entity 保留，api 相关类型删）
└── src/
    ├── main.tsx / App.tsx      # 入口；App 去掉 AuthProvider/bootstrapAuthRuntime（任务 3）
    ├── router/index.tsx        # createRouter + RouteStaticData + isPublicAuthPath（任务 3 改白名单）
    ├── routeTree.gen.ts        # tsr 生成，勿手改
    ├── stores/
    │   ├── auth-store.ts       # 新增：移植旧 useAuthStore（任务 3，认证事实源）
    │   ├── userStore.ts        # 重写：AuthUser 投影 + permList 备用（任务 3）
    │   └── settingStore/routeStore/errorStore/tableStore.ts  # 模板保留
    ├── design-tokens/          # token/base.ts + light.ts + dark.ts + antdTheme.ts（任务 5 改）
    ├── layouts/                # 模板 dashboard/simple/_common 原样保留
    ├── router/hooks/useMenu.tsx # 去掉 AuthorizationEngine 依赖（任务 3）
    ├── utils/permission/       # 保留（permList 备用）
    ├── components/
    │   ├── baseTable/          # 保留，tableStore 类型改接（任务 6）
    │   ├── descriptions/index.tsx  # 新增：antd Descriptions 轻封装（任务 6）
    │   └── error/ errorStore 相关保留
    └── pages/
        ├── __root.tsx          # 守卫重写为三段逻辑（任务 3）
        ├── login/route.tsx     # 移植旧 LoginPage（任务 3）
        ├── first-change-password/route.tsx  # 移植（任务 4）
        └── (app)/
            ├── route.tsx       # DashboardLayout 包 Outlet，去 AuthGuard（任务 6）
            ├── (forum)/route.tsx        # staticData: 论坛管理
            │   ├── index.tsx            # /          运营概览
            │   ├── banned/route.tsx     # /banned
            │   ├── posts/route.tsx      # /posts
            │   ├── posts/detail/route.tsx  # /posts/detail ?id=
            │   ├── comments/… comments/detail/… trash/… tags/…
            ├── (operation)/route.tsx    # 运营管理：announcements reports moderation reports/detail spaces
            ├── (user-perm)/route.tsx    # 用户与权限：users users/detail invites roles roles/detail groups
            ├── (system)/route.tsx       # 系统设置：api-keys webhooks backups oauth-clients plugins security sessions settings audit-logs
            └── (misc)/…                 # search（不在旧菜单里，hideInMenu 或归运营组，见任务 10）
```

**URL 不变约束**：`/` 为运营概览首页；detail 页保留 `?id=` search 参数；`/settings?tab=` 保留；`/settings/dashboard` 保留为重定向路由。`(forum)` 等括号目录是 pathless 分组，不出现在 URL。

---

### 任务 0：分支与脚手架

**文件：** 创建 `app/dashboard-v2/`（复制自 TPL，排除 node_modules/dist）

- [ ] **步骤 1：创建分支**

```bash
cd /Users/noelorin/group/Feed Chamber
git switch migrate/react-vinext
git switch -c rebuild/dashboard-template
```

- [ ] **步骤 2：复制模板（排除依赖与产物）**

```bash
rsync -a --exclude node_modules --exclude dist --exclude .git /Users/noelorin/template/apps/web/ app/dashboard-v2/
rm -rf app/dashboard-v2/tests app/dashboard-v2/dev/mocks 2>/dev/null
```

- [ ] **步骤 3：改名包并接入 workspace**

编辑 `app/dashboard-v2/package.json`：`"name": "@feedchamber/dashboard-v2"`，scripts 里的 `yarn` 全部换成 `pnpm`（`gen-route`、`prebuild` 等保持语义）。查看 root `pnpm-workspace.yaml` 与 `package.json` 里 `dev:dashboard` 的现状（旧 dashboard 是怎么被纳入的就照抄：若 workspace 只含 `packages/*`，追加 `- app/dashboard-v2`）。

- [ ] **步骤 4：加共享包依赖、删模板业务依赖**

`app/dashboard-v2/package.json` dependencies 增加：

```json
"@feedchamber/api-client": "workspace:*",
"@feedchamber/shared-utils": "workspace:^",
"@feedchamber/tokens": "workspace:*"
```

删除确定不用的：`echarts`、`echarts-for-react`、`xlsx`、`eventsource`、`fetch-jsonp`、`highlight.js`、`nprogress`、`ramda`、`react-helmet-async`、`react-sticky-box`、`simplebar-react`、`@dnd-kit/core`、`@dnd-kit/modifiers`、`@dnd-kit/sortable`、`copy-to-clipboard`、`uuid`、`await-to-js`、`mitt`、`color`（含对应 `@types/*`）。**保留** `framer-motion`（`components/animate` 在用）、`sonner`。其余（`lodash-es`、`dayjs`、`axios` 等）暂留，任务 3 删完 api 层后按 tsc 报错二次裁剪。

- [ ] **步骤 5：安装并验证 dev 起动**

```bash
pnpm install
pnpm --filter @feedchamber/dashboard-v2 gen-route
pnpm --filter @feedchamber/dashboard-v2 dev   # 预期: vite 起在 3001，登录页可渲染（登录会失败，属正常）
```

- [ ] **步骤 6：Commit**

```bash
git add app/dashboard-v2 pnpm-workspace.yaml pnpm-lock.yaml package.json
git commit -m "chore(dashboard): 以 admin-template 为基座脚手架 dashboard-v2"
```

### 任务 1：vite / tsr / biome 配置去模板化

**文件：** 修改 `app/dashboard-v2/vite.config.ts`、`tsr.config.json`、`index.html`

- [ ] **步骤 1：重写 `vite.config.ts`**——删除 `dev/mockBackend` import 与 `autoMockBackend` 插件、删除 `lefthook-plug` 插件（Feed Chamber 用 root lefthook，勿在 vite 里 exec）、删除 axios alias。保留：tanstackRouter/react/tailwindcss 插件、optimizeDeps、dedupe、react alias（workspace 根去多实例）。proxy 改为：

```ts
server: {
  open: false,
  host: true,
  port: 3001,
  proxy: { "/api": { target: env.VITE_API_TARGET || "http://localhost:8080", changeOrigin: true } },
},
```

- [ ] **步骤 2：`index.html`**——title 改为「后台管理」，删模板 favicon/统计残留。
- [ ] **步骤 3：验证**：`pnpm --filter @feedchamber/dashboard-v2 dev` 起动、`pnpm --filter @feedchamber/dashboard-v2 exec tsc -b --noEmit` 通过（此时尚有 api 层引用，若 mock 相关报错先删 `dev/` 目录与 `types/` 里的 mock 类型）。
- [ ] **步骤 4：Commit** `git commit -am "chore(dashboard): vite 配置去模板化，/api 代理指向本机 8080"`

### 任务 2：认证换血（auth-store / userStore / 删模板 api+auth 引擎 / __root 守卫 / login 页）

这是最大的一根不可分割轴，拆开任何一半 tsc 都不绿。全程以 tsc 报错为清障清单。

**文件：**
- 创建：`src/stores/auth-store.ts`（移植自 `app/dashboard/src/stores/auth-store.ts`，内容逐行相同）
- 重写：`src/store/userStore.ts`
- 重写：`src/pages/__root.tsx` 的守卫部分
- 修改：`src/App.tsx`、`src/router/index.tsx`、`src/router/hooks/useMenu.tsx`
- 替换：`src/pages/login/route.tsx`（+配套 `local.tsx` 删除）
- 删除：`src/api/`、`src/tenant/`、`src/auth/`、`src/hooks/export/`、`src/hooks/hiddenByEnv/`、`src/pages/(app)/examples/`、`src/pages/auth/`、`src/components/config/runtime/`、`types/` 中 api/tenant 类型

- [ ] **步骤 1：移植 auth-store**

```bash
mkdir -p app/dashboard-v2/src/stores
cp app/dashboard/src/stores/auth-store.ts app/dashboard-v2/src/stores/auth-store.ts
```

内容不改（它只依赖 `@feedchamber/api-client` 的 `clearAuthTokens`/`isAuthenticated`，均存在）。

- [ ] **步骤 2：重写 userStore** 为认证投影（保留模板 useMenu/PermissionGate 消费的形状）：

```ts
import { create } from "zustand";

export interface AuthUser {
	id: string;
	username: string;
	email: string;
	role: string;
	avatar?: string;
}

type UserStore = {
	userInfo: Partial<AuthUser>;
	permList: string[];
	roleAdmin: boolean;
	permissionLoaded: boolean;
	actions: {
		setUserInfo: (user: Partial<AuthUser>) => void;
		clearUserInfoAndToken: () => void;
		setPermList: (list: string[]) => void;
		setRoleAdmin: (v: boolean) => void;
	};
};

const useUserStore = create<UserStore>()((set) => ({
	userInfo: {},
	permList: [],
	roleAdmin: false,
	permissionLoaded: true, // 本次按角色放行，不做逐码鉴权；PermissionGate 备用
	actions: {
		setUserInfo: (user) =>
			set({
				userInfo: user,
				roleAdmin: user.role === "admin",
			}),
		clearUserInfoAndToken: () =>
			set({ userInfo: {}, permList: [], roleAdmin: false, permissionLoaded: true }),
		setPermList: (list) => set({ permList: list }),
		setRoleAdmin: (v) => set({ roleAdmin: v }),
	},
}));

export const useUserInfo = () => useUserStore((s) => s.userInfo);
export const useUserActions = () => useUserStore((s) => s.actions);
export const useShowPermFn = () => () => ""; // 页面未接权限码前恒可见
export default useUserStore;
```

- [ ] **步骤 3：删除目录** `src/api`、`src/tenant`、`src/auth`、`src/hooks/export`、`src/hooks/hiddenByEnv`、`src/pages/(app)/examples`、`src/pages/auth`、`src/components/config/runtime`，以及 `src/pages/login/local.tsx`。全局搜 `from "@/api`、`from "#/api`、`from "@/auth`、`from "@/tenant`、`tokenRefreshUtils`，逐个修复引用点（预计集中在：`App.tsx`、`pages/__root.tsx`、`router/hooks/useMenu.tsx`、`store/tableStore.ts`、`utils/permission/hooks.ts`+`utils.ts`、`components/baseTable/index.tsx`、`hooks/meta`、`components/error/*`）。
- [ ] **步骤 4：改 `useMenu.tsx`**——删除 `createAuthorizationEngine`/`getAuthState`/`AuthorizationEngine` 相关 import 与 `canShowRoute` 的 engine 参数，`canShowRoute` 改为 `authorization === undefined || useUserStore.getState().roleAdmin`（菜单不做逐码过滤）。
- [ ] **步骤 5：改 `tableStore.ts` 与 `baseTable/index.tsx`** 的 `#/api` 类型引用为本地类型（`Record<string, unknown>` 或 api-client 类型，任务 6 再精化）。
- [ ] **步骤 6：重写 `__root.tsx` 守卫**（三段逻辑复刻旧 `router.tsx:78-95`）：

```ts
beforeLoad: async ({ location }) => {
	const { getRouter, isPublicAuthPath } = await import("@/router");
	const routeTree = getRouter().routeTree;
	await setMenuListMap(routeTree);

	const p = location.pathname;
	const isPublic = isPublicAuthPath(p) || p === "/first-change-password";
	if (!isPublic) {
		const { accessToken, mustChangePassword, user } = useAuthStore.getState();
		if (!accessToken) {
			throw redirect({ to: "/login", search: { redirect: p } as never });
		}
		if (mustChangePassword) throw redirect({ to: "/first-change-password" });
		if (!user?.role || (user.role !== "admin" && user.role !== "moderator")) {
			useAuthStore.getState().logout();
			throw redirect({ to: "/login" });
		}
	}
},
```

`RootLayout` 组件同步简化：删除 `useAuthState`/`getAuthorizationDecision`，改为读取 `useAuthStore` 的同三段判断做渲染级兜底（denied 时 return null）。删除 `__root.tsx` 里 `/` → `/dashboard` 的 redirect（本应用 `/` 就是首页）。

- [ ] **步骤 7：改 `router/index.tsx`** 的 `isPublicAuthPath` 增加 `/first-change-password`；`RouteStaticData` 删除 `authorization` 字段。
- [ ] **步骤 8：改 `App.tsx`**——删除 `AuthProvider`、`AuthProjectionBridge`、`bootstrapAuthRuntime` 动态 import 与 `authFailure` 分支；`<MotionLazy><Suspense><RouterProvider/></Suspense></MotionLazy>` 直接挂。
- [ ] **步骤 9：移植 login 页** 到 `src/pages/login/route.tsx`：

```ts
import { createFileRoute, redirect } from "@tanstack/react-router";
import { LoginPage } from "./LoginPage";

export const Route = createFileRoute("/login")({
	beforeLoad: () => {
		const { accessToken, user } = useAuthStore.getState();
		if (accessToken && (user?.role === "admin" || user?.role === "moderator")) {
			throw redirect({ to: "/" });
		}
	},
	component: LoginPage,
});
```

`LoginPage.tsx` 从 `app/dashboard/src/pages/login/index.tsx` 移植，改动三处：① 删除组件内 `ConfigProvider theme={…}` 包壳与 `getDashboardTokens/getAntdTheme/useThemeMode`（全局 App.tsx 已提供主题；背景色用 `var(--color-bg-layout)`）；② `useAuth` 改从 `@/stores/auth-store` import；③ `getSafeRedirectUrl` 原样保留（防 open redirect，勿删）。`pages/login/index.tsx` 不再需要。

- [ ] **步骤 10：验证门**

```bash
pnpm --filter @feedchamber/dashboard-v2 exec tsc -b --noEmit
pnpm --filter @feedchamber/dashboard-v2 lint
cd app/server && DB_TYPE=sqlite DB_DSN=/tmp/of_dash_test.db go run main.go -e dev &   # 起后端（先 pnpm 安装依赖已就绪；ADMIN 密码看启动日志）
pnpm --filter @feedchamber/dashboard-v2 dev   # 浏览器：登录 admin → 应落到 '/' 且守卫生效（未登录访问 /users 应回 /login）
```

- [ ] **步骤 11：Commit** `git commit -am "feat(dashboard): 认证换血——cookie 会话模型接入模板骨架，删除模板 api/auth 引擎"`

### 任务 3：first-change-password 移植

**文件：** 创建 `src/pages/first-change-password/route.tsx` + `FirstChangePasswordPage.tsx`

- [ ] **步骤 1：** 从 `app/dashboard/src/pages/first-change-password/index.tsx` 移植组件，改动与 login 页相同（去局部 ConfigProvider/tokens，`useAuth` 改 `@/stores/auth-store`）。路由声明（公开路径已在任务 2 白名单）：

```ts
export const Route = createFileRoute("/first-change-password")({
	beforeLoad: () => {
		const { accessToken, mustChangePassword } = useAuthStore.getState();
		if (!accessToken) throw redirect({ to: "/login" });
		if (!mustChangePassword) throw redirect({ to: "/" });
	},
	component: FirstChangePasswordPage,
});
```

- [ ] **步骤 2：验证**：用种子 admin 登录（首次登录 must_change_password=true）→ 强制跳转 → 改密 → 回登录页。
- [ ] **步骤 3：Commit** `git commit -am "feat(dashboard): first-change-password 移植"`

### 任务 4：design-tokens 对齐 Offset Litho

**文件：** 修改 `src/design-tokens/token/base.ts`、`token/light.ts`、`token/dark.ts`、`antdTheme.ts`、`src/index.css`

- [ ] **步骤 1：提取设计值。** 读三处来源并制表：
  1. `packages/tokens/dist/tokens.css`（CSS 变量：`--color-primary: #8c2de4`、`--color-primary-hover/active`、`--color-seed-*` 背景/卡片/边框、`--color-error-dashboard: #ff4d4f` 等）；
  2. `app/web/app/globals.css` 的 archive 体系类（`.eyebrow` `.index-num` `.rule` `.display-title` 的 font-family/size/weight/letter-spacing）；
  3. `packages/tokens/dist/antd.js` 的 `getAntdTheme(isDark)`（现成 antd ThemeConfig，直接可用）。
- [ ] **步骤 2：改 `light.ts` / `dark.ts`**——以 `getAntdTheme(isDark)` 返回值为基础 spread，再覆盖 archive 差异项：

```ts
import type { ThemeConfig } from "antd";
import { getAntdTheme } from "@feedchamber/tokens/antd";

const lightTokens = {
	...getAntdTheme(false).token,
	// 以下值取自步骤 1 提取表，逐项填入（示例形状，值以提取结果为准）：
	colorPrimary: "#8c2de4",           // ← --color-primary
	colorError: "#ff4d4f",             // ← --color-error-dashboard
	// colorTextBase / colorBgLayout / colorBorder / borderRadius / fontFamily …
} satisfies NonNullable<ThemeConfig["token"]>;
export default lightTokens;
```

- [ ] **步骤 3：改 `base.ts`**——圆角/间距/字体对齐 archive（`fontFamily` 填 web 同款字体栈；标题字号若 archive 用非默认值，落进 `antdTheme.ts` 的 `components.Typography/Menu` token）。
- [ ] **步骤 4：`index.css`** 引入 `@feedchamber/tokens/dist/tokens.css`（Tailwind `@theme` 变量层，页面可用 `var(--color-*)`）。
- [ ] **步骤 5：验证**：dev 起动，light/dark 切换截图登录页 + 空布局，与 `app/web` 页面观感对照（主色 #8c2de4、背景/卡片色一致）。
- [ ] **步骤 6：Commit** `git commit -am "feat(dashboard): design-tokens 对齐 Offset Litho（@feedchamber/tokens 同源）"`

### 任务 5：(app) 骨架、分组菜单与 BaseTable/Descriptions 适配

**文件：**
- 修改：`src/pages/(app)/route.tsx`（去 AuthGuard）、`src/components/baseTable/index.tsx`、`src/store/tableStore.ts`
- 创建：`src/components/descriptions/index.tsx`
- 创建：四个分组 route 文件 `(forum)/(operation)/(user-perm)/(system)/route.tsx`

- [ ] **步骤 1：`(app)/route.tsx`** 去掉 `AuthGuard`：

```tsx
function RouteComponent() {
	return (
		<DashboardLayout>
			<Outlet />
		</DashboardLayout>
	);
}
```

- [ ] **步骤 2：分组 route 文件**（四个同构，以 forum 为例）：

```tsx
// src/pages/(app)/(forum)/route.tsx
import { createFileRoute, Outlet } from "@tanstack/react-router";
import { AppstoreOutlined } from "@ant-design/icons";

export const Route = createFileRoute("/(app)/(forum)")({
	staticData: { title: "论坛管理", icon: <AppstoreOutlined />, order: 2 },
	component: () => <Outlet />,
});
```

四组 order：`/` 首页 order 1（在任务 6 的 `index.tsx` staticData）；forum 2；operation 3；user-perm 4；system 5。图标沿用旧 ProShell 的映射（forum=AppstoreOutlined、operation=SafetyOutlined、user-perm=TeamOutlined、system=SettingOutlined）。

- [ ] **步骤 3：BaseTable 类型适配**：`baseTable/index.tsx` 与 `tableStore.ts` 中 `#/api` 引用改 api-client 类型或 `Record<string, unknown>`；跑通 `tsc`。
- [ ] **步骤 4：新增 Descriptions 轻封装**（供 posts/detail、comments/detail 的 39 处 ProDescriptions 替换）：

```tsx
// src/components/descriptions/index.tsx
import { Descriptions, type DescriptionsProps } from "antd";

export type ProDescItem = {
	label: string;
	children: React.ReactNode;
	span?: number;
};

/** ProDescriptions 的平替：内容就是普通 children，不带编辑态 */
export function ProDescriptions({
	items,
	column = 2,
	bordered,
}: {
	items: ProDescItem[];
	column?: number;
	bordered?: boolean;
}) {
	const antdItems: DescriptionsProps["items"] = items.map((it) => ({
		key: it.label,
		label: it.label,
		children: it.children,
		span: it.span,
	}));
	return <Descriptions items={antdItems} column={column} bordered={bordered} size="small" />;
}
```

- [ ] **步骤 5：验证**：`pnpm gen-route && tsc -b --noEmit && lint`。
- [ ] **步骤 6：Commit** `git commit -am "feat(dashboard): (app) 分组骨架 + BaseTable/Descriptions 适配"`

### 任务 6：首页（运营概览）+ 易档代表 tags（两个完整 worked example）

**文件：**
- 创建：`src/pages/(app)/(forum)/index.tsx`（路由 `/`，运营概览）、`src/pages/(app)/(forum)/tags/route.tsx`

- [ ] **步骤 1：运营概览**——移植 `app/dashboard/src/pages/dashboard/index.tsx`（276 行，Statistic 卡片 + Table，无图表库）。改写为 Query：

```tsx
const statsQuery = useQuery({
	queryKey: ["admin", "stats"],
	queryFn: () => adminService.stats(),   // 以旧页面实际调用的 service 为准
	staleTime: 60_000,
});
```

标题 `<h1>` 换 `Typography.Title level={3}`；`ThemeModeCtx.tokens` 引用删除（用 CSS 变量或 antd token）。statistic 卡片布局用原页面的 Grid 结构。
- [ ] **步骤 2：tags 页**（易档标准工序的完整示例）。旧源 `app/dashboard/src/pages/tags/index.tsx`（原生 Table + 手写分页）。目标文件：

```tsx
// src/pages/(app)/(forum)/tags/route.tsx
import { createFileRoute } from "@tanstack/react-router";
import { TagsOutlined } from "@ant-design/icons";
import TagListPage from "./TagListPage";

export const Route = createFileRoute("/(app)/(forum)/tags/")({
	staticData: { title: "标签管理", icon: <TagsOutlined />, order: 60 },
	component: TagListPage,
});
```

```tsx
// src/pages/(app)/(forum)/tags/TagListPage.tsx —— 手写分页 → useQuery 化的标准形态
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Table, Button, Popconfirm, message } from "antd";
import { tagService } from "@feedchamber/api-client";

export default function TagListPage() {
	const [page, setPage] = useState(1);
	const [pageSize, setPageSize] = useState(20);
	const qc = useQueryClient();

	const { data, isLoading } = useQuery({
		queryKey: ["tags", { page, pageSize }],
		queryFn: () => tagService.list({ page, page_size: pageSize }),
		staleTime: 60_000,
		placeholderData: (prev) => prev,   // 翻页不闪空
	});

	const invalidate = () => qc.invalidateQueries({ queryKey: ["tags"] });
	// 删除等 mutation：useMutation({ mutationFn: tagService.delete, onSuccess: () => { message.success("删除成功"); invalidate(); } })
	// 列定义、行操作、弹窗从旧文件逐列对照平移，文案不改。

	return (
		<Table
			rowKey="id"
			loading={isLoading}
			dataSource={data?.data ?? []}
			pagination={{
				current: page, pageSize, total: data?.total ?? 0,
				showSizeChanger: true,
				onChange: (p, ps) => { setPage(p); setPageSize(ps); },
			}}
			/* columns 从旧文件平移，valueEnum → 普通 render（见任务 7 posts 示例） */
		/>
	);
}
```

- [ ] **步骤 3：验证**：dev 中登录 → `/` 渲染统计卡、`/tags` 列表分页可用；菜单出现「论坛管理」组。
- [ ] **步骤 4：Commit** `git commit -am "feat(dashboard): 运营概览首页 + 标签管理（useQuery 化范式）"`

### 任务 7：难档示例 posts 列表（ProTable → BaseTable 完整 worked example）

**文件：** 创建 `src/pages/(app)/(forum)/posts/route.tsx` + `PostListPage.tsx`

- [ ] **步骤 1：写目标文件。** 这是全项目唯一一处 ProTable→BaseTable 的完整示范，后续难页照此办理。要点映射：

| 旧（ProTable） | 新（BaseTable + Query） |
|---|---|
| `request={async (params) => …}` | `requestFn: (params) => postService.list({page: params.current, page_size: params.pageSize, status: params.status})`（BaseTable 内部持有 loading/data，见其 `APIProps.requestFn` 签名） |
| `actionRef.current?.reload()` | `qc.invalidateQueries({ queryKey: ["posts"] })`（若 BaseTable 自带 refetch 则用其返回的 refetch） |
| `ProColumns.valueEnum` + `valueType: 'select'` | 列定义改 `ColumnType<PostListItem>`；筛选项放 BaseTable 的 filter 配置（`Select options`）；渲染用 `render: (r) => <Tag color={statusEnum[r.status]?.status}>{statusEnum[r.status]?.text}</Tag>` |
| `tableAlertOptionRender` 批量操作 | BaseTable 的 selection 插槽/自绘 alert 条，按钮组原样平移（批量置顶/取消/锁定/解锁/删除 + Popconfirm） |
| `toolBarRender` | BaseTable toolbar 插槽放「刷新」按钮 |
| `search={{labelWidth:'auto'}}` | BaseTable 内置搜索区（读其 props，能力不足的筛选项降级为列头筛选或省略——在 PR 描述里记录降级项） |

- [ ] **步骤 2：验证**：`/posts` 列表、状态筛选、批量操作、置顶/锁定/删除全部与旧页行为一致（对照旧文件逐项点一遍）。
- [ ] **步骤 3：Commit** `git commit -am "feat(dashboard): 帖子管理迁移（ProTable→BaseTable 范式页）"`

### 任务 8：难档其余 6 页

每页独立小节，按任务 7 的映射表 + 任务 6 的 useQuery 形态办理。每页完成后：对照旧行为逐项点验 → 该页 tsc/lint → commit。

- [ ] **posts/detail**（`src/pages/(app)/(forum)/posts/detail/route.tsx`，`validateSearch: (s) => ({ id: (s.id as string) || '' })`；17 处 ProDescriptions → `ProDescriptions`；内嵌评论 ProTable → BaseTable 复用任务 7 形态；数据 `postService.get(id)`）
- [ ] **comments**（`(forum)/comments/route.tsx`；contentService；同 posts 的筛选/批量模式）
- [ ] **comments/detail**（22 处 ProDescriptions → ProDescriptions；`validateSearch` 同上）
- [ ] **banned**（`(forum)/banned/route.tsx`；bannedService；时长表单从旧页平移）
- [ ] **announcement**（`(operation)/announcements/route.tsx`；announcementService；注意旧菜单路径是 `/announcements`）
- [ ] **roles/detail**（`(user-perm)/roles/detail/route.tsx`，348 行权限矩阵 UI 最复杂：`lib/permissions.ts` 的 GROUP_LABELS/SPACE_SCOPED 权限码分组逻辑**原样复制**到新包 `src/lib/permissions.ts`，仅换容器组件与主题引用）

每页 commit 信息：`feat(dashboard): <页名>迁移`。

### 任务 9：易档批次 A——论坛管理组剩余页 + 杂项

按任务 6 tags 的标准形态（平移 + useQuery 化 + route.tsx staticData）。每页先读旧源，迁移后对照点验，逐组 commit。

- [ ] **trash**（`(forum)/trash/route.tsx`；trashService，恢复/清除操作 → useMutation）
- [ ] **search**（`(operation)/search/route.tsx`，旧菜单无此项，`staticData: { title: "全局搜索", hideInMenu: true }`，路由 `/search` 保留）
- [ ] **groups**（`(user-perm)/groups/route.tsx`；groupService）
- [ ] **invites**（`(user-perm)/invites/route.tsx`；inviteService）
- [ ] **webhooks**（`(system)/webhooks/route.tsx`；webhookService）
- [ ] **api-keys**（`(system)/api-keys/route.tsx`）
- [ ] **backups**（`(system)/backups/route.tsx`；注意下载类操作用旧页原实现）
- [ ] **oauth-clients**（`(system)/oauth-clients/route.tsx`）
- [ ] **plugins**（`(system)/plugins/route.tsx`）
- [ ] **security**（`(system)/security/route.tsx`）
- [ ] **sessions**（`(system)/sessions/route.tsx`；吊销操作 → useMutation）
- [ ] **audit-logs**（`(system)/audit-logs/route.tsx`）

Commit：`feat(dashboard): 易档批次A（trash/search/groups/invites + 系统组 7 页）`（可拆 2-3 个 commit，按组）。

### 任务 10：易档批次 B 收尾 + 中档 8 页

- [ ] **中档按任务 6/7 混合形态**，每页要点：
  - **users**（`(user-perm)/users/route.tsx`，264 行手写分页 + 批量 → BaseTable + useQuery/useMutation；跳详情 `/users/detail?id=`）
  - **users/detail**（`(user-perm)/users/detail/route.tsx`；`validateSearch` id；资料/角色变更操作平移）
  - **reports**（`(operation)/reports/route.tsx`）与 **reports/detail**（`(operation)/reports/detail/route.tsx`；状态流转 `/{id}/status` → useMutation + invalidate）
  - **moderation**（`(operation)/moderation/route.tsx`；moderationService）
  - **spaces**（`(operation)/spaces/route.tsx`；树形数据展示从旧页平移）
  - **spaces/permissions**（并入 spaces 目录为子组件 `SpacesPermissions.tsx`，路由入口不变则挂 `/spaces/permissions` 单独 route.tsx，`validateSearch` 以旧页为准）
  - **settings**（`(system)/settings/route.tsx`；`validateSearch: (s) => ({ tab: (s.tab as string) || 'site' })`；`settingsConfig.ts` + `modules/` 3 个子组件**原样复制**，仅改 import：`useAuth`→`@/stores/auth-store`，去 ThemeModeCtx/tokens；旧 `/settings/dashboard` 重定向路由：

```ts
// src/pages/(app)/(system)/settings/dashboard/route.tsx
export const Route = createFileRoute("/(app)/(system)/settings/dashboard/")({
	beforeLoad: () => { throw redirect({ to: "/settings", search: { tab: "dashboard" } }); },
});
```
）
- [ ] **每页完成后**：对照旧页点验 → tsc/lint → commit（`feat(dashboard): 中档迁移（users/reports/spaces/moderation/settings）`，可按页拆 commit）。

### 任务 11：功能收口验证门

**文件：** 无新文件；只验证与修补

- [ ] **步骤 1：静态门**

```bash
cd app/dashboard-v2
pnpm gen-route && pnpm typecheck && pnpm lint && pnpm vitest run && pnpm build
```

- [ ] **步骤 2：路由对照**——与旧 `router.tsx` 的 30 条路由逐一 visit（dev + 登录态），核对 URL、渲染内容、菜单归属（含 `/settings/dashboard` 重定向、4 个 detail 的 `?id=`）。URL 与旧版必须一字不差。
- [ ] **步骤 3：写路径抽查**——发公告、处理举报、禁言/解禁、改角色、建 API Key 各操作一遍真实后端（sqlite fresh 库）。
- [ ] **步骤 4：截图 gallery**——全部路由 × light/dark 截图（浮层用真实点击关闭后截），汇总为单个自包含 HTML 文件 `documents/dashboard-v2-gallery.html`（不入 git，供审查）。
- [ ] **步骤 5：修补发现的回归，重新跑步骤 1。**

### 任务 12：切换（删旧改名 + 脚本 + Docker/nginx）

**前置检查（硬性）：** `cd /Users/noelorin/group/Feed Chamber && git status --short app/dashboard` 确认另一会话无未提交改动；若有，**停下来问用户**。

- [ ] **步骤 1：** `git rm -r app/dashboard`（目录内容此刻应与 git 一致），`app/dashboard-v2` 改目录名为 `app/dashboard`，`package.json` name 改回 `@feedchamber/dashboard`；修正 `pnpm-workspace.yaml` 与 root `package.json` 的 `dev:dashboard`/`build` 相关脚本指向。
- [ ] **步骤 2：** 对照旧包的 `vite.config.ts` 端口/代理与 root 脚本，把新 vite 配置对齐（`pnpm dev:dashboard` 可用）。Docker/nginx：读旧包 `docker/` 与 root `Dockerfile` 中 dashboard 相关段，按新包产物路径（vite 产 `dist/`）改写——Docker 守护进程不可用，只改配置不构建验证，在 commit 信息注明。
- [ ] **步骤 3：** 全量验证门重跑 + `pnpm dev:dashboard` 起动确认。
- [ ] **步骤 4：** Commit `refactor(dashboard)!: 删除旧 dashboard，dashboard-v2 转正`；随后 `git push -u origin rebuild/dashboard-template` **前先问用户**。

## 自检记录

- 规格覆盖：§3 架构→任务 0-2/5；§4 认证→任务 2/3；§5 数据层→任务 5-8 基建+各页；§6 视觉→任务 4；§7 迁移映射→任务 6-10（30 条路由在任务 11 步骤 2 逐条核对）；§8 验收→任务 11/12；§9 风险→任务 12 前置检查 + 任务 7 降级项记录要求。无遗漏。
- 占位符：设计值（任务 4）与每页列定义来自真实源文件提取，非占位；任务 4 步骤 2 的 `#…` 注释是「提取来源指引」而非未完成项。
- 类型一致性：`auth-store` 的 `useAuth`/`AuthUser` 与旧包同名同形；`ProDescItem` 在任务 5 定义、任务 8 posts/comments/detail 消费；query key 约定 `[resource, params]` 全文一致。

## 执行记录（2026-10-18 收口）

分支 `rebuild/dashboard-template` 的成果已合并进 `migrate/react-vinext`（merge `66f5f58`，切换提交 `ccc4f10`）。

### 本轮补的内容

- **回填分支外改动**：10-15 之后 `migrate/react-vinext` 上旧 dashboard 还有 5 处改动，已移植到新包——
  `SpaceIcon`（版面图标 emoji → SVG）与版面表单/提示去 emoji、settings 的 `jwt_*` 键换成
  `oauth_access_token_ttl` / `sso_id_token_ttl`、截图用 `vite.config.capture.ts`（8001 → 后端 8090）。
- **任务 12 切换**：删除旧 `app/dashboard`、`app/dashboard-v2` 转正、包名 `@feedchamber/dashboard`
  （旧名 `@feedchamber/dashboard-vite` 与 root 的 `dev:dashboard` filter 对不上，脚本此前一直是坏的）；
  dev 端口对齐旧包 8000；清掉模板遗留（`Dockerfile` / `nginx.conf` / `docker/` / `.env.example` /
  `public/runtime-config.json` 与没人引用的 `components/config/{env,permissionMap}.ts`）；
  裁掉未使用依赖（styled-components / zod / axios / eventsource / sonner）；
  更新 root `AGENTS.md` 后台路由表、新增 `app/dashboard/AGENTS.md`。
- **默认值对齐旧 dashboard**：QueryClient `staleTime 60s / retry 1 / refetchOnMount` 默认 true
  （模板原值 10s / retry 3 / refetchOnMount:false 会让已缓存页面不再自动刷新）；
  settingStore 主题默认 `system`、侧栏默认展开（旧 ProShell 行为）。

### 收口验证发现并修掉的问题

1. **菜单分组 key 撞车（阻断级）**：`__root.tsx` 交给菜单的是 `router.routeTree`，pathless 分组
   （`_forum` 等）的 `fullPath` 就是父级 `/`，4 个分组 key 全变成 `/` —— antd 报 Duplicated key，
   只剩一个分组能展开、顺序也乱。修法：分组 key 取路由 id 最后一段（`/_forum`…），
   并在 `routeStore.rebuildMenu` 补 `sortMenuItems`（此前只有测试走了排序）。新增运行时形状回归测试。
2. **设置页 `<form>` 嵌套**：`/settings?tab=dashboard` 外层 Form 包住自带 Form 的三个模块，
   React 19 报 hydration 告警；外层 Form 改 `component={false}`。
3. **新建 API Key 必 400**：后端要求 scopes 非空且合法，后台表单默认空串；默认值给 `read` + 必填提示。
4. **详情页 Descriptions 列宽不匹配**：帖子弹窗（`更新时间`）与举报详情（`目标 ID`）补 `span`。
5. **禁言列表用户名/邮箱/禁言时间三列全空**：`/admin/banned/list` 直接回 `model.Banned`，
   而 api-client 的 `BannedUser` 与页面列按 `username/email/banned_at/banned_by` 取值。
   handler 侧补 `enrichBanned` 投影（`/user/{id}/banned/overview` 同样处理）。
6. 其余：`Button.Group → Space.Compact`、`Space direction → orientation`（antd 6 弃用告警）。

### 验证证据

- **门禁**：`gen-route` / `typecheck` / `lint` / `test`（11 文件 41 用例，含 2 条新回归）/ `build` 全绿；
  `go build ./...` 通过。
- **路由巡检**（production build + 真实后端 + fresh sqlite）：29 条路由 URL 全部正确；
  未登录访问 `/users` → `/login?redirect=%2Fusers`；`/settings/dashboard` → `/settings?tab=dashboard`；
  0 页面异常、0 个 4xx/5xx 请求；控制台只剩 antd 开发期告警。
- **写路径**（UI 点击后回查服务端状态）：建 API Key、建并发布公告、禁言、解禁、改角色、处理举报、
  保存设置 —— 全部通过。
- **截图 gallery**：`documents/dashboard-v2-gallery.html`（浅色 29 张 + 深色 9 张，自包含，不入 git）。

### 仍未处理（已知项，非本轮回归）

- antd 开发期告警 `useForm is not connected to any Form element`（约 8 个页面在 Modal 未挂载时先
  `setFieldsValue`），行为无影响。
- `antd/dist/antd.css` 整包引入导致产物 CSS 933KB（gzip 107KB）；要瘦身需改 CSS 变量按需，不影响功能。
- 按钮级权限码未铺（计划内：`utils/permission` 机制保留、`permList` 置空）。
