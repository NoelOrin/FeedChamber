import { dehydrate } from '@tanstack/react-query';

import { NotificationsPage } from '~/components/user/notifications-page';
import { notificationsOptions, toFirstValue, toPage, toReadFilter } from '~/components/user/queries';
import { requireAuth } from '~/lib/auth/guards';
import { getQueryClient } from '~/lib/query/client';

interface RouteProps {
	searchParams: Promise<Record<string, string | string[] | undefined>>;
}

/** `/user/notifications` 首屏：按 URL 上的 ?type= &read= &page= 预取那一页通知。 */
export default async function UserNotificationsRoute({ searchParams }: RouteProps) {
	await requireAuth('/user/notifications');
	const params = await searchParams;

	const filter = {
		page: toPage(params.page),
		type: toFirstValue(params.type, 'all'),
		read: toReadFilter(params.read),
	};

	const queryClient = getQueryClient();
	await queryClient.prefetchQuery(notificationsOptions(filter));

	return <HydrationBoundary state={dehydrate(queryClient)}>{<NotificationsPage />}</HydrationBoundary>;
}

import { HydrationBoundary } from '~/components/query-hydration';
