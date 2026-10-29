'use client';

import type { ContentListItem, PostListItem } from '@feedchamber/api-client';
import { adminService } from '@feedchamber/api-client';
import { formatTime } from '@feedchamber/shared-utils';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';

import { Button } from '~/components/ui/button';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable } from '~/components/ui/data-table';
import { PageHeader } from '~/components/ui/page-header';
import { Segmented } from '~/components/ui/segmented';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

type TabKey = 'posts' | 'comments';

export default function Trash() {
	const [tab, setTab] = useState<TabKey>('posts');
	const [confirm, confirmDialog] = useConfirm();
	const queryClient = useQueryClient();

	/* 两个 tab 各自一条查询，用 enabled 让当前 tab 才取数——
	   原先是一个 useEffect 里 if/else 调用两个 fetch 函数，共用一份 loading。 */
	const postsQuery = useQuery({
		queryKey: qk.admin.trash.posts(),
		queryFn: () => adminService.listTrashPosts({ page: 1, page_size: 100 }),
		enabled: tab === 'posts',
	});

	const commentsQuery = useQuery({
		queryKey: qk.admin.trash.comments(),
		queryFn: () => adminService.listTrashComments({ page: 1, page_size: 100 }),
		enabled: tab === 'comments',
	});

	const posts = postsQuery.data?.data ?? [];
	const postTotal = postsQuery.data?.total ?? 0;
	const comments = commentsQuery.data?.data ?? [];
	const commentTotal = commentsQuery.data?.total ?? 0;
	const loading = postsQuery.isPending || commentsQuery.isPending;

	/** 写操作后让回收站缓存失效。 */
	const invalidate = useCallback(() => queryClient.invalidateQueries({ queryKey: qk.admin.trash.all }), [queryClient]);

	const restorePost = useCallback(
		async (id: string) => {
			try {
				await adminService.restorePost(id);
				toast.success('帖子已恢复');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '恢复失败');
			}
		},
		[invalidate],
	);

	const restoreComment = useCallback(
		async (id: string) => {
			try {
				await adminService.restoreComment(id);
				toast.success('评论已恢复');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '恢复失败');
			}
		},
		[invalidate],
	);

	const postColumns = useMemo<ColumnDef<PostListItem>[]>(
		() => [
			{ accessorKey: 'title', header: '标题' },
			{ accessorKey: 'author_name', header: '作者', size: 180 },
			{ accessorKey: 'space_name', header: '版面', size: 160 },
			{ accessorKey: 'comment_count', header: '评论数', size: 90 },
			{
				accessorKey: 'created_at',
				header: '创建时间',
				size: 180,
				cell: ({ getValue }) => {
					const v = getValue<string>();
					return v ? formatTime(v) : '-';
				},
			},
			{
				id: 'actions',
				header: '操作',
				size: 100,
				cell: ({ row }) => (
					<Button
						variant="outline"
						size="xs"
						onClick={() =>
							confirm({
								title: '恢复该帖子？',
								description: '帖子会回到原来的版面，重新对所有人可见。',
								confirmText: '恢复',
								onConfirm: () => restorePost(row.original.id),
							})
						}
					>
						恢复
					</Button>
				),
			},
		],
		[confirm, restorePost],
	);

	const commentColumns = useMemo<ColumnDef<ContentListItem>[]>(
		() => [
			{ accessorKey: 'body', header: '正文' },
			{ accessorKey: 'author_name', header: '作者', size: 180 },
			{ accessorKey: 'post_title', header: '帖子' },
			{
				accessorKey: 'created_at',
				header: '创建时间',
				size: 180,
				cell: ({ getValue }) => {
					const v = getValue<string>();
					return v ? formatTime(v) : '-';
				},
			},
			{
				id: 'actions',
				header: '操作',
				size: 100,
				cell: ({ row }) => (
					<Button
						variant="outline"
						size="xs"
						onClick={() =>
							confirm({
								title: '恢复该评论？',
								description: '评论会回到原帖下方，重新对所有人可见。',
								confirmText: '恢复',
								onConfirm: () => restoreComment(row.original.id),
							})
						}
					>
						恢复
					</Button>
				),
			},
		],
		[confirm, restoreComment],
	);

	return (
		<div>
			<PageHeader title="回收站" />
			<div className="mb-3">
				<Segmented
					label="回收站类型"
					value={tab}
					onValueChange={(v) => setTab(v as TabKey)}
					options={[
						{ value: 'posts', label: `帖子 (${postTotal})` },
						{ value: 'comments', label: `评论 (${commentTotal})` },
					]}
				/>
			</div>
			{tab === 'posts' ? (
				<DataTable columns={postColumns} data={posts} loading={loading} getRowId={(row) => row.id} />
			) : (
				<DataTable columns={commentColumns} data={comments} loading={loading} getRowId={(row) => row.id} />
			)}
			{confirmDialog}
		</div>
	);
}
