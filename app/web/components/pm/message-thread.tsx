'use client';

import type { Message } from '@feedchamber/api-client';
import { pmService } from '@feedchamber/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';
import Link from 'next/link';
import { useEffect, useState } from 'react';

import { conversationsOptions, messagesOptions } from '~/components/pm/queries';
import { Avatar, AvatarFallback, AvatarImage } from '~/components/ui/avatar';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { EmptyState } from '~/components/ui/empty-state';
import { Input } from '~/components/ui/input';
import { Skeleton } from '~/components/ui/skeleton';
import { useAuthStore } from '~/lib/auth';
import { ArrowLeft, MessageSquare, Send } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';
import { cn } from '~/lib/utils';

type MessageFeed = { data: Message[]; total: number };

function Bubble({ message, mine }: { message: Message; mine: boolean }) {
	return (
		<div className={cn('flex', mine ? 'justify-end' : 'justify-start')}>
			<div
				className={cn(
					'max-w-[72%] rounded-lg px-3 py-2 text-sm leading-relaxed',
					mine ? 'bg-primary text-primary-foreground' : 'border border-border bg-card text-foreground',
				)}
			>
				<div>{message.body}</div>
				<div className={cn('mt-1 text-[11px]', mine ? 'text-primary-foreground/70' : 'text-muted-foreground')}>
					{dayjs(message.created_at).format('MM-DD HH:mm')}
				</div>
			</div>
		</div>
	);
}

export function MessageThread({ conversationId }: { conversationId: string }) {
	const queryClient = useQueryClient();
	const me = useAuthStore((state) => state.user)?.id;
	const [body, setBody] = useState('');

	const messages = useQuery(messagesOptions(conversationId));
	/* 旧 $id loader 从会话列表（100 条）反查对端昵称/头像，这里复用同一份缓存 */
	const peers = useQuery(conversationsOptions({ page: 1, page_size: 100 }));
	const peer = peers.data?.data.find((item) => item.id === conversationId);

	/* 已读标记：旧版在 loader 里 fire-and-forget，平移为挂载时 mutation，并刷新会话列表未读数 */
	const { mutate: markRead } = useMutation({
		mutationFn: (id: string) => pmService.markRead(id),
		onSuccess: () => {
			void queryClient.invalidateQueries({ queryKey: qk.pm.conversations() });
		},
	});
	useEffect(() => {
		markRead(conversationId);
	}, [conversationId, markRead]);

	const send = useMutation({
		mutationFn: (text: string) => pmService.send(conversationId, { body: text }),
		onSuccess: (msg) => {
			queryClient.setQueryData<MessageFeed>(messagesOptions(conversationId).queryKey, (old) =>
				old ? { ...old, data: [...old.data, msg], total: old.total + 1 } : old,
			);
			setBody('');
			void queryClient.invalidateQueries({ queryKey: qk.pm.conversations() });
		},
		onError: (err) => toast.error((err as Error).message || '发送失败'),
	});

	const submit = () => {
		const text = body.trim();
		if (!text || send.isPending) return;
		send.mutate(text);
	};

	const items = messages.data?.data ?? [];

	return (
		<div className="flex w-full flex-col gap-3 pb-10">
			<Card className="flex-row items-center gap-3 p-3">
				<Button asChild variant="ghost" size="icon" aria-label="返回">
					<Link href="/messages">
						<ArrowLeft className="size-4" aria-hidden="true" />
					</Link>
				</Button>
				<Avatar className="size-8">
					{peer?.peer_avatar ? <AvatarImage src={peer.peer_avatar} alt="" /> : null}
					<AvatarFallback className="text-[11px]">
						{(peer?.peer_name || peer?.peer_id || '?').slice(0, 1).toUpperCase()}
					</AvatarFallback>
				</Avatar>
				<div className="min-w-0 flex-1">
					<div className="truncate text-[13px] font-medium text-foreground">
						{peer?.peer_name || peer?.peer_id || '会话'}
					</div>
					<div className="truncate text-[11px] text-muted-foreground">与 {peer?.peer_name || '对方'} 的对话</div>
				</div>
			</Card>

			<Card className="gap-0 overflow-hidden p-0">
				<div className="flex-1 space-y-3 overflow-y-auto p-4" style={{ maxHeight: '56vh' }}>
					{messages.isPending ? (
						<div className="space-y-3">
							<Skeleton className="h-12 w-1/2" />
							<Skeleton className="ml-auto h-12 w-2/5" />
							<Skeleton className="h-16 w-3/5" />
						</div>
					) : items.length === 0 ? (
						<EmptyState
							icon={<MessageSquare className="size-5" aria-hidden="true" />}
							title="暂无消息"
							description="还没有消息，打个招呼吧"
							className="border-0 py-12"
						/>
					) : (
						items.map((message) => <Bubble key={message.id} message={message} mine={message.sender_id === me} />)
					)}
				</div>
				<div className="flex gap-2 border-t border-border p-3">
					<Input
						value={body}
						onChange={(event) => setBody(event.target.value)}
						onKeyDown={(event) => {
							if (event.key === 'Enter' && !event.shiftKey) {
								event.preventDefault();
								submit();
							}
						}}
						placeholder="输入消息，回车发送…"
						aria-label="消息内容"
					/>
					<Button size="sm" className="shrink-0 gap-1.5" disabled={!body.trim() || send.isPending} onClick={submit}>
						<Send className="size-3.5" aria-hidden="true" /> 发送
					</Button>
				</div>
			</Card>
		</div>
	);
}
