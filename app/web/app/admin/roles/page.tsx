'use client';

import { roleService as adminRoleService, permissionService } from '@feedchamber/api-client';
import { zodResolver } from '@hookform/resolvers/zod';
import { useQueries, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Collapse, CollapseContent, CollapseItem, CollapseTrigger } from '~/components/ui/collapse';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '~/components/ui/form';
import { Input } from '~/components/ui/input';
import { PageHeader } from '~/components/ui/page-header';
import { useNavigate } from '~/lib/admin/navigation';
import { permissionTone } from '~/lib/admin/permissions';
import { Plus, RefreshCw, Trash2 } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

const roleSchema = z.object({
	name: z.string().min(1, '请输入角色名称'),
	description: z.string().optional(),
});

type RoleValues = z.infer<typeof roleSchema>;

export default function AdminRoles() {
	const navigate = useNavigate();
	const [modalVisible, setModalVisible] = useState(false);
	const [creating, setCreating] = useState(false);
	const [confirm, confirmDialog] = useConfirm();

	const form = useForm<RoleValues>({
		resolver: zodResolver(roleSchema),
		defaultValues: { name: '', description: '' },
	});

	const queryClient = useQueryClient();

	/* 角色列表与权限字典是两条独立查询。 */
	const { data: roles = [], isPending: loading } = useQuery({
		queryKey: qk.admin.roles.list,
		queryFn: async () => (await adminRoleService.list()) ?? [],
	});

	const { data: permissions = [] } = useQuery({
		queryKey: qk.admin.permissions.list,
		queryFn: async () => (await permissionService.list()) ?? [],
	});

	/* 每个角色的权限是 N 条查询，用 useQueries 并行发起——
	   原先是一个 useEffect 里 for 循环串行 await。 */
	const rolePermQueries = useQueries({
		queries: roles.map((role) => ({
			queryKey: qk.admin.roles.permissions(role.name),
			queryFn: () => permissionService.getByRole(role.name),
		})),
	});

	const rolePermissions = Object.fromEntries(
		roles.map((role, i) => [role.name, rolePermQueries[i]?.data?.permissions ?? []]),
	);

	/** 写操作后让角色列表与该模块缓存失效。 */
	const invalidate = () => queryClient.invalidateQueries({ queryKey: qk.admin.roles.all });

	async function handleCreateRole(values: RoleValues) {
		setCreating(true);
		try {
			await adminRoleService.create({ name: values.name.trim(), description: (values.description ?? '').trim() });
			toast.success('角色创建成功');
			setModalVisible(false);
			form.reset({ name: '', description: '' });
			invalidate();
		} catch (error: any) {
			toast.error(error.message || '角色创建失败');
		} finally {
			setCreating(false);
		}
	}

	function handleDeleteRole(roleId: string, roleName: string) {
		confirm({
			title: '删除角色',
			description: `确定要删除角色「${roleName}」吗？此操作不可恢复。`,
			danger: true,
			confirmText: '删除',
			onConfirm: async () => {
				try {
					await adminRoleService.delete(roleId);
					toast.success('角色删除成功');
					invalidate();
				} catch (error: any) {
					toast.error(error.message || '角色删除失败');
				}
			},
		});
	}

	return (
		<div>
			<PageHeader
				title="角色权限管理"
				actions={
					<div className="flex items-center gap-2">
						<Button variant="outline" size="sm" onClick={() => invalidate()}>
							<RefreshCw className="size-3.5" />
							刷新
						</Button>
						<Button size="sm" onClick={() => setModalVisible(true)}>
							<Plus className="size-3.5" />
							新建角色
						</Button>
					</div>
				}
			/>

			{roles.length === 0 && !loading ? (
				<p className="text-muted-foreground p-6 text-center text-[13px]">暂无角色数据</p>
			) : (
				<Collapse defaultOpen={roles.map((r) => r.name)}>
					{roles.map((role) => {
						const codes = rolePermissions[role.name] || [];
						return (
							<CollapseItem key={role.id} value={role.name}>
								<div className="flex items-center justify-between gap-3 pr-2">
									<CollapseTrigger className="flex-1">
										<span className="flex flex-wrap items-center gap-2">
											<Badge variant={role.name === 'admin' ? 'destructive' : role.is_default ? 'success' : 'primary'}>
												{role.display_name || role.name}
											</Badge>
											<span className="text-muted-foreground font-normal">{role.description || '-'}</span>
											{role.is_default ? <Badge variant="success">默认</Badge> : null}
										</span>
									</CollapseTrigger>
									<div className="flex shrink-0 items-center gap-1.5">
										<Button
											variant="link"
											size="xs"
											onClick={() => navigate({ to: '/admin/roles/detail', search: { id: role.id } })}
										>
											详情
										</Button>
										{!role.is_default ? (
											<Button
												variant="link"
												size="xs"
												className="text-destructive"
												onClick={() => handleDeleteRole(role.id, role.name)}
											>
												<Trash2 className="size-3" />
												删除
											</Button>
										) : null}
									</div>
								</div>
								<CollapseContent>
									<p className="mb-2 text-[13px] font-medium text-foreground">当前权限：</p>
									{codes.length === 0 ? (
										<p className="text-muted-foreground text-[13px]">暂无权限</p>
									) : (
										<div className="flex flex-wrap gap-1.5">
											{codes.map((code) => {
												const perm = permissions.find((p) => p.code === code);
												return (
													<Badge key={code} variant={permissionTone(code)}>
														{perm?.name || code}
													</Badge>
												);
											})}
										</div>
									)}
								</CollapseContent>
							</CollapseItem>
						);
					})}
				</Collapse>
			)}

			<Dialog open={modalVisible} onOpenChange={(v) => !creating && setModalVisible(v)}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>新建角色</DialogTitle>
					</DialogHeader>
					<Form {...form}>
						<form onSubmit={form.handleSubmit(handleCreateRole)} className="flex flex-col gap-3">
							<FormField
								control={form.control}
								name="name"
								render={({ field }) => (
									<FormItem>
										<FormLabel>角色名称</FormLabel>
										<FormControl>
											<Input placeholder="请输入角色名称（如：moderator）" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="description"
								render={({ field }) => (
									<FormItem>
										<FormLabel>角色描述</FormLabel>
										<FormControl>
											<Input placeholder="请输入角色描述（如：版主）" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<DialogFooter className="mt-2 gap-2">
								<Button type="button" variant="outline" size="sm" onClick={() => setModalVisible(false)}>
									取消
								</Button>
								<Button type="submit" size="sm" loading={creating}>
									创建
								</Button>
							</DialogFooter>
						</form>
					</Form>
				</DialogContent>
			</Dialog>

			{confirmDialog}
		</div>
	);
}
