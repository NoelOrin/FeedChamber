'use client';

import {
	type ModerationItem,
	ModerationMode,
	moderationService,
	SettingKey,
	settingService,
} from '@feedchamber/api-client';
import { cx, formatTime } from '@feedchamber/shared-utils';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { DataTable } from '~/components/ui/data-table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { PageHeader } from '~/components/ui/page-header';
import { Textarea } from '~/components/ui/textarea';
import { RefreshCw } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

const STATUS: Record<string, { text: string; variant: 'warning' | 'primary' | 'destructive' }> = {
	pending: { text: '待审核', variant: 'warning' },
	approved: { text: '已通过', variant: 'primary' },
	rejected: { text: '已拒绝', variant: 'destructive' },
};

const targetTypeMap: Record<string, string> = { post: '帖子', content: '评论' };

/** 四种审核模式的展示文案；取值与后端 model.ModerationMode* 常量一致。 */
const MODES: { value: ModerationMode; label: string; hint: string }[] = [
	{ value: ModerationMode.OFF, label: '不审核', hint: '发布即公开，不跑规则、不进审核队列' },
	{ value: ModerationMode.ASYNC, label: '发布后异步审核', hint: '先公开，后台规则命中即转待审，交人工判断' },
	{ value: ModerationMode.EXTERNAL, label: '外部自动化审核', hint: '发布前调用外部审核服务，服务不可用回退异步' },
	{ value: ModerationMode.MANUAL, label: '全部人工审核', hint: '所有新帖先待审，人工通过后才公开' },
];

export default function ModerationQueue() {
	const [page, setPage] = useState(1);
	const [pageSize] = useState(20);
	const [savingMode, setSavingMode] = useState(false);
	const queryClient = useQueryClient();

	const {
		data: res,
		isPending: loading,
		refetch,
	} = useQuery({
		queryKey: qk.admin.moderation.pending({ page, pageSize }),
		queryFn: () => moderationService.listPending({ page, page_size: pageSize }),
	});

	/* 审核模式存在系统设置表（moderation_mode），与「设置 → 内容」同一个 key，
	   这里只是提供就近入口，避免运营为了切模式在两个页面之间来回跳。 */
	const { data: settings } = useQuery({
		queryKey: qk.admin.settings.list,
		queryFn: () => settingService.getAll(),
	});
	const activeMode = (settings?.[SettingKey.MODERATION_MODE] as ModerationMode | undefined) ?? ModerationMode.ASYNC;
	const activeModeHint = MODES.find((m) => m.value === activeMode)?.hint ?? '';

	const handleModeChange = useCallback(
		async (next: ModerationMode) => {
			if (next === activeMode || savingMode) return;
			setSavingMode(true);
			try {
				await settingService.update(SettingKey.MODERATION_MODE, next);
				await queryClient.invalidateQueries({ queryKey: qk.admin.settings.all });
				toast.success('审核模式已更新');
			} catch (e: any) {
				toast.error(e?.message || '更新失败');
			} finally {
				setSavingMode(false);
			}
		},
		[activeMode, queryClient, savingMode],
	);

	const data = res?.data ?? [];
	const total = res?.total ?? 0;

	/** 写操作后让该模块的列表缓存失效。 */
	const invalidate = useCallback(
		() => queryClient.invalidateQueries({ queryKey: qk.admin.moderation.all }),
		[queryClient],
	);
	const [rejectOpen, setRejectOpen] = useState(false);
	const [rejectId, setRejectId] = useState('');
	const [rejectReason, setRejectReason] = useState('');
	const [actionLoading, setActionLoading] = useState(false);

	const handleApprove = useCallback(
		async (id: string) => {
			setActionLoading(true);
			try {
				await moderationService.approve(id);
				toast.success('已通过');
				invalidate();
			} catch (e: any) {
				toast.error(e?.message || '操作失败');
			} finally {
				setActionLoading(false);
			}
		},
		[invalidate],
	);

	const openReject = useCallback((id: string) => {
		setRejectId(id);
		setRejectReason('');
		setRejectOpen(true);
	}, []);

	async function handleReject() {
		if (!rejectId) return;
		setActionLoading(true);
		try {
			await moderationService.reject(rejectId, rejectReason.trim());
			toast.success('已拒绝');
			setRejectOpen(false);
			invalidate();
		} catch (e: any) {
			toast.error(e?.message || '操作失败');
		} finally {
			setActionLoading(false);
		}
	}

	const columns = useMemo<ColumnDef<ModerationItem>[]>(
		() => [
			{
				accessorKey: 'target_type',
				header: '目标类型',
				size: 100,
				cell: ({ getValue }) => targetTypeMap[getValue<string>()] || getValue<string>(),
			},
			{ accessorKey: 'target_id', header: '目标 ID', size: 160 },
			{
				accessorKey: 'scene',
				header: '场景',
				size: 100,
				cell: ({ getValue }) => getValue<string>() || '-',
			},
			{
				accessorKey: 'action',
				header: '动作',
				size: 100,
				cell: ({ getValue }) => getValue<string>() || '-',
			},
			{
				accessorKey: 'original_body',
				header: '正文摘要',
				cell: ({ getValue }) => {
					const v = getValue<string>();
					if (!v) return '-';
					return v.length > 80 ? `${v.slice(0, 80)}…` : v;
				},
			},
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
				size: 200,
				cell: ({ row }) => (
					<div className="flex items-center gap-1.5">
						<Button size="xs" loading={actionLoading} onClick={() => handleApprove(row.original.id)}>
							通过
						</Button>
						<Button variant="destructive" size="xs" loading={actionLoading} onClick={() => openReject(row.original.id)}>
							拒绝
						</Button>
					</div>
				),
			},
		],
		[actionLoading, handleApprove, openReject],
	);

	return (
		<div>
			<PageHeader
				title="内容审核"
				actions={
					<Button variant="outline" size="sm" onClick={() => refetch()}>
						<RefreshCw className="size-3.5" />
						刷新
					</Button>
				}
			/>
			<Card className="mb-3 gap-3 p-4">
				<div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
					<div className="flex flex-col gap-0.5">
						<span className="text-[13px] font-medium text-foreground">文章审核模式</span>
						<span className="text-xs text-muted-foreground">{activeModeHint}</span>
					</div>
					<span className="text-xs text-muted-foreground">评论始终走关键词规则，不受此开关影响</span>
				</div>
				<div className="grid grid-cols-2 gap-2 lg:grid-cols-4">
					{MODES.map((mode) => {
						const active = mode.value === activeMode;
						return (
							<button
								key={mode.value}
								type="button"
								disabled={savingMode || active}
								onClick={() => handleModeChange(mode.value)}
								className={cx(
									'flex flex-col items-start gap-1 rounded-md border p-2.5 text-left transition-colors',
									active
										? 'border-primary/60 bg-primary/5'
										: 'border-border hover:border-primary/40 hover:bg-muted/50 disabled:opacity-60',
								)}
							>
								<span className="flex w-full items-center justify-between gap-2">
									<span className="text-[13px] font-medium text-foreground">{mode.label}</span>
									{active ? <Badge variant="primary">当前</Badge> : null}
								</span>
								<span className="text-xs leading-relaxed text-muted-foreground">{mode.hint}</span>
							</button>
						);
					})}
				</div>
			</Card>
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

			<Dialog open={rejectOpen} onOpenChange={(v) => !actionLoading && setRejectOpen(v)}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>拒绝审核</DialogTitle>
					</DialogHeader>
					<Textarea
						rows={4}
						placeholder="请输入拒绝原因（可选）"
						value={rejectReason}
						onChange={(e) => setRejectReason(e.target.value)}
					/>
					<DialogFooter className="mt-2 gap-2">
						<Button variant="outline" size="sm" onClick={() => setRejectOpen(false)}>
							取消
						</Button>
						<Button variant="destructive" size="sm" loading={actionLoading} onClick={handleReject}>
							确认拒绝
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</div>
	);
}
