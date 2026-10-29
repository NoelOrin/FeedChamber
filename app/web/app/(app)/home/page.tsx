import { dehydrate } from '@tanstack/react-query';

import { HomeFeed } from '~/components/home/home-feed';
import { dashboardHomeOptions } from '~/components/user/queries';
import { getQueryClient } from '~/lib/query/client';

/* `/home` 在旧 Solid 树里只有 `home/route.tsx` 的空 Outlet（聚合数据挂在 /user 的 loader 上），
   迁移后把 dashboard 归位到本页。 */
export default async function HomePage() {
	const queryClient = getQueryClient();
	await queryClient.prefetchQuery(dashboardHomeOptions());

	return (
		<HydrationBoundary state={dehydrate(queryClient)}>
			<HomeFeed />
		</HydrationBoundary>
	);
}

import { HydrationBoundary } from '~/components/query-hydration';
