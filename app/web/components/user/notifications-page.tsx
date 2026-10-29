'use client';

import type { Notification } from '@orinforum/api-client';
import { notificationService } from '@orinforum/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRouter } from 'next/navigation';
import { parseAsInteger, parseAsString, useQueryState } from 'nuqs';
import { useState } from 'react';

import { Button } from '~/components/ui/button';
import { PageHeader } from '~/components/ui/page-header';
import { Segmented } from '~/components/ui/segmented';
import { BatchActionBar } from '~/components/user/batch-action-bar';
import { NotificationList } from '~/components/user/notification-list';
import { notificationsOptions, toReadFilter, totalPages } from '~/components/user/queries';
import { SkeletonList } from '~/components/user/skeletons';
import { CheckCheck } from '~/lib/icons';
import { DEFAULT_PAGE_SIZE } from '~/lib/pagination';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

const TYPE_OPTIONS = [
	{ value: 'all', label: '全部' },
	{ value: 'system', label: '系统' },
	{ value: 'comment', label: '评论' },
	{ value: 'follow', label: '关注' },
	{ value: 'like', label: '点赞' },
	{ value: 'mention', label: '提及' },
	{ value: 'pm', label: '私信' },
	{ value: 'watch', label: '关注内容' },
];

const READ_OPTIONS = [
	{ value: 'all', label: '全部' },
	{ value: 'unread', label: '未读' },
	{ value: 'read', label: '已读' },
];

/** 通知深链跳转（与 components/layout/notification-bell.tsx 同一份 URL 契约）。 */
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

type ListState = { items: Notification[]; total: number };

/** 通知中心（旧 pages/(app)/user/notifications.tsx）：?type= &read= &page= 走 nuqs。 */
export function NotificationsPage() {
	const router = useRouter();
	const queryClient = useQueryClient();
	const [type, setType] = useQueryState('type', parseAsString.withDefault('all'));
	const [readParam, setReadParam] = useQueryState('read', parseAsString.withDefault('all'));
	const [page, setPage] = useQueryState('page', parseAsInteger.withDefault(1));
	const read = toReadFilter(readParam);

	const [batchMode, setBatchMode] = useState(false);
	const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set<string>());

	const options = notificationsOptions({ page, type, read });
	const listKey = options.queryKey;
	const { data, isPending } = useQuery(options);
	const items = data?.items ?? [];
	const total = data?.total ?? 0;

	/* 旧 createEffect(loaderData) 的等价：换筛选/翻页即清空多选（渲染期对齐，不留一帧旧选择） */
	const epoch = `${type}|${read}|${page}`;
	const [seenEpoch, setSeenEpoch] = useState(epoch);
	if (seenEpoch !== epoch) {
		setSeenEpoch(epoch);
		setSelectedIds(new Set<string>());
	}

	function writeList(patch: (list: ListState) => ListState) {
		queryClient.setQueryData<ListState>(listKey, (old) => (old ? patch(old) : old));
	}

	function goPage(next: number) {
		void setPage(next);
	}

	function switchType(next: string) {
		void setType(next);
		void setPage(1);
	}

	function switchRead(next: string) {
		void setReadParam(next);
		void setPage(1);
	}

	const markOne = useMutation({
		mutationFn: (id: string) => notificationService.read(id),
	});

	const markAll = useMutation({
		mutationFn: () => notificationService.readAll(),
		onSuccess: () => {
			toast.success('已全部标为已读');
			void queryClient.invalidateQueries({ queryKey: qk.notifications.all });
		},
		onError: () => {
			void queryClient.invalidateQueries({ queryKey: qk.notifications.all });
			toast.error('操作失败');
		},
	});

	const removeOne = useMutation({
		mutationFn: (id: string) => notificationService.delete(id),
		onSuccess: () => {
			toast.success('已删除');
			void queryClient.invalidateQueries({ queryKey: qk.notifications.all });
		},
		onError: () => {
			void queryClient.invalidateQueries({ queryKey: qk.notifications.all });
			toast.error('删除失败');
		},
	});

	async function markRead(n: Notification) {
		if (!n.is_read) {
			writeList((list) => ({ ...list, items: list.items.map((i) => (i.id === n.id ? { ...i, is_read: true } : i)) }));
			try {
				await markOne.mutateAsync(n.id);
			} catch {
				writeList((list) => ({
					...list,
					items: list.items.map((i) => (i.id === n.id ? { ...i, is_read: false } : i)),
				}));
				toast.error('标记已读失败');
				return;
			}
		}
		openNotificationTarget(n, (href) => router.push(href));
	}

	function deleteNotification(n: Notification) {
		writeList((list) => ({ ...list, items: list.items.filter((i) => i.id !== n.id) }));
		removeOne.mutate(n.id);
	}

	function toggleSelect(id: string) {
		setSelectedIds((prev) => {
			const next = new Set(prev);
			if (next.has(id)) next.delete(id);
			else next.add(id);
			return next;
		});
	}

	function selectAll() {
		setSelectedIds(new Set(items.map((n) => n.id)));
	}

	function deselectAll() {
		setSelectedIds(new Set<string>());
	}

	async function batchMarkRead() {
		const ids = [...selectedIds].filter((id) => items.some((n) => n.id === id && !n.is_read));
		if (ids.length === 0) {
			toast.error('没有可标记的通知');
			return;
		}
		try {
			if (ids.length === items.filter((n) => !n.is_read).length) {
				await notificationService.readAll();
			} else {
				await Promise.all(ids.map((id) => notificationService.read(id)));
			}
			writeList((list) => ({
				...list,
				items: list.items.map((i) => (ids.includes(i.id) ? { ...i, is_read: true } : i)),
			}));
			/* readAll() 会清掉所有分页的未读，本地补丁只覆盖当前这一页 */
			void queryClient.invalidateQueries({ queryKey: qk.notifications.all });
			toast.success(`已标记 ${ids.length} 条为已读`);
			setSelectedIds(new Set<string>());
		} catch {
			toast.error('批量标记失败');
		}
	}

	async function batchDelete() {
		const ids = [...selectedIds];
		if (ids.length === 0) return;
		writeList((list) => ({ ...list, items: list.items.filter((i) => !ids.includes(i.id)) }));
		try {
			await Promise.all(ids.map((id) => notificationService.delete(id)));
			toast.success(`已删除 ${ids.length} 条通知`);
			setSelectedIds(new Set<string>());
			void queryClient.invalidateQueries({ queryKey: qk.notifications.all });
		} catch {
			void queryClient.invalidateQueries({ queryKey: qk.notifications.all });
			toast.error('批量删除失败');
		}
	}

	return (
		<div className="flex w-full flex-col gap-3 pb-10">
			<PageHeader
				title="通知"
				meta={`${total} 条`}
				actions={
					<div className="flex items-center gap-1">
						{items.length > 0 ? (
							<Button variant="outline" size="sm" onClick={() => markAll.mutate()} className="gap-1">
								<CheckCheck className="size-3.5" aria-hidden="true" />
								全部已读
							</Button>
						) : null}
						<Button
							variant={batchMode ? 'default' : 'outline'}
							size="sm"
							aria-pressed={batchMode}
							onClick={() => {
								setBatchMode((v) => !v);
								setSelectedIds(new Set<string>());
							}}
						>
							批量操作
						</Button>
					</div>
				}
			/>

			<div className="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center sm:gap-4">
				<Segmented
					label="通知类型"
					value={type}
					options={TYPE_OPTIONS}
					onValueChange={switchType}
					className="min-w-0 overflow-x-auto sm:w-auto"
				/>

				<Segmented
					label="已读状态"
					value={read}
					options={READ_OPTIONS}
					onValueChange={switchRead}
					className="min-w-0 sm:w-auto"
				/>
			</div>

			{batchMode && selectedIds.size > 0 ? (
				<BatchActionBar
					selectedCount={selectedIds.size}
					totalCount={items.length}
					onSelectAll={selectAll}
					onDeselectAll={deselectAll}
					onBatchFollow={() => void batchMarkRead()}
					onBatchUnfollow={() => void batchDelete()}
				/>
			) : null}

			{isPending ? (
				<ul className="m-0 flex list-none flex-col gap-2 p-0">
					<SkeletonList count={5} type="notification" />
				</ul>
			) : (
				<NotificationList
					items={items}
					batchMode={batchMode}
					selectedIds={selectedIds}
					onToggleSelect={toggleSelect}
					onMarkRead={(n) => void markRead(n)}
					onDelete={deleteNotification}
					currentPage={page}
					totalPages={totalPages(total, DEFAULT_PAGE_SIZE)}
					onPageChange={goPage}
				/>
			)}
		</div>
	);
}
