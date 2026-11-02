'use client';

import { type SecurityEvent, securityService } from '@feedchamber/api-client';
import { formatTime } from '@feedchamber/shared-utils';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useMemo } from 'react';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { DataTable } from '~/components/ui/data-table';
import { PageHeader } from '~/components/ui/page-header';
import { qk } from '~/lib/query/keys';

/* 严重级别 → 徽标变体。原来用 antd Tag 的 color 字符串（default/gold/orange/red），
   这里映射到主站徽标语义：critical/high 用 destructive，medium 用 warning，low 用中性。 */
const severityVariant: Record<string, 'default' | 'warning' | 'destructive' | 'outline'> = {
	low: 'outline',
	medium: 'warning',
	high: 'destructive',
	critical: 'destructive',
};

export default function SecurityEvents() {
	const queryClient = useQueryClient();

	const {
		data = [],
		isPending: loading,
		refetch,
	} = useQuery({
		queryKey: qk.admin.security.events(),
		queryFn: async () => {
			const items = await securityService.listEvents();
			return items ?? [];
		},
	});

	/** 写操作后让该模块的列表缓存失效。 */
	const _invalidate = () => queryClient.invalidateQueries({ queryKey: qk.admin.security.all });

	const columns = useMemo<ColumnDef<SecurityEvent>[]>(
		() => [
			{
				accessorKey: 'created_at',
				header: '时间',
				size: 180,
				cell: ({ getValue }) => {
					const v = getValue<number>();
					return v ? formatTime(new Date(v * 1000).toISOString()) : '-';
				},
			},
			{ accessorKey: 'event_type', header: '类型', size: 160 },
			{
				accessorKey: 'severity',
				header: '严重级别',
				size: 110,
				cell: ({ getValue }) => {
					const v = getValue<string>();
					return <Badge variant={severityVariant[v] ?? 'outline'}>{v}</Badge>;
				},
			},
			{ accessorKey: 'user_uuid', header: '用户', size: 220 },
			{ accessorKey: 'ip', header: 'IP', size: 160 },
			{ accessorKey: 'detail', header: '详情' },
			{ accessorKey: 'action', header: '动作', size: 120 },
			{
				accessorKey: 'handled',
				header: '已处理',
				size: 90,
				cell: ({ getValue }) => (getValue<boolean>() ? '是' : '否'),
			},
		],
		[],
	);

	return (
		<div>
			<PageHeader
				title="安全事件"
				actions={
					<Button variant="outline" size="sm" onClick={() => refetch()}>
						刷新
					</Button>
				}
			/>
			<DataTable columns={columns} data={data} loading={loading} getRowId={(row) => String(row.id)} />
		</div>
	);
}
