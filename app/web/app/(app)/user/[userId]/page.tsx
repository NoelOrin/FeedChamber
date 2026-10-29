import { dehydrate } from '@tanstack/react-query';
import type { Metadata } from 'next';

import {
	followCountsOptions,
	publicProfileOptions,
	userActivityOptions,
	userBadgesOptions,
	userPostsOptions,
} from '~/components/user/queries';
import { VisitorProfilePage } from '~/components/user/visitor-profile-page';
import { requireAuth } from '~/lib/auth/guards';
import { loadPublicSettings } from '~/lib/queries/server';
import { getQueryClient } from '~/lib/query/client';

interface RouteProps {
	params: Promise<{ userId: string }>;
}

/**
 * `/user/[userId]` 他人主页。
 * 与同级的 `posts/ notifications/ follow/ favorites/` 共存：App Router 的静态段优先级高于
 * 动态段，所以 `/user/posts` 永远命中 `posts/page.tsx` 的 redirect，不会被这里吞掉。
 * 取数失败（含用户不存在）不抛错：prefetchQuery 吞异常，客户端按空档案渲染 EmptyState。
 *
 * 可见性与索引都由服务端设置决定（openspec change 2026-10-28-seo-public-indexing）：
 * 他人主页是公开内容（P4 清理了后端从未读取的 user_profile_auth_required 开关；
 * 是否进索引由 user_profile_indexable 控制）。
 *   - user_profile_indexable（默认 false）→ 输出 noindex
 * 他人主页是 UGC 薄内容，默认不索引；运营要收录时在后台打开开关。
 */

async function readProfileFlags(): Promise<{ authRequired: boolean; indexable: boolean }> {
	const settings = await loadPublicSettings();
	return {
		authRequired: false,
		indexable: settings.user_profile_indexable === 'true',
	};
}

export async function generateMetadata({ params }: RouteProps): Promise<Metadata> {
	const { userId } = await params;
	const { indexable } = await readProfileFlags();
	const settings = await loadPublicSettings();
	const siteName = settings.site_name || 'Feed Chamber';
	const base = (settings.site_url || '').replace(/\/$/, '');

	return {
		title: `用户主页 - ${siteName}`,
		// 索引开关关闭时同时禁止跟进链接，避免爬虫把 UGC 页面继续扩散出去。
		robots: indexable ? undefined : { index: false, follow: false },
		// 开启索引时输出自指 canonical。base 为空时退回相对路径——不能让「没配 site_url」
		// 把 canonical 吞掉，那样开启索引后这个页面反而没有规范地址了。
		...(indexable ? { alternates: { canonical: `${base}/user/${encodeURIComponent(userId)}` } } : {}),
	};
}

export default async function UserDetailRoute({ params }: RouteProps) {
	const { userId } = await params;
	const { authRequired } = await readProfileFlags();

	const me = authRequired ? await requireAuth(`/user/${encodeURIComponent(userId)}`) : null;
	const queryClient = getQueryClient();
	const year = new Date().getFullYear();

	await Promise.all([
		queryClient.prefetchQuery(publicProfileOptions(userId)),
		queryClient.prefetchQuery(userPostsOptions(userId, 1)),
		queryClient.prefetchQuery(followCountsOptions(userId)),
		queryClient.prefetchQuery(userBadgesOptions(userId)),
		queryClient.prefetchQuery(userActivityOptions(userId, year)),
	]);

	return (
		<HydrationBoundary state={dehydrate(queryClient)}>
			<VisitorProfilePage userId={userId} currentUserId={me?.id ?? ''} />
		</HydrationBoundary>
	);
}

import { HydrationBoundary } from '~/components/query-hydration';
