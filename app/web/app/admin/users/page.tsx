'use client';

import { type AdminUser, adminService, authService, roleService, userService } from '@orinforum/api-client';
import { formatTime } from '@orinforum/shared-utils';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef, RowSelectionState } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable, selectColumn } from '~/components/ui/data-table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Label } from '~/components/ui/label';
import { PageHeader } from '~/components/ui/page-header';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { useNavigate } from '~/lib/admin/navigation';
import { RefreshCw } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

export default function AdminUsers() {
	const navigate = useNavigate();
	const [page, setPage] = useState(1);
	const [pageSize] = useState(20);
	const queryClient = useQueryClient();
	const [editUser, setEditUser] = useState<AdminUser | null>(null);
	const [modalVisible, setModalVisible] = useState(false);
	const [selectedRole, setSelectedRole] = useState<string>('');
	const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
	const [batchRole, setBatchRole] = useState<string>('');
	const [batchRoleModal, setBatchRoleModal] = useState(false);
	const [batchLoading, setBatchLoading] = useState(false);
	const [confirm, confirmDialog] = useConfirm();

	/* 角色下拉的数据同样走缓存：它与用户列表是两条独立查询，
	   原先靠同一个 useEffect 里的 then 链 + ignore 标记并发拉取。 */
	const { data: roles = [] } = useQuery({
		queryKey: qk.admin.roles.list,
		queryFn: async () => (await roleService.list()) ?? [],
	});

	const roleDisplay = Object.fromEntries(roles.map((r) => [r.name, r.display_name]));
	const selectedIds = Object.keys(rowSelection).filter((k) => rowSelection[k]);

	const { data: res, isPending: loading } = useQuery({
		queryKey: qk.admin.users.list({ page, pageSize }),
		queryFn: () => userService.list({ page, page_size: pageSize }),
	});

	const data = res?.items ?? [];
	// 后端 /user/list 只返回 items，没有 total 字段；用 items.length 兜底，
	// 否则分页条会显示「共 0 条」而表格里明明有数据。
	const total = res?.total ?? data.length;

	/** 写操作后让该模块的列表缓存失效。 */
	const invalidate = useCallback(() => queryClient.invalidateQueries({ queryKey: qk.admin.users.all }), [queryClient]);

	const handleBan = useCallback(
		(record: AdminUser) => {
			confirm({
				title: '禁言该用户？',
				description: `用户 ${record.username} 将无法发帖与评论，直到解禁。`,
				danger: true,
				confirmText: '禁言',
				onConfirm: async () => {
					try {
						await userService.ban(record.id, {});
						toast.success('禁言成功');
						invalidate();
					} catch (error: any) {
						toast.error(error.message || '禁言失败');
					}
				},
			});
		},
		[confirm, invalidate],
	);

	async function handleChangeRole() {
		if (!editUser || !selectedRole) return;
		try {
			await userService.updateRole(editUser.id, selectedRole);
			toast.success('角色更新成功');
			setModalVisible(false);
			setEditUser(null);
			setSelectedRole('');
			invalidate();
		} catch (error: any) {
			toast.error(error.message || '角色更新失败');
		}
	}

	function handleBatchDelete() {
		if (!selectedIds.length) return;
		confirm({
			title: `批量删除 ${selectedIds.length} 个用户？`,
			description: '此操作不可撤销，用户的帖子与评论会一并处理。',
			danger: true,
			confirmText: '删除',
			onConfirm: async () => {
				setBatchLoading(true);
				try {
					await adminService.batchUsers({ ids: selectedIds.map(String), action: 'delete' });
					toast.success('批量删除成功');
					setRowSelection({});
					invalidate();
				} catch (error: any) {
					toast.error(error.message || '批量删除失败');
				} finally {
					setBatchLoading(false);
				}
			},
		});
	}

	async function handleBatchSetRole() {
		if (!selectedIds.length || !batchRole) return;
		setBatchLoading(true);
		try {
			await adminService.batchUsers({
				ids: selectedIds.map(String),
				action: 'set_role',
				params: { role: batchRole },
			});
			toast.success('批量改角色成功');
			setBatchRoleModal(false);
			setBatchRole('');
			setRowSelection({});
			invalidate();
		} catch (error: any) {
			toast.error(error.message || '批量改角色失败');
		} finally {
			setBatchLoading(false);
		}
	}

	const columns = useMemo<ColumnDef<AdminUser>[]>(
		() => [
			selectColumn<AdminUser>(),
			{ accessorKey: 'username', header: '用户名', size: 150 },
			{ accessorKey: 'display_name', header: '显示名称', size: 150 },
			{ accessorKey: 'email', header: '邮箱', size: 200 },
			{
				accessorKey: 'role',
				header: '角色',
				size: 100,
				cell: ({ row }) => (
					<Badge variant={row.original.role === 'admin' ? 'destructive' : 'primary'}>
						{roleDisplay[row.original.role] || row.original.role}
					</Badge>
				),
			},
			{
				accessorKey: 'created_at',
				header: '注册时间',
				size: 180,
				cell: ({ getValue }) => formatTime(getValue<string>()),
			},
			{
				id: 'actions',
				header: '操作',
				size: 280,
				cell: ({ row }) => {
					const record = row.original;
					return (
						<div className="flex items-center gap-1.5">
							<Button
								variant="link"
								size="xs"
								onClick={() => navigate({ to: '/admin/users/detail', search: { id: record.id } })}
							>
								详情
							</Button>
							<Button
								variant="link"
								size="xs"
								onClick={() => {
									setEditUser(record);
									setSelectedRole(record.role);
									setModalVisible(true);
								}}
							>
								修改角色
							</Button>
							{record.role === 'pending_user' ? (
								<Button
									variant="link"
									size="xs"
									onClick={async () => {
										try {
											await authService.approveRegistration(record.id);
											toast.success('已审批');
											invalidate();
										} catch (e: any) {
											toast.error(e?.message || '审批失败');
										}
									}}
								>
									审批
								</Button>
							) : null}
							{record.role !== 'admin' ? (
								<Button variant="link" size="xs" className="text-destructive" onClick={() => handleBan(record)}>
									禁言
								</Button>
							) : null}
						</div>
					);
				},
			},
		],
		[roleDisplay, navigate, invalidate, handleBan],
	);

	return (
		<div>
			<PageHeader
				title="用户管理"
				actions={
					<div className="flex flex-wrap items-center gap-2">
						<Button variant="outline" size="sm" onClick={() => invalidate()}>
							<RefreshCw className="size-3.5" />
							刷新
						</Button>
						<Button
							variant="destructive"
							size="sm"
							disabled={!selectedIds.length}
							loading={batchLoading}
							onClick={handleBatchDelete}
						>
							批量删除{selectedIds.length ? ` (${selectedIds.length})` : ''}
						</Button>
						<Button
							variant="outline"
							size="sm"
							disabled={!selectedIds.length}
							loading={batchLoading}
							onClick={() => setBatchRoleModal(true)}
						>
							批量改角色{selectedIds.length ? ` (${selectedIds.length})` : ''}
						</Button>
					</div>
				}
			/>

			<DataTable
				columns={columns}
				data={data}
				loading={loading}
				getRowId={(row) => row.id}
				rowSelection={rowSelection}
				onRowSelectionChange={setRowSelection}
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

			<Dialog open={modalVisible} onOpenChange={(v) => !v && setModalVisible(false)}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>修改用户角色</DialogTitle>
					</DialogHeader>
					{editUser ? (
						<div className="flex flex-col gap-3">
							<p className="text-[13px] text-muted-foreground">
								用户：<strong className="font-medium text-foreground">{editUser.username}</strong>
							</p>
							<div className="flex flex-col gap-1.5">
								<Label htmlFor="user-role">角色</Label>
								<Select value={selectedRole} onValueChange={setSelectedRole}>
									<SelectTrigger id="user-role" className="w-full">
										<SelectValue placeholder="选择角色" />
									</SelectTrigger>
									<SelectContent>
										{roles.map((r) => (
											<SelectItem key={r.name} value={r.name}>
												{r.display_name || r.name}
											</SelectItem>
										))}
									</SelectContent>
								</Select>
							</div>
						</div>
					) : null}
					<DialogFooter className="mt-2 gap-2">
						<Button variant="outline" size="sm" onClick={() => setModalVisible(false)}>
							取消
						</Button>
						<Button size="sm" onClick={handleChangeRole}>
							确认
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			<Dialog open={batchRoleModal} onOpenChange={(v) => !batchLoading && setBatchRoleModal(v)}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>批量修改角色（{selectedIds.length} 人）</DialogTitle>
					</DialogHeader>
					<div className="flex flex-col gap-1.5">
						<Label htmlFor="batch-role">目标角色</Label>
						<Select value={batchRole} onValueChange={setBatchRole}>
							<SelectTrigger id="batch-role" className="w-full">
								<SelectValue placeholder="选择角色" />
							</SelectTrigger>
							<SelectContent>
								{roles.map((r) => (
									<SelectItem key={r.name} value={r.name}>
										{r.display_name || r.name}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</div>
					<DialogFooter className="mt-2 gap-2">
						<Button variant="outline" size="sm" onClick={() => setBatchRoleModal(false)}>
							取消
						</Button>
						<Button size="sm" loading={batchLoading} disabled={!batchRole} onClick={handleBatchSetRole}>
							确认
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			{confirmDialog}
		</div>
	);
}
