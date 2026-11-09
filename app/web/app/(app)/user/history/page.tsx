import { dehydrate } from '@tanstack/react-query';

import { HistoryPage } from '~/components/history/history-page';
import { historyListOptions } from '~/components/history/queries';
import { HydrationBoundary } from '~/components/query-hydration';
import '~/lib/api/adapter';
import { requireAuth } from '~/lib/auth/guards';
import { DEFAULT_PAGE_SIZE, toPage } from '~/lib/pagination';
import { getQueryClient } from '~/lib/query/client';

interface RouteProps {
	searchParams: Promise<Record<string, string | string[] | undefined>>;
}

/**
 * `/user/history` 首屏：预取第一页浏览记录。
 *
 * 结构性私有面（与 favorites / follow / notifications 同批），由页面内 requireAuth 守；
 * `proxy.ts` 只挡 /messages /home /timeline/create 与精确的 /user，不参与这里。
 */
export default async function UserHistoryRoute({ searchParams }: RouteProps) {
	await requireAuth('/user/history');
	const params = await searchParams;
	const page = toPage(params.page);

	const queryClient = getQueryClient();
	await queryClient.prefetchQuery(historyListOptions(page, DEFAULT_PAGE_SIZE));

	return <HydrationBoundary state={dehydrate(queryClient)}>{<HistoryPage />}</HydrationBoundary>;
}
