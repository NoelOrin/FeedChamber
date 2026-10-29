import { dehydrate } from '@tanstack/react-query';

import { ConversationList } from '~/components/pm/conversation-list';
import { conversationsOptions } from '~/components/pm/queries';
import { requireAuth } from '~/lib/auth/guards';
import { getQueryClient } from '~/lib/query/client';

/* 旧 messages/index loader 的等价物：首屏在 RSC 预取，客户端 useQuery 命中同一份缓存。
   (app)/layout 不再兜底 requireAuth（公开内容面共用该布局），私信页自己守。 */
export default async function MessagesPage() {
	await requireAuth('/messages');
	const queryClient = getQueryClient();
	await queryClient.prefetchQuery(conversationsOptions());

	return (
		<HydrationBoundary state={dehydrate(queryClient)}>
			<ConversationList />
		</HydrationBoundary>
	);
}

import { HydrationBoundary } from '~/components/query-hydration';
