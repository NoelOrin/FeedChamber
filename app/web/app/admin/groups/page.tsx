'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { type Group, type GroupMember, groupService, type Permission, permissionService } from '@orinforum/api-client';
import { formatTime } from '@orinforum/shared-utils';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Checkbox } from '~/components/ui/checkbox';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable } from '~/components/ui/data-table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '~/components/ui/form';
import { Input } from '~/components/ui/input';
import { PageHeader } from '~/components/ui/page-header';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Plus, RefreshCw } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

const groupSchema = z.object({
	name: z.string().optional(),
	display_name: z.string().optional(),
	visibility: z.string().optional(),
});
type GroupValues = z.infer<typeof groupSchema>;

const memberSchema = z.object({
	user_id: z.string().min(1, '请输入用户 ID'),
	role: z.string().optional(),
});
type MemberValues = z.infer<typeof memberSchema>;

const VISIBILITY = [
	{ value: 'public', label: '公开' },
	{ value: 'closed', label: '关闭' },
	{ value: 'secret', label: '私密' },
];
const MEMBER_ROLES = [
	{ value: 'member', label: 'member' },
	{ value: 'moderator', label: 'moderator' },
	{ value: 'owner', label: 'owner' },
];

export default function GroupsPage() {
	const [modalOpen, setModalOpen] = useState(false);
	const [editing, setEditing] = useState<Group | null>(null);
	const [submitting, setSubmitting] = useState(false);
	const [memberModalOpen, setMemberModalOpen] = useState(false);
	const [activeGroup, setActiveGroup] = useState<Group | null>(null);
	const [members, setMembers] = useState<GroupMember[]>([]);
	const [permModalOpen, setPermModalOpen] = useState(false);
	const [groupPerms, setGroupPerms] = useState<string[]>([]);
	const [permGroup, setPermGroup] = useState<Group | null>(null);
	const [confirm, confirmDialog] = useConfirm();

	const groupForm = useForm<GroupValues>({
		resolver: zodResolver(groupSchema),
		defaultValues: { name: '', display_name: '', visibility: '' },
	});
	const memberForm = useForm<MemberValues>({
		resolver: zodResolver(memberSchema),
		defaultValues: { user_id: '', role: 'member' },
	});

	const queryClient = useQueryClient();

	const {
		data = [],
		isPending: loading,
		refetch,
	} = useQuery({
		queryKey: qk.admin.groups.list,
		queryFn: async () => ((await groupService.list()) as unknown as Group[]) ?? [],
	});

	const { data: permissions = [] } = useQuery({
		queryKey: qk.admin.permissions.list,
		queryFn: async () => ((await permissionService.list()) as unknown as Permission[]) ?? [],
	});

	/** 写操作后让该模块的列表缓存失效。 */
	const invalidate = useCallback(() => queryClient.invalidateQueries({ queryKey: qk.admin.groups.all }), [queryClient]);

	function openCreate() {
		setEditing(null);
		groupForm.reset({ name: '', display_name: '', visibility: '' });
		setModalOpen(true);
	}

	const openEdit = useCallback(
		(r: Group) => {
			setEditing(r);
			groupForm.reset({ display_name: r.display_name ?? '', visibility: r.visibility ?? '' });
			setModalOpen(true);
		},
		[groupForm],
	);

	async function handleSubmit(values: GroupValues) {
		setSubmitting(true);
		try {
			if (editing) {
				await groupService.update(editing.id, values);
			} else {
				await groupService.create({
					name: values.name ?? '',
					display_name: values.display_name ?? '',
					visibility: values.visibility ?? '',
				});
			}
			toast.success(editing ? '更新成功' : '创建成功');
			setModalOpen(false);
			setEditing(null);
			groupForm.reset({ name: '', display_name: '', visibility: '' });
			invalidate();
		} catch (e: any) {
			toast.error(e.message || '操作失败');
		} finally {
			setSubmitting(false);
		}
	}

	const handleDelete = useCallback(
		async (id: string) => {
			try {
				await groupService.delete(id);
				toast.success('删除成功');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '删除失败');
			}
		},
		[invalidate],
	);

	const reloadMembers = useCallback(async (groupId: string) => {
		const res = await groupService.listMembers(groupId, { page: 1, page_size: 20 });
		setMembers((res as any)?.items || (res as any) || []);
	}, []);

	const openMembers = useCallback(
		async (g: Group) => {
			setActiveGroup(g);
			setMemberModalOpen(true);
			try {
				await reloadMembers(g.id);
			} catch (e: any) {
				toast.error(e.message || '加载成员失败');
			}
		},
		[reloadMembers],
	);

	async function handleAddMember(values: MemberValues) {
		if (!activeGroup) return;
		try {
			await groupService.addMember(activeGroup.id, { user_id: values.user_id, role: values.role ?? 'member' });
			toast.success('已添加');
			memberForm.reset({ user_id: '', role: 'member' });
			await reloadMembers(activeGroup.id);
		} catch (e: any) {
			toast.error(e.message || '添加失败');
		}
	}

	const handleRemoveMember = useCallback(
		async (userId: string) => {
			if (!activeGroup) return;
			try {
				await groupService.removeMember(activeGroup.id, { user_id: userId });
				toast.success('已移除');
				await reloadMembers(activeGroup.id);
			} catch (e: any) {
				toast.error(e.message || '移除失败');
			}
		},
		[activeGroup, reloadMembers],
	);

	const openPerms = useCallback(async (g: Group) => {
		setPermGroup(g);
		setPermModalOpen(true);
		try {
			const res = await groupService.getPermissions(g.id);
			setGroupPerms((res as any)?.permissions || []);
		} catch {
			setGroupPerms([]);
		}
	}, []);

	async function handleSavePerms() {
		if (!permGroup) return;
		try {
			await groupService.syncPermissions(permGroup.id, groupPerms);
			toast.success('权限已同步');
			setPermModalOpen(false);
		} catch (e: any) {
			toast.error(e.message || '同步失败');
		}
	}

	const columns = useMemo<ColumnDef<Group>[]>(
		() => [
			{
				accessorKey: 'name',
				header: '名称',
				size: 160,
				cell: ({ row }) => (
					<span className="inline-flex items-center gap-1.5">
						<Badge variant="secondary">{row.original.name}</Badge>
						{row.original.is_system ? <Badge variant="primary">系统</Badge> : null}
					</span>
				),
			},
			{ accessorKey: 'display_name', header: '显示名', size: 160 },
			{
				accessorKey: 'visibility',
				header: '可见性',
				size: 100,
				cell: ({ getValue }) => <Badge variant="outline">{getValue<string>()}</Badge>,
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
					const r = row.original;
					return (
						<div className="flex items-center gap-1.5">
							<Button variant="outline" size="xs" onClick={() => openEdit(r)}>
								编辑
							</Button>
							<Button variant="outline" size="xs" onClick={() => openMembers(r)}>
								成员
							</Button>
							<Button variant="outline" size="xs" onClick={() => openPerms(r)}>
								权限
							</Button>
							{!r.is_system ? (
								<Button
									variant="destructive"
									size="xs"
									onClick={() =>
										confirm({
											title: '删除该用户组？',
											description: '组成员与权限配置会一并移除，且不可恢复。',
											danger: true,
											confirmText: '删除',
											onConfirm: () => handleDelete(r.id),
										})
									}
								>
									删除
								</Button>
							) : null}
						</div>
					);
				},
			},
		],
		[confirm, openEdit, openMembers, openPerms, handleDelete],
	);

	const memberColumns = useMemo<ColumnDef<GroupMember>[]>(
		() => [
			{ accessorKey: 'user_id', header: '用户 ID' },
			{
				accessorKey: 'role',
				header: '角色',
				size: 120,
				cell: ({ getValue }) => <Badge variant="outline">{getValue<string>()}</Badge>,
			},
			{
				accessorKey: 'joined_at',
				header: '加入时间',
				size: 180,
				cell: ({ getValue }) => {
					const v = getValue<string>();
					return v ? formatTime(v) : '-';
				},
			},
			{
				id: 'actions',
				header: '操作',
				size: 100,
				cell: ({ row }) => (
					<Button
						variant="destructive"
						size="xs"
						onClick={() =>
							confirm({
								title: '移除该成员？',
								description: '该用户会失去这个组的全部权限。',
								danger: true,
								confirmText: '移除',
								onConfirm: () => handleRemoveMember(row.original.user_id),
							})
						}
					>
						移除
					</Button>
				),
			},
		],
		[confirm, handleRemoveMember],
	);

	return (
		<div>
			<PageHeader
				title="用户组管理"
				actions={
					<div className="flex items-center gap-2">
						<Button size="sm" onClick={openCreate}>
							<Plus className="size-3.5" />
							新建用户组
						</Button>
						<Button variant="outline" size="sm" onClick={() => refetch()}>
							<RefreshCw className="size-3.5" />
							刷新
						</Button>
					</div>
				}
			/>
			<DataTable columns={columns} data={data} loading={loading} getRowId={(row) => row.id} />

			{/* 新建 / 编辑 */}
			<Dialog open={modalOpen} onOpenChange={(v) => !submitting && setModalOpen(v)}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>{editing ? '编辑用户组' : '新建用户组'}</DialogTitle>
					</DialogHeader>
					<Form {...groupForm}>
						<form onSubmit={groupForm.handleSubmit(handleSubmit)} className="flex flex-col gap-3">
							{!editing ? (
								<FormField
									control={groupForm.control}
									name="name"
									render={({ field }) => (
										<FormItem>
											<FormLabel>组标识</FormLabel>
											<FormControl>
												<Input placeholder="如 team-ops" {...field} />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
							) : null}
							<FormField
								control={groupForm.control}
								name="display_name"
								render={({ field }) => (
									<FormItem>
										<FormLabel>显示名</FormLabel>
										<FormControl>
											<Input placeholder="显示名" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={groupForm.control}
								name="visibility"
								render={({ field }) => (
									<FormItem>
										<FormLabel>可见性</FormLabel>
										<Select
											value={field.value || '__none__'}
											onValueChange={(v) => field.onChange(v === '__none__' ? '' : v)}
										>
											<SelectTrigger className="w-full">
												<SelectValue placeholder="public" />
											</SelectTrigger>
											<SelectContent>
												<SelectItem value="__none__">不指定</SelectItem>
												{VISIBILITY.map((v) => (
													<SelectItem key={v.value} value={v.value}>
														{v.label}
													</SelectItem>
												))}
											</SelectContent>
										</Select>
										<FormMessage />
									</FormItem>
								)}
							/>
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

			{/* 成员管理 */}
			<Dialog open={memberModalOpen} onOpenChange={(v) => !v && setMemberModalOpen(false)}>
				<DialogContent className="max-w-[720px]">
					<DialogHeader>
						<DialogTitle>{activeGroup ? `成员 — ${activeGroup.display_name || activeGroup.name}` : '成员'}</DialogTitle>
					</DialogHeader>
					<Form {...memberForm}>
						<form onSubmit={memberForm.handleSubmit(handleAddMember)} className="mb-3 flex items-end gap-2">
							<FormField
								control={memberForm.control}
								name="user_id"
								render={({ field }) => (
									<FormItem className="flex-1">
										<FormControl>
											<Input placeholder="用户 ID" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={memberForm.control}
								name="role"
								render={({ field }) => (
									<FormItem className="w-[140px]">
										<Select value={field.value || 'member'} onValueChange={field.onChange}>
											<SelectTrigger className="w-full">
												<SelectValue />
											</SelectTrigger>
											<SelectContent>
												{MEMBER_ROLES.map((r) => (
													<SelectItem key={r.value} value={r.value}>
														{r.label}
													</SelectItem>
												))}
											</SelectContent>
										</Select>
									</FormItem>
								)}
							/>
							<Button type="submit" size="sm">
								添加
							</Button>
						</form>
					</Form>
					<DataTable
						columns={memberColumns}
						data={members}
						getRowId={(row) => `${row.group_id}:${row.user_id}`}
						emptyText="暂无成员"
					/>
				</DialogContent>
			</Dialog>

			{/* 组权限 */}
			<Dialog open={permModalOpen} onOpenChange={(v) => !v && setPermModalOpen(false)}>
				<DialogContent className="max-w-[720px]">
					<DialogHeader>
						<DialogTitle>{permGroup ? `组权限 — ${permGroup.display_name || permGroup.name}` : '组权限'}</DialogTitle>
					</DialogHeader>
					<p className="text-muted-foreground mb-2 text-[13px]">勾选该组拥有的权限码（共 {groupPerms.length} 项）</p>
					<div className="max-h-[50vh] overflow-y-auto rounded-md border border-border p-3">
						<div className="grid grid-cols-2 gap-1.5">
							{permissions.map((p) => {
								const checked = groupPerms.includes(p.code);
								return (
									<label
										key={p.code}
										htmlFor={`group-perm-${p.code}`}
										className="flex cursor-pointer items-center gap-2 text-[13px] text-foreground"
									>
										<Checkbox
											id={`group-perm-${p.code}`}
											checked={checked}
											onCheckedChange={(v) =>
												setGroupPerms((prev) => (v ? [...prev, p.code] : prev.filter((c) => c !== p.code)))
											}
										/>
										<span className="truncate" title={`${p.code} — ${p.name}`}>
											{p.code}
										</span>
									</label>
								);
							})}
						</div>
					</div>
					<DialogFooter className="mt-2 gap-2">
						<Button variant="outline" size="sm" onClick={() => setPermModalOpen(false)}>
							取消
						</Button>
						<Button size="sm" onClick={handleSavePerms}>
							同步权限
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			{confirmDialog}
		</div>
	);
}
