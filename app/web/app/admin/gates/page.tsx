'use client';

import {
	type GateCondition,
	type GateDecision,
	type GateDomainDefault,
	type GateMeta,
	type GateRule,
	gateService,
} from '@orinforum/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable } from '~/components/ui/data-table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { PageHeader } from '~/components/ui/page-header';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Switch } from '~/components/ui/switch';
import { Textarea } from '~/components/ui/textarea';
import { Plus } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

type CondRow = { key: string; field: string; op: string; value: string; window_days?: number };

function newCondRow(field = 'level', op = 'eq', value = '0'): CondRow {
	return { key: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`, field, op, value };
}

const BOOL_FIELDS = new Set([
	'is_author',
	'is_anonymous',
	'email_verified',
	'banned',
	'exiled',
	'contains_external_link',
]);
const NUM_FIELDS = new Set(['level', 'xp', 'author_level']);

const ACTION_LABEL: Record<string, string> = {
	allow: '放行',
	deny: '拒绝',
	require_review: '转待审',
	rate_limit: '限流',
};

function emptyRule(): GateRule {
	return {
		id: '',
		priority: 100,
		enabled: true,
		domain: 'post_create',
		conditions: '[]',
		action: 'deny',
		action_params: '{}',
		note: '',
	};
}

function parseConditions(raw: string): CondRow[] {
	try {
		const list = JSON.parse(raw) as GateCondition[];
		return list.map((c, i) => ({
			key: `c-${i}-${c.field}`,
			field: c.field,
			op: c.op,
			value: Array.isArray(c.value) ? (c.value as unknown[]).join(',') : String(c.value ?? ''),
			window_days: c.window_days,
		}));
	} catch {
		return [];
	}
}

function serializeConditions(rows: CondRow[]): string {
	return JSON.stringify(
		rows.map((r) => {
			const base: GateCondition = { field: r.field, op: r.op, value: r.value };
			if (r.field === 'window_metric') base.value = r.value;
			else if (BOOL_FIELDS.has(r.field)) base.value = r.value === 'true';
			else if (NUM_FIELDS.has(r.field)) base.value = Number(r.value);
			else if (r.op === 'in')
				base.value = r.value
					.split(',')
					.map((s) => s.trim())
					.filter(Boolean);
			if (r.window_days && r.field === 'window_metric') base.window_days = r.window_days;
			return base;
		}),
	);
}

function conditionsSummary(raw: string, meta?: GateMeta): string {
	const rows = parseConditions(raw);
	if (rows.length === 0) return '—';
	return rows.map((r) => `${meta?.fields[r.field] ?? r.field} ${r.op} ${r.value || '(空)'}`).join(' 且 ');
}

export default function GatesPage() {
	const queryClient = useQueryClient();
	const [confirm, confirmDialog] = useConfirm();
	const [editing, setEditing] = useState<GateRule | null>(null);
	const [rows, setRows] = useState<CondRow[]>([]);
	const [saving, setSaving] = useState(false);
	const [simulateOpen, setSimulateOpen] = useState(false);
	const [simUserId, setSimUserId] = useState('');
	const [simDomain, setSimDomain] = useState('post_create');
	const [simContent, setSimContent] = useState('');
	const [simIsAuthor, setSimIsAuthor] = useState(false);
	const [decision, setDecision] = useState<GateDecision | null>(null);

	const { data: meta } = useQuery({ queryKey: qk.admin.gates.meta, queryFn: () => gateService.meta() });
	const { data: rules = [], isPending } = useQuery({
		queryKey: qk.admin.gates.rules,
		queryFn: () => gateService.listRules(),
	});
	const { data: defaults = [] } = useQuery({
		queryKey: qk.admin.gates.defaults,
		queryFn: () => gateService.listDefaults(),
	});

	const invalidate = useCallback(
		() =>
			Promise.all([
				queryClient.invalidateQueries({ queryKey: qk.admin.gates.rules }),
				queryClient.invalidateQueries({ queryKey: qk.admin.gates.defaults }),
			]),
		[queryClient],
	);

	function openCreate() {
		const rule = emptyRule();
		setEditing(rule);
		setRows([newCondRow()]);
	}

	const openEdit = useCallback((rule: GateRule) => {
		setEditing({ ...rule });
		setRows(parseConditions(rule.conditions));
	}, []);

	async function saveRule() {
		if (!editing) return;
		if (rows.length === 0) {
			toast.error('至少需要一个条件');
			return;
		}
		setSaving(true);
		try {
			await gateService.saveRule({ ...editing, conditions: serializeConditions(rows) });
			toast.success('保存成功');
			setEditing(null);
			invalidate();
		} catch (e: any) {
			toast.error(e?.message || '保存失败');
		} finally {
			setSaving(false);
		}
	}

	const removeRule = useCallback(
		(rule: GateRule) => {
			confirm({
				title: `删除规则 ${rule.id}？`,
				description: '删除后该规则不再参与求值；无其它规则命中时走能力域默认动作。',
				danger: true,
				confirmText: '删除',
				onConfirm: async () => {
					try {
						await gateService.deleteRule(rule.id);
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

	const saveDefault = useMutation({
		mutationFn: ({ domain, action }: { domain: string; action: string }) => gateService.saveDefault(domain, action),
		onSuccess: () => {
			toast.success('已更新默认动作');
			invalidate();
		},
		onError: (e: Error) => toast.error(e.message || '更新失败'),
	});

	const simulate = useMutation({
		mutationFn: () =>
			gateService.simulate({
				user_id: simUserId,
				domain: simDomain,
				content: simContent,
				is_author: simIsAuthor,
			}),
		onSuccess: (d) => setDecision(d),
		onError: (e: Error) => toast.error(e.message || '试算失败'),
	});

	const columns = useMemo<ColumnDef<GateRule>[]>(
		() => [
			{ accessorKey: 'priority', header: '优先级', size: 80 },
			{
				accessorKey: 'domain',
				header: '能力域',
				size: 110,
				cell: ({ getValue }) => meta?.domains[getValue<string>()] ?? getValue<string>(),
			},
			{
				accessorKey: 'conditions',
				header: '条件',
				cell: ({ getValue }) => (
					<span className="text-[13px]">{conditionsSummary(getValue<string>(), meta as GateMeta)}</span>
				),
			},
			{
				accessorKey: 'action',
				header: '动作',
				size: 100,
				cell: ({ getValue }) => {
					const action = getValue<string>();
					const variant = action === 'deny' ? 'destructive' : action === 'allow' ? 'primary' : 'warning';
					return <Badge variant={variant}>{ACTION_LABEL[action] ?? action}</Badge>;
				},
			},
			{
				accessorKey: 'enabled',
				header: '启用',
				size: 70,
				cell: ({ getValue }) =>
					getValue<boolean>() ? <Badge variant="primary">是</Badge> : <Badge variant="outline">否</Badge>,
			},
			{ accessorKey: 'note', header: '说明', size: 200 },
			{
				id: 'actions',
				header: '操作',
				size: 140,
				cell: ({ row }) => (
					<div className="flex items-center gap-1.5">
						<Button variant="outline" size="xs" onClick={() => openEdit(row.original)}>
							编辑
						</Button>
						<Button variant="destructive" size="xs" onClick={() => removeRule(row.original)}>
							删除
						</Button>
					</div>
				),
			},
		],
		[meta, openEdit, removeRule],
	);

	const defaultsColumns = useMemo<ColumnDef<GateDomainDefault>[]>(
		() => [
			{
				accessorKey: 'domain',
				header: '能力域',
				cell: ({ getValue }) => meta?.domains[getValue<string>()] ?? getValue<string>(),
			},
			{
				accessorKey: 'default_action',
				header: '默认动作',
				size: 160,
				cell: ({ row }) => (
					<Select
						value={row.original.default_action}
						onValueChange={(v) => saveDefault.mutate({ domain: row.original.domain, action: v })}
					>
						<SelectTrigger className="h-8 w-[140px]">
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							{(meta?.actions ?? ['allow', 'deny', 'require_review', 'rate_limit']).map((a) => (
								<SelectItem key={a} value={a}>
									{ACTION_LABEL[a] ?? a}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				),
			},
			{
				id: 'hint',
				header: '说明',
				cell: ({ row }) =>
					row.original.domain === 'external_link'
						? '默认拒绝时需显式 allow 规则放行，否则所有外链都会被拦'
						: '没有任何规则命中时执行该动作',
			},
		],
		[meta, saveDefault],
	);

	return (
		<div className="flex flex-col gap-6">
			<PageHeader
				title="门控规则"
				actions={
					<div className="flex items-center gap-2">
						<Button variant="outline" size="sm" onClick={() => setSimulateOpen(true)}>
							以用户试算
						</Button>
						<Button size="sm" onClick={openCreate}>
							<Plus className="size-3.5" />
							新增规则
						</Button>
					</div>
				}
			/>

			<section>
				<h2 className="mb-2 text-sm font-medium">规则（优先级降序，首个命中生效）</h2>
				<DataTable
					columns={columns}
					data={rules}
					loading={isPending}
					getRowId={(row) => row.id}
					emptyText="还没有规则"
				/>
			</section>

			<section>
				<h2 className="mb-2 text-sm font-medium">能力域默认动作</h2>
				<DataTable columns={defaultsColumns} data={defaults} loading={false} getRowId={(row) => row.domain} />
			</section>

			<Dialog open={Boolean(editing)} onOpenChange={(v) => !v && setEditing(null)}>
				<DialogContent className="max-w-[680px]">
					<DialogHeader>
						<DialogTitle>{editing?.id ? '编辑规则' : '新增规则'}</DialogTitle>
					</DialogHeader>
					{editing ? (
						<div className="flex flex-col gap-3">
							<div className="grid grid-cols-2 gap-3">
								<div className="flex flex-col gap-1.5">
									<Label>能力域</Label>
									<Select value={editing.domain} onValueChange={(v) => setEditing({ ...editing, domain: v })}>
										<SelectTrigger>
											<SelectValue />
										</SelectTrigger>
										<SelectContent>
											{Object.entries(meta?.domains ?? {}).map(([value, label]) => (
												<SelectItem key={value} value={value}>
													{label}
												</SelectItem>
											))}
										</SelectContent>
									</Select>
								</div>
								<div className="flex flex-col gap-1.5">
									<Label htmlFor="gate-priority">优先级（越大越先）</Label>
									<Input
										id="gate-priority"
										type="number"
										value={editing.priority}
										onChange={(e) => setEditing({ ...editing, priority: Number(e.target.value) })}
									/>
								</div>
								<div className="flex flex-col gap-1.5">
									<Label>动作</Label>
									<Select value={editing.action} onValueChange={(v) => setEditing({ ...editing, action: v })}>
										<SelectTrigger>
											<SelectValue />
										</SelectTrigger>
										<SelectContent>
											{(meta?.actions ?? ['allow', 'deny', 'require_review', 'rate_limit']).map((a) => (
												<SelectItem key={a} value={a}>
													{ACTION_LABEL[a] ?? a}
												</SelectItem>
											))}
										</SelectContent>
									</Select>
								</div>
								<div className="flex flex-col gap-1.5">
									<Label htmlFor="gate-note">说明</Label>
									<Input
										id="gate-note"
										value={editing.note}
										onChange={(e) => setEditing({ ...editing, note: e.target.value })}
									/>
								</div>
							</div>

							{editing.action === 'rate_limit' ? (
								<div className="grid grid-cols-2 gap-3">
									<div className="flex flex-col gap-1.5">
										<Label htmlFor="gate-limit">次数上限</Label>
										<Input
											id="gate-limit"
											type="number"
											value={rateParams(editing).limit}
											onChange={(e) =>
												setEditing({
													...editing,
													action_params: JSON.stringify({ ...rateParams(editing), limit: Number(e.target.value) }),
												})
											}
										/>
									</div>
									<div className="flex flex-col gap-1.5">
										<Label htmlFor="gate-window">窗口（秒）</Label>
										<Input
											id="gate-window"
											type="number"
											value={rateParams(editing).window_seconds}
											onChange={(e) =>
												setEditing({
													...editing,
													action_params: JSON.stringify({
														...rateParams(editing),
														window_seconds: Number(e.target.value),
													}),
												})
											}
										/>
									</div>
								</div>
							) : null}

							<div className="flex flex-col gap-2">
								<div className="flex items-center justify-between">
									<Label>条件（全部满足）</Label>
									<Button variant="outline" size="xs" onClick={() => setRows([...rows, newCondRow()])}>
										添加条件
									</Button>
								</div>
								{rows.map((row, idx) => (
									<div key={row.key} className="flex items-end gap-2">
										<div className="flex flex-1 flex-col gap-1">
											<Select
												value={row.field}
												onValueChange={(v) => {
													const next = [...rows];
													next[idx] = { ...row, field: v, op: 'eq', value: '' };
													setRows(next);
												}}
											>
												<SelectTrigger className="h-8">
													<SelectValue />
												</SelectTrigger>
												<SelectContent>
													{Object.entries(meta?.fields ?? {}).map(([value, label]) => (
														<SelectItem key={value} value={value}>
															{label}
														</SelectItem>
													))}
												</SelectContent>
											</Select>
										</div>
										<div className="w-24">
											<Select
												value={row.op}
												onValueChange={(v) => {
													const next = [...rows];
													next[idx] = { ...row, op: v };
													setRows(next);
												}}
											>
												<SelectTrigger className="h-8">
													<SelectValue />
												</SelectTrigger>
												<SelectContent>
													{(meta?.field_ops[row.field] ?? ['eq']).map((op) => (
														<SelectItem key={op} value={op}>
															{op}
														</SelectItem>
													))}
												</SelectContent>
											</Select>
										</div>
										<div className="flex-1">
											{BOOL_FIELDS.has(row.field) ? (
												<Select
													value={row.value || 'true'}
													onValueChange={(v) => {
														const next = [...rows];
														next[idx] = { ...row, value: v };
														setRows(next);
													}}
												>
													<SelectTrigger className="h-8">
														<SelectValue />
													</SelectTrigger>
													<SelectContent>
														<SelectItem value="true">是</SelectItem>
														<SelectItem value="false">否</SelectItem>
													</SelectContent>
												</Select>
											) : (
												<Input
													className="h-8"
													value={row.value}
													placeholder={row.field === 'window_metric' ? '如 active_days:30' : '值 / 逗号分隔'}
													onChange={(e) => {
														const next = [...rows];
														next[idx] = { ...row, value: e.target.value };
														setRows(next);
													}}
												/>
											)}
										</div>
										<Button variant="outline" size="xs" onClick={() => setRows(rows.filter((_, i) => i !== idx))}>
											删除
										</Button>
									</div>
								))}
								{rows.some((r) => r.field === 'window_metric') ? (
									<p className="text-muted-foreground text-[12px]">
										窗口指标填「指标:阈值」，可用：{Object.keys(meta?.window_metric ?? {}).join(' / ')}
									</p>
								) : null}
							</div>

							<div className="flex items-center justify-between rounded-md border border-border px-3 py-2 text-[13px]">
								<span>启用</span>
								<Switch checked={editing.enabled} onCheckedChange={(v) => setEditing({ ...editing, enabled: v })} />
							</div>
						</div>
					) : null}
					<DialogFooter className="mt-2 gap-2">
						<Button variant="outline" size="sm" onClick={() => setEditing(null)}>
							取消
						</Button>
						<Button size="sm" loading={saving} onClick={saveRule}>
							保存
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			<Dialog open={simulateOpen} onOpenChange={setSimulateOpen}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>以用户试算</DialogTitle>
					</DialogHeader>
					<div className="flex flex-col gap-3">
						<div className="flex flex-col gap-1.5">
							<Label htmlFor="sim-user">用户 ID（留空 = 游客）</Label>
							<Input id="sim-user" value={simUserId} onChange={(e) => setSimUserId(e.target.value)} />
						</div>
						<div className="flex flex-col gap-1.5">
							<Label>能力域</Label>
							<Select value={simDomain} onValueChange={setSimDomain}>
								<SelectTrigger>
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									{Object.entries(meta?.domains ?? {}).map(([value, label]) => (
										<SelectItem key={value} value={value}>
											{label}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
						</div>
						<div className="flex flex-col gap-1.5">
							<Label htmlFor="sim-content">内容（用于判断外链）</Label>
							<Textarea id="sim-content" rows={3} value={simContent} onChange={(e) => setSimContent(e.target.value)} />
						</div>
						<div className="flex items-center justify-between rounded-md border border-border px-3 py-2 text-[13px]">
							<span>视为作者本人</span>
							<Switch checked={simIsAuthor} onCheckedChange={setSimIsAuthor} />
						</div>
						{decision ? (
							<Card className="p-3">
								<p className="text-[13px]">
									结果：
									<Badge variant={decision.action === 'deny' ? 'destructive' : 'primary'}>
										{ACTION_LABEL[decision.action] ?? decision.action}
									</Badge>
									{decision.is_default ? (
										<span className="text-muted-foreground ml-2">走域默认动作</span>
									) : (
										<span className="text-muted-foreground ml-2">
											命中规则 {decision.rule_id}（优先级 {decision.priority}）
										</span>
									)}
								</p>
								{decision.note ? <p className="text-muted-foreground mt-1 text-[12px]">{decision.note}</p> : null}
							</Card>
						) : null}
					</div>
					<DialogFooter className="mt-2 gap-2">
						<Button variant="outline" size="sm" onClick={() => setSimulateOpen(false)}>
							关闭
						</Button>
						<Button size="sm" loading={simulate.isPending} onClick={() => simulate.mutate()}>
							试算
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			{confirmDialog}
		</div>
	);
}

function rateParams(rule: GateRule): { limit: number; window_seconds: number } {
	try {
		const parsed = JSON.parse(rule.action_params || '{}') as { limit?: number; window_seconds?: number };
		return { limit: parsed.limit ?? 30, window_seconds: parsed.window_seconds ?? 60 };
	} catch {
		return { limit: 30, window_seconds: 60 };
	}
}
