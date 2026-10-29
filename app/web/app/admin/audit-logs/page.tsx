'use client';

import { type AuditLog, adminService } from '@feedchamber/api-client';
import { formatTime } from '@feedchamber/shared-utils';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useMemo, useState } from 'react';

import { Button } from '~/components/ui/button';
import { DataTable } from '~/components/ui/data-table';
import { Input } from '~/components/ui/input';
import { PageHeader } from '~/components/ui/page-header';
import { qk } from '~/lib/query/keys';

export default function AdminAuditLogs() {
	const [page, setPage] = useState(1);
	const [pageSize] = useState(20);
	const [actorID, setActorID] = useState('');
	const [action, setAction] = useState('');
	const queryClient = useQueryClient();

	const { data: res, isPending: loading } = useQuery({
		queryKey: qk.admin.auditLogs.list({ page, pageSize, actorID, action }),
		queryFn: () =>
			adminService.listAuditLogs({
				page,
				pageSize: pageSize,
				actor_id: actorID || undefined,
				action: action || undefined,
			}),
	});

	const data = res?.items ?? [];
	const total = res?.total ?? 0;

	/** 写操作后让该模块的列表缓存失效。 */
	const invalidate = () => queryClient.invalidateQueries({ queryKey: qk.admin.auditLogs.all });

	const columns = useMemo<ColumnDef<AuditLog>[]>(
		() => [
			{
				accessorKey: 'created_at',
				header: '时间',
				size: 180,
				cell: ({ getValue }) => formatTime(getValue<string>()),
			},
			{ accessorKey: 'actor_id', header: '操作者', size: 200 },
			{ accessorKey: 'action', header: '动作', size: 200 },
			{ accessorKey: 'target', header: '目标', size: 180 },
			{ accessorKey: 'detail', header: '详情' },
		],
		[],
	);

	return (
		<div>
			<PageHeader title="审计日志" />
			<div className="mb-3 flex flex-wrap items-center gap-2">
				<Input
					placeholder="操作者 ID"
					value={actorID}
					onChange={(e) => setActorID(e.target.value)}
					className="w-[220px]"
				/>
				<Input
					placeholder="动作，如 settings.update"
					value={action}
					onChange={(e) => setAction(e.target.value)}
					className="w-60"
				/>
				<Button
					size="sm"
					onClick={() => {
						setPage(1);
						invalidate();
					}}
				>
					查询
				</Button>
				<Button
					variant="outline"
					size="sm"
					onClick={() => {
						setActorID('');
						setAction('');
						setPage(1);
						invalidate();
					}}
				>
					重置
				</Button>
			</div>
			<DataTable
				columns={columns}
				data={data}
				loading={loading}
				getRowId={(row) => String(row.id)}
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
		</div>
	);
}
