'use client';

import {
	type Plugin,
	type PluginInstall,
	type PluginInstallStatus,
	type PluginSource,
	type PluginSourceStatus,
	pluginService,
} from '@feedchamber/api-client';
import { zodResolver } from '@hookform/resolvers/zod';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import { Alert, AlertDescription } from '~/components/ui/alert';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable } from '~/components/ui/data-table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '~/components/ui/form';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { PageHeader } from '~/components/ui/page-header';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Switch } from '~/components/ui/switch';
import { Textarea } from '~/components/ui/textarea';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

function errorMessage(error: unknown): string {
	return error instanceof Error ? error.message : String(error ?? '未知错误');
}

function isJson(value: string): boolean {
	try {
		JSON.parse(value);
		return true;
	} catch {
		return false;
	}
}

/* 安装表单：v2 装的是「来源 + plugin.json 原文」，不再是旧的 id/name/version 声明式字段。 */
const installSchema = z.object({
	plugin_id: z.string().min(1, '插件 ID 不能为空'),
	source_id: z.string().min(1, '请选择插件来源'),
	manifest: z.string().min(1, 'Manifest 不能为空').refine(isJson, 'Manifest 必须是合法 JSON（plugin.json 原文）'),
});
type InstallValues = z.infer<typeof installSchema>;

/* 编辑表单：只改声明式元数据。enabled 不进表单，提交时显式带上 editing.enabled，
   否则后端 `pluginRow.Enabled = input.Enabled` 会把插件静默禁用。 */
const editSchema = z.object({
	name: z.string().min(1, '名称不能为空'),
	version: z.string().optional(),
	author: z.string().optional(),
	homepage: z.string().optional(),
	description: z.string().optional(),
	settings: z.string().refine((v) => !v.trim() || isJson(v), 'Settings 必须是合法 JSON'),
});
type EditValues = z.infer<typeof editSchema>;

const INSTALL_EMPTY: InstallValues = { plugin_id: '', source_id: '', manifest: '{}' };
const EDIT_EMPTY: EditValues = {
	name: '',
	version: '',
	author: '',
	homepage: '',
	description: '',
	settings: '{}',
};

const SOURCE_STATUS: Record<PluginSourceStatus, { label: string; variant: 'warning' | 'success' | 'destructive' }> = {
	pending: { label: '待审批', variant: 'warning' },
	approved: { label: '已通过', variant: 'success' },
	rejected: { label: '已拒绝', variant: 'destructive' },
};

const INSTALL_STATUS: Record<
	PluginInstallStatus,
	{ label: string; variant: 'info' | 'warning' | 'success' | 'destructive' | 'outline' }
> = {
	installed: { label: '已安装', variant: 'info' },
	starting: { label: '启动中', variant: 'warning' },
	running: { label: '运行中', variant: 'success' },
	crashed: { label: '已崩溃', variant: 'destructive' },
	stopped: { label: '已停止', variant: 'outline' },
};

type ModalMode = 'install' | 'edit' | null;

export default function Plugins() {
	const [modal, setModal] = useState<ModalMode>(null);
	const [editing, setEditing] = useState<Plugin | null>(null);
	const [saving, setSaving] = useState(false);
	const [sourceOpen, setSourceOpen] = useState(false);
	const [sourceSaving, setSourceSaving] = useState(false);
	const [sourceName, setSourceName] = useState('');
	const [sourceKind, setSourceKind] = useState<'git'>('git');
	const [sourceUrl, setSourceUrl] = useState('');
	const [confirm, confirmDialog] = useConfirm();
	const queryClient = useQueryClient();

	const installForm = useForm<InstallValues>({ resolver: zodResolver(installSchema), defaultValues: INSTALL_EMPTY });
	const editForm = useForm<EditValues>({ resolver: zodResolver(editSchema), defaultValues: EDIT_EMPTY });

	/* 每个查询都保留 query 对象：error 态用 Alert variant="destructive" 显式呈现，
	   不用 `const { data = [] }`（解构默认值每次渲染都是新数组）。 */
	const pluginsQuery = useQuery({
		queryKey: qk.admin.plugins.list,
		queryFn: async () => (await pluginService.list()) ?? [],
	});
	const sourcesQuery = useQuery({
		queryKey: qk.admin.plugins.sources,
		queryFn: async () => (await pluginService.listSources()) ?? [],
	});
	const installsQuery = useQuery({
		queryKey: qk.admin.plugins.installs,
		queryFn: async () => (await pluginService.listInstalls()) ?? [],
	});

	const plugins = pluginsQuery.data ?? [];
	const sources = sourcesQuery.data ?? [];
	const installs = installsQuery.data ?? [];

	/** 写操作后一次失效该模块的 list / sources / installs 三份缓存。 */
	const invalidate = useCallback(
		() => queryClient.invalidateQueries({ queryKey: qk.admin.plugins.all }),
		[queryClient],
	);

	/** 运行时状态按 plugin_id 合并到登记记录（后端不提供单插件详情端点）。 */
	const installByPlugin = useMemo(() => {
		const map = new Map<string, PluginInstall>();
		for (const item of installs) map.set(item.plugin_id, item);
		return map;
	}, [installs]);

	const openInstall = useCallback(() => {
		setEditing(null);
		installForm.reset(INSTALL_EMPTY);
		setModal('install');
	}, [installForm]);

	const openEdit = useCallback(
		(plugin: Plugin) => {
			setEditing(plugin);
			editForm.reset({
				name: plugin.name ?? '',
				version: plugin.version ?? '',
				author: plugin.author ?? '',
				homepage: plugin.homepage ?? '',
				description: plugin.description ?? '',
				settings: plugin.settings || '{}',
			});
			setModal('edit');
		},
		[editForm],
	);

	const handleInstall = useCallback(
		async (values: InstallValues) => {
			setSaving(true);
			try {
				await pluginService.install(values);
				toast.success('插件已注册');
				setModal(null);
				await invalidate();
			} catch (err) {
				toast.error(errorMessage(err));
			} finally {
				setSaving(false);
			}
		},
		[invalidate],
	);

	const handleEdit = useCallback(
		async (values: EditValues) => {
			if (!editing) return;
			setSaving(true);
			try {
				// enabled 必须显式带上：省略会被后端按零值 false 覆盖，静默禁用插件。
				await pluginService.update(editing.id, { ...values, enabled: editing.enabled });
				toast.success('已保存');
				setModal(null);
				await invalidate();
			} catch (err) {
				toast.error(errorMessage(err));
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
				await invalidate();
			} catch (err) {
				toast.error(errorMessage(err));
			}
		},
		[invalidate],
	);

	const handleUninstall = useCallback(
		async (id: string) => {
			try {
				await pluginService.uninstall(id);
				toast.success('已卸载');
				await invalidate();
			} catch (err) {
				toast.error(errorMessage(err));
			}
		},
		[invalidate],
	);

	const openSourceCreate = useCallback(() => {
		setSourceName('');
		setSourceKind('git');
		setSourceUrl('');
		setSourceOpen(true);
	}, []);

	const handleCreateSource = useCallback(async () => {
		if (!sourceName.trim() || !sourceUrl.trim()) {
			toast.error('名称与地址不能为空');
			return;
		}
		setSourceSaving(true);
		try {
			await pluginService.createSource({ name: sourceName.trim(), kind: sourceKind, url: sourceUrl.trim() });
			toast.success('来源已登记，待审批后方可安装');
			setSourceOpen(false);
			await invalidate();
		} catch (err) {
			toast.error(errorMessage(err));
		} finally {
			setSourceSaving(false);
		}
	}, [sourceName, sourceKind, sourceUrl, invalidate]);

	const handleApprove = useCallback(
		async (source: PluginSource, approve: boolean) => {
			try {
				if (approve) await pluginService.approveSource(source.id);
				else await pluginService.rejectSource(source.id);
				toast.success(approve ? '来源已通过' : '来源已拒绝');
				await invalidate();
			} catch (err) {
				toast.error(errorMessage(err));
			}
		},
		[invalidate],
	);

	const columns = useMemo<ColumnDef<Plugin>[]>(
		() => [
			{ accessorKey: 'id', header: 'ID' },
			{ accessorKey: 'name', header: '名称', size: 160 },
			{ accessorKey: 'version', header: '版本', size: 90 },
			{ accessorKey: 'author', header: '作者', size: 120 },
			{
				id: 'runtime',
				header: '运行状态',
				size: 200,
				cell: ({ row }) => {
					const inst = installByPlugin.get(row.original.id);
					if (!inst) return <span className="text-xs text-muted-foreground">未安装</span>;
					const meta = INSTALL_STATUS[inst.status] ?? { label: inst.status, variant: 'outline' as const };
					return (
						<div className="flex flex-col items-start gap-1">
							<Badge variant={meta.variant}>{meta.label}</Badge>
							<span className="text-xs text-muted-foreground">重启 {inst.restarts} 次</span>
							{inst.last_error ? (
								<span className="max-w-[220px] truncate text-xs text-destructive" title={inst.last_error}>
									{inst.last_error}
								</span>
							) : null}
						</div>
					);
				},
			},
			{
				accessorKey: 'enabled',
				header: '启用',
				size: 70,
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
									title: '卸载该插件？',
									description: '将停止插件子进程，并移除插件配置与安装记录（两张表），不可恢复。',
									danger: true,
									confirmText: '卸载',
									onConfirm: () => handleUninstall(row.original.id),
								})
							}
						>
							卸载
						</Button>
					</div>
				),
			},
		],
		[confirm, handleToggle, handleUninstall, installByPlugin, openEdit],
	);

	return (
		<div className="flex flex-col gap-5">
			<PageHeader
				title="插件管理"
				meta={`${plugins.length} 个插件`}
				actions={
					<Button size="sm" onClick={openInstall}>
						注册插件
					</Button>
				}
			/>

			{pluginsQuery.isError ? (
				<Alert variant="destructive">
					<AlertDescription>插件列表读取失败：{errorMessage(pluginsQuery.error)}</AlertDescription>
				</Alert>
			) : null}
			{sourcesQuery.isError ? (
				<Alert variant="destructive">
					<AlertDescription>插件来源读取失败：{errorMessage(sourcesQuery.error)}</AlertDescription>
				</Alert>
			) : null}
			{installsQuery.isError ? (
				<Alert variant="destructive">
					<AlertDescription>运行状态读取失败：{errorMessage(installsQuery.error)}</AlertDescription>
				</Alert>
			) : null}

			<DataTable columns={columns} data={plugins} loading={pluginsQuery.isPending} getRowId={(row) => row.id} />

			{/* 来源审批：pending/rejected 的来源不可安装，安装表单里也会被禁用。 */}
			<section className="flex flex-col gap-3">
				<div className="flex flex-wrap items-center justify-between gap-2">
					<div className="flex items-baseline gap-2">
						<h2 className="text-base font-semibold text-foreground">插件来源</h2>
						<span className="text-[13px] text-muted-foreground tabular-nums">{sources.length} 个</span>
					</div>
					<Button variant="outline" size="sm" onClick={openSourceCreate}>
						新增来源
					</Button>
				</div>
				{sourcesQuery.isPending ? (
					<p className="text-sm text-muted-foreground">加载中…</p>
				) : sources.length === 0 ? (
					<p className="text-sm text-muted-foreground">还没有登记插件来源。</p>
				) : (
					<div className="flex flex-col gap-2">
						{sources.map((source) => {
							const meta = SOURCE_STATUS[source.status] ?? { label: source.status, variant: 'outline' as const };
							return (
								<div
									key={source.id}
									className="flex flex-wrap items-center gap-2 rounded-md border border-border px-3 py-2.5"
								>
									<span className="text-sm font-medium text-foreground">{source.name}</span>
									<Badge variant="outline">{source.kind}</Badge>
									<Badge variant={meta.variant}>{meta.label}</Badge>
									<span className="min-w-0 flex-1 truncate font-mono text-xs text-muted-foreground">{source.url}</span>
									{source.status === 'pending' ? (
										<>
											<Button variant="outline" size="xs" onClick={() => handleApprove(source, true)}>
												通过
											</Button>
											<Button variant="ghost" size="xs" onClick={() => handleApprove(source, false)}>
												拒绝
											</Button>
										</>
									) : null}
								</div>
							);
						})}
					</div>
				)}
			</section>

			{/* 注册插件：来源 + plugin.json 原文 */}
			<Dialog open={modal === 'install'} onOpenChange={(v) => !saving && !v && setModal(null)}>
				<DialogContent className="max-h-[85vh] overflow-y-auto">
					<DialogHeader>
						<DialogTitle>注册插件</DialogTitle>
					</DialogHeader>
					<Form {...installForm}>
						<form onSubmit={installForm.handleSubmit(handleInstall)} className="flex flex-col gap-3">
							<FormField
								control={installForm.control}
								name="plugin_id"
								render={({ field }) => (
									<FormItem>
										<FormLabel>插件 ID</FormLabel>
										<FormControl>
											<Input placeholder="com.example.my-plugin" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={installForm.control}
								name="source_id"
								render={({ field }) => (
									<FormItem>
										<FormLabel>插件来源</FormLabel>
										<Select value={field.value} onValueChange={field.onChange}>
											<FormControl>
												<SelectTrigger>
													<SelectValue placeholder="选择已审批的来源" />
												</SelectTrigger>
											</FormControl>
											<SelectContent>
												{sources.length === 0 ? (
													<SelectItem value="__none__" disabled>
														暂无来源，请先在下方新增并审批
													</SelectItem>
												) : (
													sources.map((source) => (
														<SelectItem key={source.id} value={source.id} disabled={source.status !== 'approved'}>
															{source.name}（{SOURCE_STATUS[source.status]?.label ?? source.status}）
														</SelectItem>
													))
												)}
											</SelectContent>
										</Select>
										<FormMessage />
										<p className="text-xs text-muted-foreground">只有「已通过」的来源可用于安装。</p>
									</FormItem>
								)}
							/>
							<FormField
								control={installForm.control}
								name="manifest"
								render={({ field }) => (
									<FormItem>
										<FormLabel>Manifest（plugin.json 原文）</FormLabel>
										<FormControl>
											<Textarea rows={8} className="font-mono text-xs" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<DialogFooter className="mt-2 gap-2">
								<Button type="button" variant="outline" size="sm" onClick={() => setModal(null)}>
									取消
								</Button>
								<Button type="submit" size="sm" loading={saving}>
									注册
								</Button>
							</DialogFooter>
						</form>
					</Form>
				</DialogContent>
			</Dialog>

			{/* 编辑：只改声明式元数据；enabled 由提交时显式携带，不进表单 */}
			<Dialog open={modal === 'edit'} onOpenChange={(v) => !saving && !v && setModal(null)}>
				<DialogContent className="max-h-[85vh] overflow-y-auto">
					<DialogHeader>
						<DialogTitle>编辑插件</DialogTitle>
					</DialogHeader>
					<Form {...editForm}>
						<form onSubmit={editForm.handleSubmit(handleEdit)} className="flex flex-col gap-3">
							<FormField
								control={editForm.control}
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
									control={editForm.control}
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
									control={editForm.control}
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
								control={editForm.control}
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
								control={editForm.control}
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
								control={editForm.control}
								name="settings"
								render={({ field }) => (
									<FormItem>
										<FormLabel>Settings（JSON）</FormLabel>
										<FormControl>
											<Textarea rows={3} className="font-mono text-xs" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<DialogFooter className="mt-2 gap-2">
								<Button type="button" variant="outline" size="sm" onClick={() => setModal(null)}>
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

			{/* 新增来源 */}
			<Dialog open={sourceOpen} onOpenChange={setSourceOpen}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>新增插件来源</DialogTitle>
					</DialogHeader>
					<div className="flex flex-col gap-3">
						<div className="flex flex-col gap-1.5">
							<Label>名称</Label>
							<Input value={sourceName} onChange={(e) => setSourceName(e.target.value)} placeholder="官方插件仓库" />
						</div>
						<div className="flex flex-col gap-1.5">
							<Label>类型</Label>
							<Select value={sourceKind} onValueChange={(v) => setSourceKind(v as 'git')}>
								<SelectTrigger>
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="git">git（受信任代码仓库）</SelectItem>
								</SelectContent>
							</Select>
						</div>
						<div className="flex flex-col gap-1.5">
							<Label>地址</Label>
							<Input
								value={sourceUrl}
								onChange={(e) => setSourceUrl(e.target.value)}
								placeholder="https://example.com/plugins.git"
								className="font-mono text-xs"
							/>
						</div>
						<p className="text-xs text-muted-foreground">新来源默认为「待审批」，通过后才能用于安装。</p>
					</div>
					<DialogFooter className="gap-2">
						<Button variant="outline" size="sm" onClick={() => setSourceOpen(false)}>
							取消
						</Button>
						<Button size="sm" loading={sourceSaving} onClick={handleCreateSource}>
							登记
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			{confirmDialog}
		</div>
	);
}
