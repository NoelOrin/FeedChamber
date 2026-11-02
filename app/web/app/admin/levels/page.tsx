'use client';

import { type TrustLevel, trustService } from '@feedchamber/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable } from '~/components/ui/data-table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { PageHeader } from '~/components/ui/page-header';
import { Switch } from '~/components/ui/switch';
import { Plus, RefreshCw } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

/* 等级定义：名称 / XP 门槛 / 100 天窗口保级条件全部可配。
   等级数量不固定，新增时取当前最大等级 + 1。 */
type Draft = TrustLevel;

function emptyDraft(levels: TrustLevel[]): Draft {
	const max = levels.reduce((acc, l) => Math.max(acc, l.level), -1);
	return {
		level: max + 1,
		key: `tl${max + 1}`,
		name: '',
		min_xp: 0,
		window_days: 100,
		min_active_days: 0,
		min_posts: 0,
		min_likes: 0,
		max_reports: null,
		manual_only: false,
		enabled: true,
	};
}

export default function LevelsPage() {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);
	const [draft, setDraft] = useState<Draft | null>(null);
	const [editing, setEditing] = useState(false);
	const [saving, setSaving] = useState(false);
	const [confirm, confirmDialog] = useConfirm();

	const { data: levels = [], isPending } = useQuery({
		queryKey: qk.admin.trust.levels,
		queryFn: () => trustService.listLevels(),
	});

	const invalidate = useCallback(
		() => queryClient.invalidateQueries({ queryKey: qk.admin.trust.levels }),
		[queryClient],
	);

	const recompute = useMutation({
		mutationFn: () => trustService.recomputeAll(),
		onSuccess: (res) => {
			toast.success(`已重算 ${res?.count ?? 0} 个用户`);
			queryClient.invalidateQueries({ queryKey: qk.admin.users.all });
		},
		onError: (e: Error) => toast.error(e.message || '重算失败'),
	});

	function openCreate() {
		setDraft(emptyDraft(levels));
		setEditing(false);
		setOpen(true);
	}

	const openEdit = useCallback((level: TrustLevel) => {
		setDraft({ ...level });
		setEditing(true);
		setOpen(true);
	}, []);

	async function submit() {
		if (!draft) return;
		if (!draft.name.trim()) {
			toast.error('等级名称不能为空');
			return;
		}
		setSaving(true);
		try {
			await trustService.saveLevel(draft);
			toast.success('保存成功');
			setOpen(false);
			invalidate();
			/* 等级求值结果会变，用户详情缓存一并失效 */
			queryClient.invalidateQueries({ queryKey: qk.admin.users.all });
		} catch (e: any) {
			toast.error(e?.message || '保存失败');
		} finally {
			setSaving(false);
		}
	}

	const remove = useCallback(
		(level: TrustLevel) => {
			confirm({
				title: `删除等级 ${level.name}？`,
				description: '删除后下一次等级评估将不再考虑该等级；已处于该等级的用户会被重新定级。',
				danger: true,
				confirmText: '删除',
				onConfirm: async () => {
					try {
						await trustService.deleteLevel(level.level);
						toast.success('删除成功');
						invalidate();
						queryClient.invalidateQueries({ queryKey: qk.admin.users.all });
					} catch (e: any) {
						toast.error(e?.message || '删除失败');
					}
				},
			});
		},
		[confirm, invalidate, queryClient],
	);

	const columns = useMemo<ColumnDef<TrustLevel>[]>(
		() => [
			{ accessorKey: 'level', header: '等级', size: 70 },
			{
				accessorKey: 'name',
				header: '名称',
				size: 140,
				cell: ({ row }) => (
					<span className="flex items-center gap-2">
						{row.original.name}
						{row.original.manual_only ? <Badge variant="warning">仅人工</Badge> : null}
						{!row.original.enabled ? <Badge variant="outline">已停用</Badge> : null}
					</span>
				),
			},
			{ accessorKey: 'key', header: 'Key', size: 90 },
			{ accessorKey: 'min_xp', header: 'XP 门槛', size: 90 },
			{ accessorKey: 'window_days', header: '保级窗口', size: 90 },
			{
				id: 'retention',
				header: '保级条件',
				cell: ({ row }) => {
					const l = row.original;
					if (l.window_days <= 0) return <span className="text-muted-foreground">无</span>;
					const parts = [
						l.min_active_days > 0 ? `活跃 ≥${l.min_active_days} 天` : '',
						l.min_posts > 0 ? `发帖 ≥${l.min_posts}` : '',
						l.min_likes > 0 ? `获赞 ≥${l.min_likes}` : '',
						l.max_reports !== null ? `成立举报 ≤${l.max_reports}` : '',
					].filter(Boolean);
					return <span className="text-[13px]">{parts.length ? parts.join(' · ') : '仅窗口'}</span>;
				},
			},
			{
				id: 'actions',
				header: '操作',
				size: 140,
				cell: ({ row }) => (
					<div className="flex items-center gap-1.5">
						<Button variant="outline" size="xs" onClick={() => openEdit(row.original)}>
							编辑
						</Button>
						<Button
							variant="destructive"
							size="xs"
							disabled={row.original.level === 0}
							onClick={() => remove(row.original)}
						>
							删除
						</Button>
					</div>
				),
			},
		],
		[openEdit, remove],
	);

	return (
		<div>
			<PageHeader
				title="等级定义"
				actions={
					<div className="flex flex-wrap items-center gap-2">
						<Button variant="outline" size="sm" loading={recompute.isPending} onClick={() => recompute.mutate()}>
							<RefreshCw className="size-3.5" />
							全量重算
						</Button>
						<Button size="sm" onClick={openCreate}>
							<Plus className="size-3.5" />
							新增等级
						</Button>
					</div>
				}
			/>

			<p className="mb-3 text-[13px] text-muted-foreground">
				等级 = 累计 XP 定级 + 滚动窗口保级。保级窗口为 0 表示该等级没有保级条件；「仅人工」的等级不参与自动升降。
			</p>

			<DataTable
				columns={columns}
				data={levels}
				loading={isPending}
				getRowId={(row) => String(row.level)}
				emptyText="还没有等级定义"
			/>

			<Dialog open={open} onOpenChange={setOpen}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>{editing ? '编辑等级' : '新增等级'}</DialogTitle>
					</DialogHeader>
					{draft ? (
						<div className="grid grid-cols-2 gap-3">
							<div className="flex flex-col gap-1.5">
								<Label htmlFor="lv-level">等级编号</Label>
								<Input
									id="lv-level"
									type="number"
									value={draft.level}
									disabled={editing}
									onChange={(e) => setDraft({ ...draft, level: Number(e.target.value) })}
								/>
							</div>
							<div className="flex flex-col gap-1.5">
								<Label htmlFor="lv-key">Key</Label>
								<Input id="lv-key" value={draft.key} onChange={(e) => setDraft({ ...draft, key: e.target.value })} />
							</div>
							<div className="col-span-2 flex flex-col gap-1.5">
								<Label htmlFor="lv-name">名称（可改，前端展示用）</Label>
								<Input id="lv-name" value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} />
							</div>
							<div className="flex flex-col gap-1.5">
								<Label htmlFor="lv-minxp">XP 门槛</Label>
								<Input
									id="lv-minxp"
									type="number"
									value={draft.min_xp}
									onChange={(e) => setDraft({ ...draft, min_xp: Number(e.target.value) })}
								/>
							</div>
							<div className="flex flex-col gap-1.5">
								<Label htmlFor="lv-window">保级窗口（天，0=无）</Label>
								<Input
									id="lv-window"
									type="number"
									value={draft.window_days}
									onChange={(e) => setDraft({ ...draft, window_days: Number(e.target.value) })}
								/>
							</div>
							<div className="flex flex-col gap-1.5">
								<Label htmlFor="lv-days">最低活跃天数</Label>
								<Input
									id="lv-days"
									type="number"
									value={draft.min_active_days}
									onChange={(e) => setDraft({ ...draft, min_active_days: Number(e.target.value) })}
								/>
							</div>
							<div className="flex flex-col gap-1.5">
								<Label htmlFor="lv-posts">最低发帖数</Label>
								<Input
									id="lv-posts"
									type="number"
									value={draft.min_posts}
									onChange={(e) => setDraft({ ...draft, min_posts: Number(e.target.value) })}
								/>
							</div>
							<div className="flex flex-col gap-1.5">
								<Label htmlFor="lv-likes">最低获赞数</Label>
								<Input
									id="lv-likes"
									type="number"
									value={draft.min_likes}
									onChange={(e) => setDraft({ ...draft, min_likes: Number(e.target.value) })}
								/>
							</div>
							<div className="flex flex-col gap-1.5">
								<Label htmlFor="lv-reports">成立举报上限（空=不限）</Label>
								<Input
									id="lv-reports"
									type="number"
									value={draft.max_reports ?? ''}
									onChange={(e) =>
										setDraft({
											...draft,
											max_reports: e.target.value === '' ? null : Number(e.target.value),
										})
									}
								/>
							</div>
							<div className="flex items-center justify-between rounded-md border border-border px-3 py-2 text-[13px]">
								<span>仅人工授予</span>
								<Switch checked={draft.manual_only} onCheckedChange={(v) => setDraft({ ...draft, manual_only: v })} />
							</div>
							<div className="flex items-center justify-between rounded-md border border-border px-3 py-2 text-[13px]">
								<span>启用</span>
								<Switch checked={draft.enabled} onCheckedChange={(v) => setDraft({ ...draft, enabled: v })} />
							</div>
						</div>
					) : null}
					<DialogFooter className="mt-2 gap-2">
						<Button variant="outline" size="sm" onClick={() => setOpen(false)}>
							取消
						</Button>
						<Button size="sm" loading={saving} onClick={submit}>
							保存
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			{confirmDialog}
		</div>
	);
}
