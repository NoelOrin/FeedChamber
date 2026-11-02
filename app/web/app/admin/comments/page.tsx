'use client';

import { adminService, type ContentListItem, contentService } from '@feedchamber/api-client';
import { formatTime } from '@feedchamber/shared-utils';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef, RowSelectionState } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';

import { Button } from '~/components/ui/button';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable, selectColumn } from '~/components/ui/data-table';
import { PageHeader } from '~/components/ui/page-header';
import { useNavigate } from '~/lib/admin/navigation';
import { RefreshCw } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

/*
 * ProTable → DataTable 的关键差异：
 *   ProTable 用 request(params) 回调自己管取数/分页/加载态/刷新（actionRef.reload）；
 *   这里把取数提到页面用 useState + 显式 fetchData，DataTable 只接收 data/page/total。
 *   于是「刷新」就是再调一次 fetchData，不需要 actionRef 这种命令式句柄。
 */
export default function CommentList() {
	const navigate = useNavigate();
	const [page, setPage] = useState(1);
	const [pageSize] = useState(20);
	const queryClient = useQueryClient();

	const {
		data: res,
		isPending: loading,
		refetch,
	} = useQuery({
		queryKey: qk.admin.comments.list({ page, pageSize }),
		queryFn: () => contentService.listAll({ page, page_size: pageSize }),
	});

	const data = res?.data ?? [];
	const total = res?.total ?? 0;

	/** 写操作后让该模块的列表缓存失效。 */
	const invalidate = useCallback(
		() => queryClient.invalidateQueries({ queryKey: qk.admin.comments.all }),
		[queryClient],
	);
	const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
	const [batchLoading, setBatchLoading] = useState(false);
	const [confirm, confirmDialog] = useConfirm();

	const selectedIds = Object.keys(rowSelection).filter((k) => rowSelection[k]);

	const handleDelete = useCallback(
		async (record: ContentListItem) => {
			try {
				await contentService.adminDelete(record.id);
				toast.success('删除成功');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '删除失败');
			}
		},
		[invalidate],
	);

	async function handleBatchDelete() {
		if (selectedIds.length === 0) {
			toast.error('请先选择评论');
			return;
		}
		setBatchLoading(true);
		try {
			await adminService.batchComments(selectedIds as string[]);
			toast.success('批量删除成功');
			setRowSelection({});
			invalidate();
		} catch (e: any) {
			toast.error(e.message || '批量删除失败');
		} finally {
			setBatchLoading(false);
		}
	}

	const columns = useMemo<ColumnDef<ContentListItem>[]>(
		() => [
			selectColumn<ContentListItem>(),
			{ accessorKey: 'id', header: '评论 ID', size: 120 },
			{ accessorKey: 'author_name', header: '作者', size: 120 },
			{
				accessorKey: 'body',
				header: '内容',
				size: 400,
				cell: ({ getValue }) => (
					<p className="line-clamp-2 max-w-[400px] text-[13px] text-foreground" title={getValue<string>()}>
						{getValue<string>()}
					</p>
				),
			},
			{ accessorKey: 'like_count', header: '点赞', size: 60 },
			{
				accessorKey: 'post_title',
				header: '帖子',
				size: 150,
				cell: ({ row }) =>
					row.original.post_title ? (
						<Button
							variant="link"
							size="xs"
							onClick={() => navigate({ to: '/admin/posts/detail', search: { id: row.original.post_id } })}
						>
							{row.original.post_title}
						</Button>
					) : (
						(row.original.post_id ?? '-')
					),
			},
			{ accessorKey: 'post_id', header: '帖子 ID', size: 120 },
			{
				accessorKey: 'created_at',
				header: '创建时间',
				size: 170,
				cell: ({ getValue }) => formatTime(getValue<string>()),
			},
			{
				id: 'actions',
				header: '操作',
				size: 180,
				cell: ({ row }) => (
					<div className="flex items-center gap-1.5">
						<Button
							variant="outline"
							size="xs"
							onClick={() => navigate({ to: '/admin/comments/detail', search: { id: row.original.id } })}
						>
							详情
						</Button>
						<Button
							variant="destructive"
							size="xs"
							onClick={() =>
								confirm({
									title: '删除该评论？',
									description: '评论会从帖子下移除，且不可恢复。',
									danger: true,
									confirmText: '删除',
									onConfirm: () => handleDelete(row.original),
								})
							}
						>
							删除
						</Button>
					</div>
				),
			},
		],
		[confirm, navigate, handleDelete],
	);

	return (
		<div>
			<PageHeader
				title="评论管理"
				actions={
					<div className="flex items-center gap-2">
						<Button variant="outline" size="sm" onClick={() => refetch()}>
							<RefreshCw className="size-3.5" />
							刷新
						</Button>
						<Button
							variant="destructive"
							size="sm"
							disabled={!selectedIds.length}
							loading={batchLoading}
							onClick={() =>
								confirm({
									title: `删除选中的 ${selectedIds.length} 条评论？`,
									description: '这些评论会从各自帖子下移除，且不可恢复。',
									danger: true,
									confirmText: '批量删除',
									onConfirm: handleBatchDelete,
								})
							}
						>
							批量删除{selectedIds.length ? ` (${selectedIds.length})` : ''}
						</Button>
					</div>
				}
			/>
			<DataTable
				columns={columns}
				data={data}
				loading={loading}
				getRowId={(row) => row.id}
				rowSelection={rowSelection}
				onRowSelectionChange={setRowSelection}
				pagination={{
					page,
					pageSize,
					total,
					onChange: (p) => {
						setPage(p);
						invalidate();
					},
				}}
			/>
			{confirmDialog}
		</div>
	);
}
