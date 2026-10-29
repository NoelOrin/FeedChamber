import type * as React from 'react';

import { AppShell } from '~/components/layout/app-shell';
import { loadPublicSettings } from '~/lib/queries/server';

/**
 * 公开内容面的外壳。这里**刻意不再调用 requireAuth()**：
 *
 * 一个页面要不要登录是服务端设置（post_auth_required / space_view_auth_required /
 * 各页面自己的服务端设置）决定的，不是前端决定的。此前本布局无条件 requireAuth，
 * 与后端 AuthGate 的默认放行语义相反，导致匿名访客连帖子详情都被 307 到 /login，
 * 爬虫一条都抓不到（openspec change 2026-10-28-seo-public-indexing）。
 *
 * 真正的私有面（/messages、/timeline/create、/user 自身主页、/user/{notifications,…}）
 * 各自在页面内 requireAuth，见 proxy.ts 的 PROTECTED 注释。
 */
export default async function AppLayout({ children }: { children: React.ReactNode }) {
	const settings = await loadPublicSettings();

	return <AppShell settings={settings}>{children}</AppShell>;
}
