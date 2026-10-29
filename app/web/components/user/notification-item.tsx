'use client';

import type { Notification } from '@orinforum/api-client';
import dayjs from 'dayjs';

import { Button } from '~/components/ui/button';
import { Trash2 } from '~/lib/icons';
import { cn } from '~/lib/utils';

interface NotificationItemProps {
	notification: Notification;
	selectable: boolean;
	selected: boolean;
	onMarkRead: () => void;
	onDelete: () => void;
	onToggleSelect: () => void;
}

/** 通知条目（由 NotificationList 的 Card + divide-y 承载外壳）。 */
export function NotificationItem({
	notification,
	selectable,
	selected,
	onMarkRead,
	onDelete,
	onToggleSelect,
}: NotificationItemProps) {
	const unread = !notification.is_read;

	return (
		<li className="group flex list-none items-start gap-3 px-4 py-3 transition-colors hover:bg-muted/50">
			{selectable ? (
				<input
					type="checkbox"
					aria-label="选择该通知"
					checked={selected}
					onChange={onToggleSelect}
					className="mt-1.5 size-3.5 shrink-0 cursor-pointer accent-primary"
				/>
			) : null}

			<span
				aria-hidden="true"
				className={cn('mt-1.5 size-1.5 shrink-0 rounded-full', unread ? 'bg-primary' : 'bg-transparent')}
			/>

			<button
				type="button"
				onClick={onMarkRead}
				className="min-w-0 flex-1 cursor-pointer text-left"
				title={unread ? '点击标记已读并打开' : '打开'}
			>
				<p
					className={cn(
						'text-[13px] wrap-break-word',
						unread ? 'font-medium text-foreground' : 'text-muted-foreground',
					)}
				>
					{notification.content}
				</p>
				<span className="mt-0.5 block text-[11px] text-muted-foreground tabular-nums">
					{dayjs(notification.created_at).format('YYYY-MM-DD HH:mm')}
				</span>
			</button>

			<Button
				size="icon"
				variant="ghost"
				onClick={onDelete}
				title="删除"
				aria-label="删除通知"
				className="size-7 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100 hover:text-destructive focus-visible:opacity-100"
			>
				<Trash2 className="size-3.5" aria-hidden="true" />
			</Button>
		</li>
	);
}
