'use client';

import type { ConversationListItem } from '@orinforum/api-client';
import { useQuery } from '@tanstack/react-query';
import dayjs from 'dayjs';
import Link from 'next/link';
import { useRouter } from 'next/navigation';

import { conversationsOptions } from '~/components/pm/queries';
import { Avatar, AvatarFallback, AvatarImage } from '~/components/ui/avatar';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { EmptyState } from '~/components/ui/empty-state';
import { PageHeader } from '~/components/ui/page-header';
import { Skeleton } from '~/components/ui/skeleton';

function ConversationRow({ item }: { item: ConversationListItem }) {
	const router = useRouter();
	return (
		<button
			type="button"
			className="group flex w-full cursor-pointer items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-muted/50"
			onClick={() => router.push(`/messages/${encodeURIComponent(item.id)}`)}
		>
			<Avatar className="size-8">
				{item.peer_avatar ? <AvatarImage src={item.peer_avatar} alt="" /> : null}
				<AvatarFallback className="text-[11px]">
					{(item.peer_name || item.peer_id || '?').slice(0, 1).toUpperCase()}
				</AvatarFallback>
			</Avatar>
			<div className="min-w-0 flex-1">
				<div className="flex items-center gap-2">
					<span className="truncate text-[13px] font-medium text-foreground">{item.peer_name || item.peer_id}</span>
					{item.unread_count > 0 ? (
						<Badge variant="primary" className="min-w-4 px-1 tabular-nums">
							{item.unread_count}
						</Badge>
					) : null}
				</div>
				<div className="mt-0.5 truncate text-xs text-muted-foreground">{item.last_message || '暂无消息'}</div>
			</div>
			<span className="shrink-0 text-[11px] text-muted-foreground tabular-nums">
				{item.last_message_at ? dayjs(item.last_message_at).format('MM-DD HH:mm') : ''}
			</span>
		</button>
	);
}

export function ConversationList() {
	const { data, isPending } = useQuery(conversationsOptions());
	const items = data?.data ?? [];
	const total = data?.total ?? 0;

	return (
		<div className="flex w-full flex-col gap-3 pb-10">
			<PageHeader title="私信" meta={`${total} 个会话`} />

			{isPending ? (
				<Card className="gap-0 divide-y divide-border/60 p-0">
					{(['first', 'second', 'third'] as const).map((row) => (
						<div key={`pm-skeleton-${row}`} className="flex items-center gap-3 px-4 py-3">
							<Skeleton className="size-8 rounded-full" />
							<div className="flex-1 space-y-1.5">
								<Skeleton className="h-3.5 w-28" />
								<Skeleton className="h-3 w-52" />
							</div>
							<Skeleton className="h-3 w-16" />
						</div>
					))}
				</Card>
			) : items.length === 0 ? (
				<EmptyState title="暂无私信" description="在用户主页点「私信」即可开启会话。">
					<Button asChild variant="outline" size="sm">
						<Link href="/timeline">返回时间线</Link>
					</Button>
				</EmptyState>
			) : (
				<Card className="gap-0 divide-y divide-border/60 overflow-hidden p-0">
					{items.map((item) => (
						<ConversationRow key={item.id} item={item} />
					))}
				</Card>
			)}
		</div>
	);
}
