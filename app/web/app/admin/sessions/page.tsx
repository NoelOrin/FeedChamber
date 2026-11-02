'use client';

import { sessionService, type UserSession } from '@feedchamber/api-client';
import { formatTime } from '@feedchamber/shared-utils';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo } from 'react';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable } from '~/components/ui/data-table';
import { PageHeader } from '~/components/ui/page-header';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

export default function Sessions() {
	const [confirm, confirmDialog] = useConfirm();
	const queryClient = useQueryClient();

	const { data = [], isPending: loading } = useQuery({
		queryKey: qk.admin.sessions.list(),
		queryFn: async () => {
			const items = await sessionService.list();
			return items ?? [];
		},
	});

	/** 写操作后让该模块的列表缓存失效。 */
	const invalidate = useCallback(
		() => queryClient.invalidateQueries({ queryKey: qk.admin.sessions.all }),
		[queryClient],
	);

	const handleRevoke = useCallback(
		async (id: number) => {
			try {
				await sessionService.revoke(id);
				toast.success('会话已撤销');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '撤销失败');
			}
		},
		[invalidate],
	);

	async function handleLogoutAll() {
		try {
			await sessionService.logoutAll();
			toast.success('已撤销全部会话');
			invalidate();
		} catch (e: any) {
			toast.error(e.message || '操作失败');
		}
	}

	const columns = useMemo<ColumnDef<UserSession>[]>(
		() => [
			{ accessorKey: 'device_name', header: '设备', size: 220 },
			{ accessorKey: 'ip', header: 'IP', size: 160 },
			{ accessorKey: 'user_agent', header: 'User-Agent' },
			{
				accessorKey: 'last_active',
				header: '最近活跃',
				size: 180,
				cell: ({ getValue }) => {
					const v = getValue<number>();
					return v ? formatTime(new Date(v * 1000).toISOString()) : '-';
				},
			},
			{
				accessorKey: 'expires_at',
				header: '过期时间',
				size: 180,
				cell: ({ getValue }) => {
					const v = getValue<number>();
					return v ? formatTime(new Date(v * 1000).toISOString()) : '-';
				},
			},
			{
				accessorKey: 'is_current',
				header: '当前',
				size: 80,
				cell: ({ getValue }) => (getValue<boolean>() ? <Badge variant="primary">当前</Badge> : '-'),
			},
			{
				id: 'actions',
				header: '操作',
				size: 120,
				cell: ({ row }) =>
					row.original.is_current ? null : (
						<Button
							variant="destructive"
							size="xs"
							onClick={() =>
								confirm({
									title: '撤销该会话？',
									description: '该设备将立即退出登录，需要重新输入密码。',
									danger: true,
									confirmText: '撤销',
									onConfirm: () => handleRevoke(row.original.id),
								})
							}
						>
							撤销
						</Button>
					),
			},
		],
		[confirm, handleRevoke],
	);

	return (
		<div>
			<PageHeader
				title="会话管理"
				actions={
					<Button
						variant="destructive"
						size="sm"
						onClick={() =>
							confirm({
								title: '撤销全部其他会话？',
								description: '当前账号在其他设备上的登录会全部失效。',
								danger: true,
								confirmText: '全部撤销',
								onConfirm: handleLogoutAll,
							})
						}
					>
						撤销全部会话
					</Button>
				}
			/>
			<DataTable columns={columns} data={data} loading={loading} getRowId={(row) => String(row.id)} />
			{confirmDialog}
		</div>
	);
}
