'use client';

import { adminService, type PostListItem, PostStatus, postService, STATUS_FILTER_ALL } from '@orinforum/api-client';
import { formatTime } from '@orinforum/shared-utils';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef, RowSelectionState } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable, selectColumn } from '~/components/ui/data-table';
import { PageHeader } from '~/components/ui/page-header';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { useNavigate } from '~/lib/admin/navigation';
import { CircleCheck, CircleX, RefreshCw, Star } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

/* 状态 → 徽标。原来用 antd 的 Tag color + valueEnum 双轨（一份文案给筛选器、
   一份给渲染），现在只留一份映射，筛选器与单元格共用。 */
const STATUS: Record<string, { text: string; variant: 'warning' | 'primary' | 'destructive' | 'outline' }> = {
	pending: { text: '待审', variant: 'warning' },
	approved: { text: '已通过', variant: 'primary' },
	rejected: { text: '已拒绝', variant: 'destructive' },
	closed: { text: '已关闭', variant: 'outline' },
};

/* 「全部状态」= 不过滤状态（后端只对 report:manage 持有者生效）。
   默认就发这个哨兵，所以待审帖只会出现在这里，不会出现在前台列表。 */
const ALL = STATUS_FILTER_ALL;

export default function PostList() {
	const navigate = useNavigate();
	const [page, setPage] = useState(1);
	const [pageSize] = useState(20);
	const [statusFilter, setStatusFilter] = useState<string>(ALL);
	const queryClient = useQueryClient();

	const {
		data: res,
		isPending: loading,
		refetch,
	} = useQuery({
		queryKey: qk.admin.posts.list({ page, pageSize, status: statusFilter }),
		queryFn: () => postService.list({ page, page_size: pageSize, status: statusFilter }),
	});

	const data = res?.data ?? [];
	const total = res?.total ?? 0;

	/** 写操作后让该模块的列表缓存失效。 */
	const invalidate = useCallback(() => queryClient.invalidateQueries({ queryKey: qk.admin.posts.all }), [queryClient]);
	const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
	const [batchLoading, setBatchLoading] = useState(false);
	const [confirm, confirmDialog] = useConfirm();

	const selectedIds = Object.keys(rowSelection).filter((k) => rowSelection[k]);

	const handleTogglePin = useCallback(
		async (id: string) => {
			try {
				await postService.togglePin(id);
				toast.success('操作成功');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '操作失败');
			}
		},
		[invalidate],
	);

	const handleToggleLock = useCallback(
		async (id: string) => {
			try {
				await postService.toggleLock(id);
				toast.success('操作成功');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '操作失败');
			}
		},
		[invalidate],
	);

	/** 设为 / 取消精华：写 posts.featured_at，作者因此获得/回收 featured_post_count XP。 */
	const handleToggleFeatured = useCallback(
		async (id: string, featured: boolean) => {
			try {
				await postService.setFeatured(id, !featured);
				toast.success(featured ? '已取消精华' : '已设为精华');
				invalidate();
			} catch (e: any) {
				toast.error(e?.message || '操作失败');
			}
		},
		[invalidate],
	);

	const handleDelete = useCallback(
		async (id: string) => {
			try {
				await postService.delete(id);
				toast.success('删除成功');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '删除失败');
			}
		},
		[invalidate],
	);

	/** 审核流转：存量待审帖只能在这里救回来（前台列表看不到 pending）。 */
	const handleSetStatus = useCallback(
		async (id: string, next: PostStatus, label: string) => {
			try {
				await postService.setStatus(id, next);
				toast.success(`${label}成功`);
				invalidate();
				/* 审核结果决定帖子在前台是否可见，前台列表/详情缓存也要失效 */
				queryClient.invalidateQueries({ queryKey: qk.posts.all });
			} catch (e) {
				toast.error((e as Error)?.message || `${label}失败`);
			}
		},
		[invalidate, queryClient],
	);

	async function doBatchAction(action: string, label: string) {
		if (selectedIds.length === 0) {
			toast.error('请先选择帖子');
			return;
		}
		setBatchLoading(true);
		try {
			await adminService.batchPosts({ ids: selectedIds as string[], action });
			toast.success(`${label}成功`);
			setRowSelection({});
			invalidate();
		} catch (e: any) {
			toast.error(e.message || `${label}失败`);
		} finally {
			setBatchLoading(false);
		}
	}

	const columns = useMemo<ColumnDef<PostListItem>[]>(
		() => [
			selectColumn<PostListItem>(),
			{
				accessorKey: 'title',
				header: '标题',
				size: 220,
				cell: ({ row }) => (
					<Button
						variant="link"
						size="xs"
						className="h-auto max-w-[220px] truncate p-0"
						onClick={() => navigate({ to: '/admin/posts/detail', search: { id: row.original.id } })}
					>
						{row.original.title}
					</Button>
				),
			},
			{ accessorKey: 'author_name', header: '作者', size: 100 },
			{
				accessorKey: 'status',
				header: '状态',
				size: 90,
				cell: ({ getValue }) => {
					const cfg = STATUS[getValue<string>()];
					return cfg ? <Badge variant={cfg.variant}>{cfg.text}</Badge> : getValue<string>();
				},
			},
			{
				accessorKey: 'featured_at',
				header: '精华',
				size: 70,
				cell: ({ getValue }) =>
					getValue<string | null>() ? <Badge variant="warning">精华</Badge> : <Badge variant="outline">否</Badge>,
			},
			{
				accessorKey: 'pinned',
				header: '置顶',
				size: 60,
				cell: ({ getValue }) =>
					getValue<boolean>() ? <Badge variant="primary">是</Badge> : <Badge variant="outline">否</Badge>,
			},
			{
				accessorKey: 'locked',
				header: '锁定',
				size: 60,
				cell: ({ getValue }) =>
					getValue<boolean>() ? <Badge variant="destructive">是</Badge> : <Badge variant="outline">否</Badge>,
			},
			{ accessorKey: 'view_count', header: '浏览', size: 60 },
			{ accessorKey: 'comment_count', header: '评论', size: 60 },
			{
				accessorKey: 'created_at',
				header: '创建时间',
				size: 160,
				cell: ({ getValue }) => formatTime(getValue<string>()),
			},
			{
				id: 'actions',
				header: '操作',
				size: 420,
				cell: ({ row }) => {
					const record = row.original;
					return (
						<div className="flex flex-wrap items-center gap-1.5">
							{record.status !== PostStatus.APPROVED ? (
								<Button
									variant="outline"
									size="xs"
									className="gap-1"
									onClick={() => handleSetStatus(record.id, PostStatus.APPROVED, '通过')}
								>
									<CircleCheck className="size-3.5" aria-hidden="true" />
									通过
								</Button>
							) : null}
							{record.status !== PostStatus.REJECTED ? (
								<Button
									variant="destructive"
									size="xs"
									className="gap-1"
									onClick={() => handleSetStatus(record.id, PostStatus.REJECTED, '拒绝')}
								>
									<CircleX className="size-3.5" aria-hidden="true" />
									拒绝
								</Button>
							) : null}
							<Button
								variant="outline"
								size="xs"
								onClick={() => navigate({ to: '/admin/posts/detail', search: { id: record.id } })}
							>
								详情
							</Button>
							<Button
								variant={record.featured_at ? 'default' : 'outline'}
								size="xs"
								className="gap-1"
								onClick={() => handleToggleFeatured(record.id, Boolean(record.featured_at))}
							>
								<Star className="size-3.5" aria-hidden="true" />
								{record.featured_at ? '取消精华' : '设为精华'}
							</Button>
							<Button
								variant={record.pinned ? 'default' : 'outline'}
								size="xs"
								onClick={() => handleTogglePin(record.id)}
							>
								{record.pinned ? '取消置顶' : '置顶'}
							</Button>
							<Button
								variant={record.locked ? 'destructive' : 'outline'}
								size="xs"
								onClick={() => handleToggleLock(record.id)}
							>
								{record.locked ? '解锁' : '锁定'}
							</Button>
							<Button
								variant="destructive"
								size="xs"
								onClick={() =>
									confirm({
										title: '删除该帖子？',
										description: '帖子及其评论会一并移除，且不可恢复（可从回收站找回）。',
										danger: true,
										confirmText: '删除',
										onConfirm: () => handleDelete(record.id),
									})
								}
							>
								删除
							</Button>
						</div>
					);
				},
			},
		],
		[confirm, navigate, handleTogglePin, handleToggleLock, handleToggleFeatured, handleDelete, handleSetStatus],
	);

	return (
		<div>
			<PageHeader
				title="帖子管理"
				actions={
					<div className="flex flex-wrap items-center gap-2">
						<Select
							value={statusFilter}
							onValueChange={(v) => {
								setStatusFilter(v);
								setPage(1);
								invalidate();
							}}
						>
							<SelectTrigger className="h-8 w-[130px]">
								<SelectValue placeholder="全部状态" />
							</SelectTrigger>
							<SelectContent>
								<SelectItem value={ALL}>全部状态</SelectItem>
								<SelectItem value="pending">待审</SelectItem>
								<SelectItem value="approved">已通过</SelectItem>
								<SelectItem value="rejected">已拒绝</SelectItem>
								<SelectItem value="closed">已关闭</SelectItem>
							</SelectContent>
						</Select>
						<Button variant="outline" size="sm" onClick={() => refetch()}>
							<RefreshCw className="size-3.5" />
							刷新
						</Button>
					</div>
				}
			/>

			{selectedIds.length > 0 ? (
				<div className="mb-2 flex flex-wrap items-center gap-2">
					<Button size="sm" loading={batchLoading} onClick={() => doBatchAction('pin', '置顶')}>
						批量置顶
					</Button>
					<Button variant="outline" size="sm" loading={batchLoading} onClick={() => doBatchAction('unpin', '取消置顶')}>
						取消置顶
					</Button>
					<Button variant="destructive" size="sm" loading={batchLoading} onClick={() => doBatchAction('lock', '锁定')}>
						批量锁定
					</Button>
					<Button variant="outline" size="sm" loading={batchLoading} onClick={() => doBatchAction('unlock', '解锁')}>
						批量解锁
					</Button>
					<Button
						variant="destructive"
						size="sm"
						loading={batchLoading}
						onClick={() =>
							confirm({
								title: `删除选中的 ${selectedIds.length} 条帖子？`,
								description: '这些帖子及其评论会一并移除，且不可恢复（可从回收站找回）。',
								danger: true,
								confirmText: '批量删除',
								onConfirm: () => doBatchAction('delete', '删除'),
							})
						}
					>
						批量删除
					</Button>
				</div>
			) : null}

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
