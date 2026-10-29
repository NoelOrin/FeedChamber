'use client';

import { trustService, type XPRule, type XPStreakBonus } from '@feedchamber/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
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
import { Plus } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

/* XP 规则：指标 → XP、自然日上限；以及连续签到里程碑阶梯。
   改分值后下一次求值即对全员生效（XP 是行为聚合，不是累加计数器）。 */
export default function XPRulesPage() {
	const queryClient = useQueryClient();
	const [ruleDraft, setRuleDraft] = useState<XPRule | null>(null);
	const [bonusDraft, setBonusDraft] = useState<XPStreakBonus | null>(null);
	const [saving, setSaving] = useState(false);
	const [confirm, confirmDialog] = useConfirm();

	const { data: rules = [], isPending: rulesLoading } = useQuery({
		queryKey: qk.admin.trust.xpRules,
		queryFn: () => trustService.listXPRules(),
	});
	const { data: bonuses = [], isPending: bonusesLoading } = useQuery({
		queryKey: qk.admin.trust.streak,
		queryFn: () => trustService.listStreakBonuses(),
	});

	const invalidate = useCallback(
		() =>
			Promise.all([
				queryClient.invalidateQueries({ queryKey: qk.admin.trust.xpRules }),
				queryClient.invalidateQueries({ queryKey: qk.admin.trust.streak }),
				queryClient.invalidateQueries({ queryKey: qk.admin.users.all }),
			]),
		[queryClient],
	);

	async function saveRule() {
		if (!ruleDraft) return;
		setSaving(true);
		try {
			await trustService.saveXPRule(ruleDraft);
			toast.success('保存成功');
			setRuleDraft(null);
			invalidate();
		} catch (e: any) {
			toast.error(e?.message || '保存失败');
		} finally {
			setSaving(false);
		}
	}

	async function saveBonus() {
		if (!bonusDraft) return;
		setSaving(true);
		try {
			await trustService.saveStreakBonus(bonusDraft);
			toast.success('保存成功');
			setBonusDraft(null);
			invalidate();
		} catch (e: any) {
			toast.error(e?.message || '保存失败');
		} finally {
			setSaving(false);
		}
	}

	const removeBonus = useCallback(
		(days: number) => {
			confirm({
				title: `删除第 ${days} 天里程碑？`,
				description: '删除后该档奖励不再计入 XP；已经获得的 XP 会在下一次求值时回收。',
				danger: true,
				confirmText: '删除',
				onConfirm: async () => {
					try {
						await trustService.deleteStreakBonus(days);
						toast.success('删除成功');
						invalidate();
					} catch (e: any) {
						toast.error(e?.message || '删除失败');
					}
				},
			});
		},
		[confirm, invalidate],
	);

	const ruleColumns = useMemo<ColumnDef<XPRule>[]>(
		() => [
			{ accessorKey: 'label', header: '指标', cell: ({ row }) => row.original.label || row.original.metric },
			{ accessorKey: 'metric', header: '指标标识', size: 190 },
			{ accessorKey: 'delta', header: '单次 XP', size: 90 },
			{
				accessorKey: 'daily_cap',
				header: '每日上限（条）',
				size: 120,
				cell: ({ getValue }) => (getValue<number>() > 0 ? getValue<number>() : '不限'),
			},
			{
				accessorKey: 'enabled',
				header: '启用',
				size: 70,
				cell: ({ getValue }) =>
					getValue<boolean>() ? <Badge variant="primary">是</Badge> : <Badge variant="outline">否</Badge>,
			},
			{
				id: 'actions',
				header: '操作',
				size: 90,
				cell: ({ row }) => (
					<Button variant="outline" size="xs" onClick={() => setRuleDraft({ ...row.original })}>
						编辑
					</Button>
				),
			},
		],
		[],
	);

	const bonusColumns = useMemo<ColumnDef<XPStreakBonus>[]>(
		() => [
			{ accessorKey: 'days', header: '连续天数', size: 100 },
			{ accessorKey: 'bonus', header: '奖励 XP', size: 100 },
			{
				accessorKey: 'enabled',
				header: '启用',
				size: 70,
				cell: ({ getValue }) =>
					getValue<boolean>() ? <Badge variant="primary">是</Badge> : <Badge variant="outline">否</Badge>,
			},
			{
				id: 'actions',
				header: '操作',
				size: 150,
				cell: ({ row }) => (
					<div className="flex items-center gap-1.5">
						<Button variant="outline" size="xs" onClick={() => setBonusDraft({ ...row.original })}>
							编辑
						</Button>
						<Button variant="destructive" size="xs" onClick={() => removeBonus(row.original.days)}>
							删除
						</Button>
					</div>
				),
			},
		],
		[removeBonus],
	);

	return (
		<div className="flex flex-col gap-6">
			<PageHeader title="XP 规则" />

			<section>
				<div className="mb-2 flex items-center justify-between">
					<div>
						<h2 className="text-sm font-medium text-foreground">行为指标</h2>
						<p className="text-[13px] text-muted-foreground">
							每日上限按「自然日 × 指标」截断；0 表示不限。只有通过审核且未删除的内容计入。
						</p>
					</div>
				</div>
				<DataTable
					columns={ruleColumns}
					data={rules}
					loading={rulesLoading}
					getRowId={(row) => row.metric}
					emptyText="还没有 XP 规则"
				/>
			</section>

			<section>
				<div className="mb-2 flex items-center justify-between">
					<div>
						<h2 className="text-sm font-medium text-foreground">连续签到阶梯</h2>
						<p className="text-[13px] text-muted-foreground">
							连续天数恰好达到里程碑时一次性奖励；断签重来可再次获得。
						</p>
					</div>
					<Button size="sm" onClick={() => setBonusDraft({ days: 0, bonus: 0, enabled: true })}>
						<Plus className="size-3.5" />
						新增阶梯
					</Button>
				</div>
				<DataTable
					columns={bonusColumns}
					data={bonuses}
					loading={bonusesLoading}
					getRowId={(row) => String(row.days)}
					emptyText="还没有里程碑"
				/>
			</section>

			<Dialog open={Boolean(ruleDraft)} onOpenChange={(v) => !v && setRuleDraft(null)}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>编辑 XP 规则</DialogTitle>
					</DialogHeader>
					{ruleDraft ? (
						<div className="flex flex-col gap-3">
							<p className="text-[13px] text-muted-foreground">
								{ruleDraft.label || ruleDraft.metric}
								<code className="ml-2 rounded bg-muted px-1.5 py-0.5 text-[12px]">{ruleDraft.metric}</code>
							</p>
							<div className="grid grid-cols-2 gap-3">
								<div className="flex flex-col gap-1.5">
									<Label htmlFor="rule-delta">单次 XP</Label>
									<Input
										id="rule-delta"
										type="number"
										value={ruleDraft.delta}
										onChange={(e) => setRuleDraft({ ...ruleDraft, delta: Number(e.target.value) })}
									/>
								</div>
								<div className="flex flex-col gap-1.5">
									<Label htmlFor="rule-cap">每日上限（0=不限）</Label>
									<Input
										id="rule-cap"
										type="number"
										value={ruleDraft.daily_cap}
										onChange={(e) => setRuleDraft({ ...ruleDraft, daily_cap: Number(e.target.value) })}
									/>
								</div>
							</div>
							<div className="flex items-center justify-between rounded-md border border-border px-3 py-2 text-[13px]">
								<span>启用该规则</span>
								<Switch
									checked={ruleDraft.enabled}
									onCheckedChange={(v) => setRuleDraft({ ...ruleDraft, enabled: v })}
								/>
							</div>
							<p className="text-[12px] text-muted-foreground">
								改分值后下一次求值即对全员生效；可在「等级定义」页点「全量重算」立即刷新所有用户快照。
							</p>
						</div>
					) : null}
					<DialogFooter className="mt-2 gap-2">
						<Button variant="outline" size="sm" onClick={() => setRuleDraft(null)}>
							取消
						</Button>
						<Button size="sm" loading={saving} onClick={saveRule}>
							保存
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			<Dialog open={Boolean(bonusDraft)} onOpenChange={(v) => !v && setBonusDraft(null)}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>里程碑</DialogTitle>
					</DialogHeader>
					{bonusDraft ? (
						<div className="flex flex-col gap-3">
							<div className="grid grid-cols-2 gap-3">
								<div className="flex flex-col gap-1.5">
									<Label htmlFor="bonus-days">连续天数</Label>
									<Input
										id="bonus-days"
										type="number"
										value={bonusDraft.days}
										onChange={(e) => setBonusDraft({ ...bonusDraft, days: Number(e.target.value) })}
									/>
								</div>
								<div className="flex flex-col gap-1.5">
									<Label htmlFor="bonus-value">奖励 XP</Label>
									<Input
										id="bonus-value"
										type="number"
										value={bonusDraft.bonus}
										onChange={(e) => setBonusDraft({ ...bonusDraft, bonus: Number(e.target.value) })}
									/>
								</div>
							</div>
							<div className="flex items-center justify-between rounded-md border border-border px-3 py-2 text-[13px]">
								<span>启用该阶梯</span>
								<Switch
									checked={bonusDraft.enabled}
									onCheckedChange={(v) => setBonusDraft({ ...bonusDraft, enabled: v })}
								/>
							</div>
						</div>
					) : null}
					<DialogFooter className="mt-2 gap-2">
						<Button variant="outline" size="sm" onClick={() => setBonusDraft(null)}>
							取消
						</Button>
						<Button size="sm" loading={saving} onClick={saveBonus}>
							保存
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			{confirmDialog}
		</div>
	);
}
