import { dehydrate } from '@tanstack/react-query';

import { MessageThread } from '~/components/pm/message-thread';
import { conversationsOptions, messagesOptions } from '~/components/pm/queries';
import { requireAuth } from '~/lib/auth/guards';
import { getQueryClient } from '~/lib/query/client';

/* 旧 messages/$id loader：消息流 + 会话列表（反查对端）并行预取。
   markRead 是写操作，不在渲染期发起，交给 MessageThread 挂载后的 mutation。
   私信是结构性私有面，(app)/layout 不再兜底，本页自己守。 */
export default async function MessageThreadPage({ params }: { params: Promise<{ id: string }> }) {
	const { id } = await params;
	await requireAuth(`/messages/${encodeURIComponent(id)}`);
	const queryClient = getQueryClient();

	await Promise.all([
		queryClient.prefetchQuery(messagesOptions(id)),
		queryClient.prefetchQuery(conversationsOptions({ page: 1, page_size: 100 })),
	]);

	return (
		<HydrationBoundary state={dehydrate(queryClient)}>
			<MessageThread conversationId={id} />
		</HydrationBoundary>
	);
}

import { HydrationBoundary } from '~/components/query-hydration';
