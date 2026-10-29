import { dehydrate } from '@tanstack/react-query';
import { MyProfilePage } from '~/components/user/my-profile-page';
import {
	checkinHistoryOptions,
	checkinStatusOptions,
	dashboardHomeOptions,
	publicProfileOptions,
	userActivityOptions,
	userBadgesOptions,
	userPostsOptions,
} from '~/components/user/queries';
import { requireAuth } from '~/lib/auth/guards';
import { getQueryClient } from '~/lib/query/client';

/**
 * `/user` 我的主页：旧 route loader 的 `dashboardService.home()` 换成 RSC 预取 +
 * HydrationBoundary，客户端组件再用同一批 queryOptions 命中缓存（首屏不再空一遍）。
 */
export default async function UserProfilePage() {
	const user = await requireAuth('/user');
	const queryClient = getQueryClient();
	const now = new Date();
	const year = now.getFullYear();
	const month = now.getMonth() + 1;

	await Promise.all([
		queryClient.prefetchQuery(dashboardHomeOptions()),
		queryClient.prefetchQuery(userPostsOptions(user.id, 1)),
		queryClient.prefetchQuery(publicProfileOptions(user.id)),
		queryClient.prefetchQuery(userBadgesOptions(user.id)),
		queryClient.prefetchQuery(userActivityOptions(user.id, year)),
		queryClient.prefetchQuery(checkinStatusOptions()),
		queryClient.prefetchQuery(checkinHistoryOptions(year, month)),
	]);

	return <HydrationBoundary state={dehydrate(queryClient)}>{<MyProfilePage initialUser={user} />}</HydrationBoundary>;
}

import { HydrationBoundary } from '~/components/query-hydration';
