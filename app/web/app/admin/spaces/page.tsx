'use client';

import { type Space as SpaceItem, type SpaceNode, spaceService } from '@feedchamber/api-client';
import { formatTime } from '@feedchamber/shared-utils';
import { zodResolver } from '@hookform/resolvers/zod';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { SpaceIcon } from '~/components/admin/SpaceIcon';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable } from '~/components/ui/data-table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '~/components/ui/form';
import { Input } from '~/components/ui/input';
import { PageHeader } from '~/components/ui/page-header';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Textarea } from '~/components/ui/textarea';
import { type TreeNode, TreeSelect } from '~/components/ui/tree-select';
import { Lightbulb as Bulb, Plus, RefreshCw } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';
import SpacePermissionModal from './permissions';

function flattenTree(nodes: SpaceNode[]): SpaceItem[] {
	const out: SpaceItem[] = [];
	const walk = (arr: SpaceNode[]) => {
		for (const n of arr) {
			const { children, ...rest } = n;
			out.push(rest as SpaceItem);
			if (children && children.length > 0) walk(children);
		}
	};
	walk(nodes);
	return out;
}

function collectDescendantIds(nodes: SpaceNode[], targetId: string): Set<string> {
	const ids = new Set<string>();
	let found: SpaceNode | null = null;
	const find = (arr: SpaceNode[]): boolean => {
		for (const n of arr) {
			if (n.id === targetId) {
				found = n;
				return true;
			}
			if (n.children && n.children.length > 0 && find(n.children)) return true;
		}
		return false;
	};
	find(nodes);
	if (!found) return ids;
	const dfs = (n: SpaceNode) => {
		ids.add(n.id);
		if (n.children) for (const c of n.children) dfs(c);
	};
	dfs(found);
	return ids;
}

function buildTreeData(nodes: SpaceNode[], excludeId?: string): TreeNode[] {
	const excludeSet = excludeId ? collectDescendantIds(nodes, excludeId) : new Set<string>();
	const walk = (arr: SpaceNode[]): TreeNode[] =>
		arr
			.filter((n) => !excludeSet.has(n.id))
			.map((n) => ({
				value: n.id,
				title: `${n.name} /${n.slug}${n.level !== undefined ? ` Lv.${n.level}` : ''}`,
				children: n.children && n.children.length > 0 ? walk(n.children) : undefined,
			}));
	return [{ value: '', title: '— 根目录（大区）—' }, ...walk(nodes)];
}

const spaceSchema = z.object({
	name: z.string().min(1, '请输入版面名称'),
	slug: z
		.string()
		.min(1, '请输入唯一标识')
		.regex(/^[a-z0-9_-]+$/, '仅小写字母数字_-'),
	parent_id: z.string().optional(),
	icon: z.string().max(16, '最多 16 个字符').optional(),
	description: z.string().optional(),
	sort_order: z
		.string()
		.optional()
		.refine((v) => !v || /^\d+$/.test(v), '请输入非负整数'),
	visibility: z.enum(['public', 'login', 'level']),
	min_level: z
		.string()
		.optional()
		.refine((v) => !v || /^\d+$/.test(v), '请输入非负整数'),
});

type Values = z.infer<typeof spaceSchema>;

const EMPTY: Values = {
	name: '',
	slug: '',
	parent_id: '',
	icon: '',
	description: '',
	sort_order: '0',
	visibility: 'public',
	min_level: '0',
};

export default function SpaceList() {
	const queryClient = useQueryClient();

	const {
		data: tree = [],
		isPending: loading,
		refetch,
	} = useQuery({
		queryKey: qk.admin.spaces.list(),
		queryFn: async () => {
			const res = await spaceService.tree();
			return res ?? [];
		},
	});

	/** 写操作后让该模块的列表缓存失效。 */
	const invalidate = useCallback(() => queryClient.invalidateQueries({ queryKey: qk.admin.spaces.all }), [queryClient]);
	const [modalOpen, setModalOpen] = useState(false);
	const [editingSpace, setEditingSpace] = useState<SpaceItem | null>(null);
	const [parentForCreate, setParentForCreate] = useState<string>('');
	const [submitting, setSubmitting] = useState(false);
	const [permModalOpen, setPermModalOpen] = useState(false);
	const [permSpace, setPermSpace] = useState<SpaceItem | null>(null);
	const [confirm, confirmDialog] = useConfirm();

	const isEdit = editingSpace !== null;
	const form = useForm<Values>({ resolver: zodResolver(spaceSchema), defaultValues: EMPTY });

	const flat = useMemo(() => flattenTree(tree), [tree]);

	const openCreate = useCallback(
		(parentId = '') => {
			setEditingSpace(null);
			setParentForCreate(parentId);
			form.reset({ ...EMPTY, parent_id: parentId });
			setModalOpen(true);
		},
		[form],
	);

	const openEdit = useCallback(
		(record: SpaceItem) => {
			setEditingSpace(record);
			setParentForCreate(record.parent_id || '');
			form.reset({
				name: record.name,
				slug: record.slug,
				description: record.description ?? '',
				icon: (record as any).icon || '',
				parent_id: record.parent_id || '',
				sort_order: String((record as any).sort_order ?? 0),
				visibility: (record.visibility as Values['visibility']) || 'public',
				min_level: String(record.min_level ?? 0),
			});
			setModalOpen(true);
		},
		[form],
	);

	const handleSubmit = useCallback(
		async (values: Values) => {
			setSubmitting(true);
			try {
				const payload: any = {
					name: values.name,
					slug: values.slug,
					description: values.description || '',
					icon: values.icon || '',
					parent_id: values.parent_id || '',
					sort_order: values.sort_order ? Number(values.sort_order) : 0,
					visibility: values.visibility,
					min_level: values.min_level ? Number(values.min_level) : 0,
				};
				if (isEdit && editingSpace) {
					await spaceService.update(editingSpace.id, { ...payload, is_active: (editingSpace as any).is_active } as any);
					toast.success('更新成功');
				} else {
					await spaceService.create(payload);
					toast.success('创建成功 — 已支持无限嵌套子版面');
				}
				setModalOpen(false);
				setEditingSpace(null);
				form.reset(EMPTY);
				invalidate();
			} catch (e: any) {
				toast.error(e?.msg || e.message || '操作失败');
			} finally {
				setSubmitting(false);
			}
		},
		[editingSpace, form, invalidate, isEdit],
	);

	const handleDelete = useCallback(
		async (id: string) => {
			try {
				await spaceService.delete(id);
				toast.success('删除成功');
				invalidate();
			} catch (e: any) {
				toast.error(e?.msg || e.message || '删除失败 — 请先清理子版面与帖子');
			}
		},
		[invalidate],
	);

	const treeData = useMemo(() => buildTreeData(tree, editingSpace?.id), [tree, editingSpace?.id]);

	const columns = useMemo<ColumnDef<SpaceItem>[]>(
		() => [
			{
				accessorKey: 'name',
				header: '版面',
				size: 280,
				cell: ({ row }) => {
					const record = row.original as any;
					const level = record.level ?? 0;
					const hasChildren = record.has_children || record.children?.length > 0;
					return (
						<div className="flex items-center gap-2" style={{ paddingLeft: level * 16 }}>
							<SpaceIcon icon={record.icon} level={level} />
							<strong className={level === 0 ? 'font-extrabold' : 'font-semibold'}>{row.original.name}</strong>
							<Badge variant={level === 0 ? 'primary' : level === 1 ? 'secondary' : 'outline'}>Lv.{level}</Badge>
							{hasChildren ? (
								<Badge variant="success">{record.children_count ?? record.children?.length ?? 0} 子版</Badge>
							) : null}
						</div>
					);
				},
			},
			{
				accessorKey: 'slug',
				header: 'Slug',
				size: 130,
				cell: ({ getValue }) => <span className="font-mono text-xs">{getValue<string>()}</span>,
			},
			{
				accessorKey: 'parent_id',
				header: '父级',
				size: 100,
				cell: ({ getValue }) => {
					const v = getValue<string>();
					return v ? (
						<span className="font-mono text-[11px]">{flat.find((f) => f.id === v)?.name || v.slice(0, 6)}</span>
					) : (
						<Badge variant="outline">根</Badge>
					);
				},
			},
			{
				accessorKey: 'visibility',
				header: '可见性',
				size: 110,
				cell: ({ row }) => {
					const v = row.original.visibility || 'public';
					const label = v === 'login' ? '需登录' : v === 'level' ? `等级 ≥${row.original.min_level ?? 0}` : '公开';
					const variant = v === 'public' ? 'outline' : v === 'login' ? 'secondary' : 'warning';
					return <Badge variant={variant as any}>{label}</Badge>;
				},
			},
			{ accessorKey: 'sort_order', header: '排序', size: 70 },
			{
				accessorKey: 'created_at',
				header: '创建时间',
				size: 160,
				cell: ({ getValue }) => formatTime(getValue<string>()),
			},
			{
				id: 'actions',
				header: '操作',
				size: 320,
				cell: ({ row }) => {
					const record = row.original;
					return (
						<div className="flex flex-wrap items-center gap-1.5">
							<Button variant="outline" size="xs" onClick={() => openCreate(record.id)} title="创建子版面">
								+子版
							</Button>
							<Button variant="outline" size="xs" onClick={() => openEdit(record)}>
								编辑
							</Button>
							<Button
								variant="outline"
								size="xs"
								onClick={() => {
									setPermSpace(record);
									setPermModalOpen(true);
								}}
							>
								权限
							</Button>
							<Button
								variant="destructive"
								size="xs"
								onClick={() =>
									confirm({
										title: '删除该版面？',
										description: '若有子版面或帖子，删除会失败——请先清理。',
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
		[confirm, flat, openCreate, openEdit, handleDelete],
	);

	return (
		<div>
			<PageHeader
				title={
					<span className="flex items-baseline gap-2">
						版面管理
						<span className="text-muted-foreground text-xs font-normal">/ 无限嵌套子版面</span>
					</span>
				}
				meta={`${flat.length} 个版面 · ${tree.length} 个大区`}
				actions={
					<div className="flex items-center gap-2">
						<Button size="sm" onClick={() => openCreate('')}>
							<Plus className="size-3.5" />
							新建大区
						</Button>
						<Button variant="outline" size="sm" onClick={() => openCreate()}>
							新建版面
						</Button>
						<Button variant="outline" size="sm" onClick={() => refetch()}>
							<RefreshCw className="size-3.5" />
							刷新
						</Button>
					</div>
				}
			/>

			<p className="text-muted-foreground mb-3 text-[11px]">支持子版面 → 子子版面无限层级 · 层级深度无限</p>

			<DataTable
				columns={columns}
				data={tree as unknown as SpaceItem[]}
				loading={loading}
				getRowId={(row: any) => row.id}
			/>

			<Dialog open={modalOpen} onOpenChange={(v) => !submitting && setModalOpen(v)}>
				<DialogContent className="max-w-[560px]">
					<DialogHeader>
						<DialogTitle>{isEdit ? '编辑版面' : parentForCreate ? '新建子版面' : '新建版面'}</DialogTitle>
					</DialogHeader>
					<Form {...form}>
						<form onSubmit={form.handleSubmit(handleSubmit)} className="flex flex-col gap-3">
							<FormField
								control={form.control}
								name="name"
								render={({ field }) => (
									<FormItem>
										<FormLabel>版面名称</FormLabel>
										<FormControl>
											<Input placeholder="例如：艾泽拉斯议事厅 / 技术讨论 / 子版 A" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="slug"
								render={({ field }) => (
									<FormItem>
										<FormLabel>标识 (slug)</FormLabel>
										<FormControl>
											<Input placeholder="例如：tech / tech-frontend / wow-lore" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="parent_id"
								render={({ field }) => (
									<FormItem>
										<FormLabel>父版面（决定层级，空为大区）</FormLabel>
										<FormControl>
											<TreeSelect
												value={field.value}
												onChange={field.onChange}
												treeData={treeData}
												placeholder="选择父版面，空为根大区"
											/>
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="icon"
								render={({ field }) => (
									<FormItem>
										<FormLabel>图标（可选）</FormLabel>
										<FormControl>
											<Input placeholder="例如：laptop / message / server" maxLength={16} {...field} />
										</FormControl>
										<FormDescription>支持图标名或历史 emoji，留空按层级自动</FormDescription>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="description"
								render={({ field }) => (
									<FormItem>
										<FormLabel>描述</FormLabel>
										<FormControl>
											<Textarea rows={3} placeholder="请输入版面描述（可选）" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="visibility"
								render={({ field }) => (
									<FormItem>
										<FormLabel>可见性策略</FormLabel>
										<Select value={field.value} onValueChange={field.onChange}>
											<SelectTrigger>
												<SelectValue />
											</SelectTrigger>
											<SelectContent>
												<SelectItem value="public">公开（匿名可读）</SelectItem>
												<SelectItem value="login">需登录</SelectItem>
												<SelectItem value="level">需等级</SelectItem>
											</SelectContent>
										</Select>
										<FormDescription>子版面继承祖先策略，取整条链上最严格的一档</FormDescription>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="min_level"
								render={({ field }) => (
									<FormItem>
										<FormLabel>最低等级（仅「需等级」生效）</FormLabel>
										<FormControl>
											<Input type="number" min={0} placeholder="0" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="sort_order"
								render={({ field }) => (
									<FormItem>
										<FormLabel>排序值（越小越前）</FormLabel>
										<FormControl>
											<Input type="number" min={0} max={9999} placeholder="0" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<div className="bg-muted text-muted-foreground rounded-md p-2 text-[11px]">
								<Bulb className="mr-1.5 inline size-3.5" />
								NGA 提示：可为任意版面创建子版面，子版面下可再创建子子版面，无限嵌套。例：大区「游戏」→
								版面「魔兽世界」→ 子版「剧情讨论」→ 子子版「暗影国度」。
							</div>
							<DialogFooter className="mt-2 gap-2">
								<Button type="button" variant="outline" size="sm" onClick={() => setModalOpen(false)}>
									取消
								</Button>
								<Button type="submit" size="sm" loading={submitting}>
									保存
								</Button>
							</DialogFooter>
						</form>
					</Form>
				</DialogContent>
			</Dialog>

			<SpacePermissionModal
				spaceId={permSpace?.id || ''}
				spaceName={permSpace?.name || ''}
				open={permModalOpen}
				onClose={() => {
					setPermModalOpen(false);
					setPermSpace(null);
				}}
			/>

			{confirmDialog}
		</div>
	);
}
