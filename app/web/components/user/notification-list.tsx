import type { Notification } from '@orinforum/api-client';

import { Pager } from '~/components/pagination/pager';
import { EmptyState } from '~/components/ui/empty-state';
import { NotificationItem } from '~/components/user/notification-item';

interface NotificationListProps {
	items: Notification[];
	batchMode: boolean;
	selectedIds: Set<string>;
	onToggleSelect: (id: string) => void;
	onMarkRead: (n: Notification) => void;
	onDelete: (n: Notification) => void;
	currentPage: number;
	totalPages: number;
	onPageChange: (page: number) => void;
}

/** 通知列表（旧 -modules/NotificationList.tsx）：条目 + Pager。 */
export function NotificationList({
	items,
	batchMode,
	selectedIds,
	onToggleSelect,
	onMarkRead,
	onDelete,
	currentPage,
	totalPages,
	onPageChange,
}: NotificationListProps) {
	if (items.length === 0) {
		return <EmptyState title="暂无通知" description="没有新消息" />;
	}

	return (
		<>
			<ul className="m-0 list-none divide-y divide-border/60 overflow-hidden rounded-lg border border-border bg-card p-0">
				{items.map((n) => (
					<NotificationItem
						key={n.id}
						notification={n}
						selectable={batchMode}
						selected={selectedIds.has(n.id)}
						onMarkRead={() => onMarkRead(n)}
						onDelete={() => onDelete(n)}
						onToggleSelect={() => onToggleSelect(n.id)}
					/>
				))}
			</ul>
			<Pager currentPage={currentPage} totalPages={totalPages} onPageChange={onPageChange} />
		</>
	);
}
