import { dehydrate } from '@tanstack/react-query';

import { FollowPage } from '~/components/user/follow-page';
import {
	type FollowTab,
	followingIdsOptions,
	followListOptions,
	toFirstValue,
	toPage,
	userSearchOptions,
} from '~/components/user/queries';
import { requireAuth } from '~/lib/auth/guards';
import { getQueryClient } from '~/lib/query/client';

interface RouteProps {
	searchParams: Promise<Record<string, string | string[] | undefined>>;
}

/** `/user/follow` 首屏：旧 loader 先取关注状态、再按 tab / q 取列表，这里同样两跳预取。 */
export default async function UserFollowRoute({ searchParams }: RouteProps) {
	const user = await requireAuth('/user/follow');
	const params = await searchParams;

	const tab: FollowTab = toFirstValue(params.tab, 'following') === 'followers' ? 'followers' : 'following';
	const q = toFirstValue(params.q, '');
	const page = toPage(params.page);

	const queryClient = getQueryClient();
	await Promise.all([
		queryClient.prefetchQuery(followingIdsOptions(user.id)),
		q.length > 0
			? queryClient.prefetchQuery(userSearchOptions(q, page))
			: queryClient.prefetchQuery(followListOptions(user.id, tab, page)),
	]);

	return <HydrationBoundary state={dehydrate(queryClient)}>{<FollowPage userId={user.id} />}</HydrationBoundary>;
}

import { HydrationBoundary } from '~/components/query-hydration';
