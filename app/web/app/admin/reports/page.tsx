'use client';

import { type Report, reportService, STATUS_FILTER_ALL } from '@feedchamber/api-client';
import { formatTime } from '@feedchamber/shared-utils';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable } from '~/components/ui/data-table';
import { PageHeader } from '~/components/ui/page-header';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { useNavigate } from '~/lib/admin/navigation';
import { RefreshCw } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

const STATUS: Record<string, { text: string; variant: 'warning' | 'primary' | 'outline' }> = {
	pending: { text: '待处理', variant: 'warning' },
	resolved: { text: '已处理', variant: 'primary' },
	dismissed: { text: '已驳回', variant: 'outline' },
};

const targetTypeMap: Record<string, string> = { post: '帖子', comment: '评论' };
/* 「全部状态」哨兵与帖子列表共用同一个常量（后端只对 report:manage 持有者按「不过滤」处理）。 */
const ALL = STATUS_FILTER_ALL;

export default function ReportList() {
	const navigate = useNavigate();
	const [page, setPage] = useState(1);
	const [pageSize] = useState(20);
	const [status, setStatus] = useState<string>(ALL);
	const queryClient = useQueryClient();

	const {
		data: res,
		isPending: loading,
		refetch,
	} = useQuery({
		queryKey: qk.admin.reports.list({ page, pageSize, status }),
		queryFn: () =>
			reportService.list({
				page,
				page_size: pageSize,
				status: status === ALL ? undefined : status,
			}),
	});

	const data = res?.items ?? [];
	const total = res?.total ?? 0;

	/** 写操作后让该模块的列表缓存失效。 */
	const invalidate = useCallback(
		() => queryClient.invalidateQueries({ queryKey: qk.admin.reports.all }),
		[queryClient],
	);
	const [confirm, confirmDialog] = useConfirm();

	const handleUpdateStatus = useCallback(
		async (id: string, next: string) => {
			try {
				await reportService.updateStatus(id, next);
				toast.success('状态更新成功');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '操作失败');
			}
		},
		[invalidate],
	);

	const handleDelete = useCallback(
		async (id: string) => {
			try {
				await reportService.delete(id);
				toast.success('删除成功');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '删除失败');
			}
		},
		[invalidate],
	);

	const columns = useMemo<ColumnDef<Report>[]>(
		() => [
			{
				accessorKey: 'target_type',
				header: '举报类型',
				size: 100,
				cell: ({ getValue }) => targetTypeMap[getValue<string>()] || getValue<string>(),
			},
			{
				accessorKey: 'flag_type',
				header: 'Flag',
				size: 120,
				cell: ({ getValue }) => getValue<string>() || '-',
			},
			{
				accessorKey: 'count',
				header: '聚合数',
				size: 80,
				cell: ({ getValue }) => getValue<number>() ?? 1,
			},
			{ accessorKey: 'reason', header: '举报原因' },
			{
				accessorKey: 'target_title',
				header: '目标',
				cell: ({ row }) => row.original.target_title || row.original.target_id,
			},
			{ accessorKey: 'reporter_id', header: '举报人 ID', size: 130 },
			{
				accessorKey: 'status',
				header: '状态',
				size: 100,
				cell: ({ getValue }) => {
					const cfg = STATUS[getValue<string>()];
					return cfg ? <Badge variant={cfg.variant}>{cfg.text}</Badge> : getValue<string>();
				},
			},
			{
				accessorKey: 'created_at',
				header: '创建时间',
				size: 180,
				cell: ({ getValue }) => formatTime(getValue<string>()),
			},
			{
				id: 'actions',
				header: '操作',
				size: 320,
				cell: ({ row }) => {
					const record = row.original;
					return (
						<div className="flex items-center gap-1.5">
							<Button
								variant="outline"
								size="xs"
								onClick={() => navigate({ to: '/admin/reports/detail', search: { id: record.id } })}
							>
								详情
							</Button>
							{record.status === 'pending' ? (
								<>
									<Button size="xs" onClick={() => handleUpdateStatus(record.id, 'resolved')}>
										处理
									</Button>
									<Button variant="outline" size="xs" onClick={() => handleUpdateStatus(record.id, 'dismissed')}>
										驳回
									</Button>
								</>
							) : null}
							<Button
								variant="destructive"
								size="xs"
								onClick={() =>
									confirm({
										title: '删除该举报？',
										description: '举报记录会永久删除，被举报内容不受影响。',
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
		[confirm, navigate, handleUpdateStatus, handleDelete],
	);

	return (
		<div>
			<PageHeader
				title="举报管理"
				actions={
					<div className="flex items-center gap-2">
						<Select
							value={status}
							onValueChange={(v) => {
								setStatus(v);
								setPage(1);
								invalidate();
							}}
						>
							<SelectTrigger className="h-8 w-36">
								<SelectValue placeholder="全部状态" />
							</SelectTrigger>
							<SelectContent>
								<SelectItem value={ALL}>全部状态</SelectItem>
								<SelectItem value="pending">待处理</SelectItem>
								<SelectItem value="resolved">已处理</SelectItem>
								<SelectItem value="dismissed">已驳回</SelectItem>
							</SelectContent>
						</Select>
						<Button variant="outline" size="sm" onClick={() => refetch()}>
							<RefreshCw className="size-3.5" />
							刷新
						</Button>
					</div>
				}
			/>
			<DataTable
				columns={columns}
				data={data}
				loading={loading}
				getRowId={(row) => row.id}
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
