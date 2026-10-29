import { dehydrate } from '@tanstack/react-query';
import { FavoritesPage } from '~/components/user/favorites-page';
import { favoriteIdsOptions, favoritePostsOptions, toPage } from '~/components/user/queries';
import { requireAuth } from '~/lib/auth/guards';
import { getQueryClient } from '~/lib/query/client';

interface RouteProps {
	searchParams: Promise<Record<string, string | string[] | undefined>>;
}

/** `/user/favorites` 首屏：先预取收藏 id 页，再用 id 批量预取帖子（与旧 loader 两跳一致）。 */
export default async function UserFavoritesRoute({ searchParams }: RouteProps) {
	await requireAuth('/user/favorites');
	const params = await searchParams;
	const page = toPage(params.page);

	const queryClient = getQueryClient();
	await queryClient.prefetchQuery(favoriteIdsOptions(page));
	const cached = queryClient.getQueryData<{ data: string[]; total: number }>(favoriteIdsOptions(page).queryKey);
	const ids = cached?.data ?? [];
	if (ids.length > 0) await queryClient.prefetchQuery(favoritePostsOptions(ids));

	return <HydrationBoundary state={dehydrate(queryClient)}>{<FavoritesPage />}</HydrationBoundary>;
}

import { HydrationBoundary } from '~/components/query-hydration';
