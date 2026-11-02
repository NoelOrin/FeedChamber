'use client';

import { type Plugin, pluginService } from '@feedchamber/api-client';
import { zodResolver } from '@hookform/resolvers/zod';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import { Button } from '~/components/ui/button';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable } from '~/components/ui/data-table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '~/components/ui/form';
import { Input } from '~/components/ui/input';
import { PageHeader } from '~/components/ui/page-header';
import { Switch } from '~/components/ui/switch';
import { Textarea } from '~/components/ui/textarea';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

/* 插件表单校验：ID 与名称必填，settings 必须是合法 JSON 对象。
   原来只在 handleSave 里判空（message.error），非法 JSON 会直接提交给后端。 */
const pluginSchema = z.object({
	id: z.string().min(1, '插件 ID 不能为空'),
	name: z.string().min(1, '名称不能为空'),
	version: z.string().optional(),
	author: z.string().optional(),
	homepage: z.string().optional(),
	description: z.string().optional(),
	settings: z.string().refine((v) => {
		if (!v.trim()) return true;
		try {
			JSON.parse(v);
			return true;
		} catch {
			return false;
		}
	}, 'Settings 必须是合法 JSON'),
});

type PluginValues = z.infer<typeof pluginSchema>;

const EMPTY: PluginValues = {
	id: '',
	name: '',
	version: '0.1.0',
	author: '',
	homepage: '',
	description: '',
	settings: '{}',
};

export default function Plugins() {
	const [modalOpen, setModalOpen] = useState(false);
	const [editing, setEditing] = useState<Plugin | null>(null);
	const [saving, setSaving] = useState(false);
	const [confirm, confirmDialog] = useConfirm();
	const queryClient = useQueryClient();

	const form = useForm<PluginValues>({ resolver: zodResolver(pluginSchema), defaultValues: EMPTY });

	const { data = [], isPending: loading } = useQuery({
		queryKey: qk.admin.plugins.list,
		queryFn: async () => {
			const items = await pluginService.list();
			return items ?? [];
		},
	});

	/** 写操作后让该模块的列表缓存失效。 */
	const invalidate = useCallback(
		() => queryClient.invalidateQueries({ queryKey: qk.admin.plugins.all }),
		[queryClient],
	);

	const openCreate = useCallback(() => {
		setEditing(null);
		form.reset(EMPTY);
		setModalOpen(true);
	}, [form]);

	const openEdit = useCallback(
		(plugin: Plugin) => {
			setEditing(plugin);
			form.reset({
				id: plugin.id,
				name: plugin.name,
				version: plugin.version ?? '',
				author: plugin.author ?? '',
				homepage: plugin.homepage ?? '',
				description: plugin.description ?? '',
				settings: plugin.settings || '{}',
			});
			setModalOpen(true);
		},
		[form],
	);

	const handleSave = useCallback(
		async (values: PluginValues) => {
			setSaving(true);
			try {
				if (editing) {
					await pluginService.update(editing.id, values);
				} else {
					await pluginService.install({ ...values, enabled: true });
				}
				toast.success('已保存');
				setModalOpen(false);
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '保存失败');
			} finally {
				setSaving(false);
			}
		},
		[editing, invalidate],
	);

	const handleToggle = useCallback(
		async (plugin: Plugin, enabled: boolean) => {
			try {
				await pluginService.setEnabled(plugin.id, enabled);
				toast.success(enabled ? '已启用' : '已禁用');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '切换失败');
			}
		},
		[invalidate],
	);

	const handleDelete = useCallback(
		async (id: string) => {
			try {
				await pluginService.delete(id);
				toast.success('已删除');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '删除失败');
			}
		},
		[invalidate],
	);

	const columns = useMemo<ColumnDef<Plugin>[]>(
		() => [
			{ accessorKey: 'id', header: 'ID' },
			{ accessorKey: 'name', header: '名称', size: 180 },
			{ accessorKey: 'version', header: '版本', size: 100 },
			{ accessorKey: 'author', header: '作者', size: 140 },
			{ accessorKey: 'description', header: '描述' },
			{
				accessorKey: 'enabled',
				header: '启用',
				size: 80,
				cell: ({ row }) => (
					<Switch
						checked={row.original.enabled}
						onCheckedChange={(v) => handleToggle(row.original, v)}
						aria-label="启用插件"
					/>
				),
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
							onClick={() =>
								confirm({
									title: '删除该插件？',
									description: '插件元数据与配置会一并移除，不可恢复。',
									danger: true,
									confirmText: '删除',
									onConfirm: () => handleDelete(row.original.id),
								})
							}
						>
							删除
						</Button>
					</div>
				),
			},
		],
		[confirm, handleDelete, handleToggle, openEdit],
	);

	return (
		<div>
			<PageHeader
				title="插件管理"
				actions={
					<Button size="sm" onClick={openCreate}>
						注册插件
					</Button>
				}
			/>
			<DataTable columns={columns} data={data} loading={loading} getRowId={(row) => row.id} />

			<Dialog open={modalOpen} onOpenChange={(v) => !saving && setModalOpen(v)}>
				<DialogContent className="max-h-[85vh] overflow-y-auto">
					<DialogHeader>
						<DialogTitle>{editing ? '编辑插件' : '注册插件'}</DialogTitle>
					</DialogHeader>
					<Form {...form}>
						<form onSubmit={form.handleSubmit(handleSave)} className="flex flex-col gap-3">
							<FormField
								control={form.control}
								name="id"
								render={({ field }) => (
									<FormItem>
										<FormLabel>ID</FormLabel>
										<FormControl>
											<Input disabled={Boolean(editing)} {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="name"
								render={({ field }) => (
									<FormItem>
										<FormLabel>名称</FormLabel>
										<FormControl>
											<Input {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<div className="grid grid-cols-2 gap-3">
								<FormField
									control={form.control}
									name="version"
									render={({ field }) => (
										<FormItem>
											<FormLabel>版本</FormLabel>
											<FormControl>
												<Input {...field} />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={form.control}
									name="author"
									render={({ field }) => (
										<FormItem>
											<FormLabel>作者</FormLabel>
											<FormControl>
												<Input {...field} />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
							</div>
							<FormField
								control={form.control}
								name="homepage"
								render={({ field }) => (
									<FormItem>
										<FormLabel>主页</FormLabel>
										<FormControl>
											<Input {...field} />
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
										<FormLabel>描述</FormLabel>
										<FormControl>
											<Textarea rows={2} {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="settings"
								render={({ field }) => (
									<FormItem>
										<FormLabel>Settings（JSON）</FormLabel>
										<FormControl>
											<Textarea rows={3} {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<DialogFooter className="mt-2 gap-2">
								<Button type="button" variant="outline" size="sm" onClick={() => setModalOpen(false)}>
									取消
								</Button>
								<Button type="submit" size="sm" loading={saving}>
									保存
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
