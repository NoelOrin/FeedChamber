'use client';

import { type Notification, notificationService } from '@orinforum/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';
import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';

import { Button } from '~/components/ui/button';
import { EmptyState } from '~/components/ui/empty-state';
import { Loading } from '~/components/ui/loading';
import { Sheet, SheetContent, SheetTitle } from '~/components/ui/sheet';
import { useAuthStore } from '~/lib/auth';
import { useNotificationSSE } from '~/lib/hooks/use-notification-sse';
import { Bell, CheckCheck, X } from '~/lib/icons';
import { DEFAULT_PAGE_SIZE } from '~/lib/pagination';
import { qk } from '~/lib/query/keys';
import { cn } from '~/lib/utils';

const listKey = qk.notifications.list({ page: 1, page_size: DEFAULT_PAGE_SIZE });
const NO_ITEMS: Notification[] = [];

/** 通知深链跳转（旧 utils/openNotificationTarget.ts 的等价实现，router.push 版）。 */
function openNotificationTarget(n: Notification, push: (href: string) => void) {
	if (!n.reference_id) return;
	switch (n.type) {
		case 'comment':
		case 'replied':
		case 'like':
		case 'liked':
			push(`/timeline/post?id=${encodeURIComponent(n.reference_id)}`);
			return;
		case 'follow':
		case 'followed':
			push(`/user/${encodeURIComponent(n.reference_id)}`);
			return;
		case 'pm':
			push(`/messages/${encodeURIComponent(n.reference_id)}`);
			return;
	}
}

export function NotificationBell() {
	const router = useRouter();
	const queryClient = useQueryClient();
	const loggedIn = useAuthStore((state) => state.status === 'authenticated' && state.user !== null);
	const { unread, start } = useNotificationSSE();
	const [open, setOpen] = useState(false);

	// 只在登录态订阅。此前挂载即 start()：公开内容页对匿名访客开放后，/notifications/stream
	// 是受保护端点，匿名访客会拿到 401 并触发 SSEClient 的指数退避重连（最多 5 次），
	// 每个匿名页面浏览都变成一串失败请求。服务端 window 未定义时 start 仍是 no-op。
	useEffect(() => {
		if (!loggedIn) return;
		start();
	}, [loggedIn, start]);

	const { data, isPending } = useQuery({
		queryKey: listKey,
		queryFn: () => notificationService.list({ page: 1, page_size: DEFAULT_PAGE_SIZE }),
		// 旧实现是「每次打开抽屉拉取最新列表」，故不留缓存时间
		enabled: open,
		staleTime: 0,
		refetchOnMount: 'always',
	});
	const items = data?.items ?? NO_ITEMS;

	const markOne = useMutation({ mutationFn: (id: string) => notificationService.read(id) });
	const markAll = useMutation({
		mutationFn: () => notificationService.readAll(),
		onMutate: () => {
			queryClient.setQueryData<{ items: Notification[]; total: number } | undefined>(listKey, (old) =>
				old ? { ...old, items: old.items.map((i) => ({ ...i, is_read: true })) } : old,
			);
		},
	});

	function patchItem(id: string) {
		queryClient.setQueryData<{ items: Notification[]; total: number } | undefined>(listKey, (old) =>
			old ? { ...old, items: old.items.map((i) => (i.id === id ? { ...i, is_read: true } : i)) } : old,
		);
	}

	async function markRead(n: Notification) {
		if (!n.is_read) {
			patchItem(n.id);
			// silent + 不回滚：与旧实现一致
			await markOne.mutateAsync(n.id).catch(() => {});
		}
		setOpen(false);
		openNotificationTarget(n, (href) => router.push(href));
	}

	function markAllRead() {
		markAll.mutate();
	}

	return (
		<>
			<Button
				variant="ghost"
				size="icon"
				className="relative text-muted-foreground"
				title="通知"
				aria-label={unread > 0 ? `通知（${unread} 条未读）` : '通知'}
				onClick={() => setOpen(true)}
			>
				<Bell size={16} aria-hidden="true" />
				{unread > 0 ? (
					<span className="absolute top-1.5 right-1.5 size-1.5 rounded-full bg-primary ring-2 ring-background" />
				) : null}
			</Button>

			<Sheet open={open} onOpenChange={setOpen}>
				<SheetContent side="right" showClose={false} aria-describedby={undefined} className="w-96 gap-0 p-0">
					<div className="flex h-14 shrink-0 items-center justify-between border-b border-border px-4">
						<SheetTitle>通知</SheetTitle>
						<Button variant="ghost" size="icon" aria-label="关闭" onClick={() => setOpen(false)}>
							<X className="size-4" aria-hidden="true" />
						</Button>
					</div>

					<div className="flex shrink-0 items-center justify-between border-b border-border px-4 py-2">
						<span className="text-xs text-muted-foreground">{unread > 0 ? `${unread} 条未读` : '暂无未读'}</span>
						<Button variant="ghost" size="xs" className="gap-1" disabled={items.length === 0} onClick={markAllRead}>
							<CheckCheck size={14} aria-hidden="true" />
							全部已读
						</Button>
					</div>

					<div className="min-h-0 flex-1 overflow-y-auto">
						{isPending ? (
							<Loading />
						) : items.length > 0 ? (
							<ul className="divide-y divide-border/60">
								{items.map((n) => (
									<li key={n.id}>
										<button
											type="button"
											className="flex w-full cursor-pointer items-start gap-2.5 px-4 py-3 text-left transition-colors hover:bg-muted"
											onClick={() => void markRead(n)}
										>
											<span
												aria-hidden="true"
												className={cn(
													'mt-1.5 size-1.5 shrink-0 rounded-full',
													n.is_read ? 'bg-transparent' : 'bg-primary',
												)}
											/>
											<span className="min-w-0 flex-1">
												<span
													className={cn(
														'block text-[13px] wrap-break-word',
														n.is_read ? 'text-muted-foreground' : 'font-medium text-foreground',
													)}
												>
													{n.content}
												</span>
												<span className="mt-0.5 block text-[11px] text-muted-foreground">
													{dayjs(n.created_at).format('YYYY-MM-DD HH:mm')}
												</span>
											</span>
										</button>
									</li>
								))}
							</ul>
						) : (
							<EmptyState className="m-4" title="暂无通知" />
						)}
					</div>
				</SheetContent>
			</Sheet>
		</>
	);
}
