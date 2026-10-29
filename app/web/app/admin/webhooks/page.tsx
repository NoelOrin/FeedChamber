'use client';

import { type Webhook, webhookService } from '@feedchamber/api-client';
import { formatTime } from '@feedchamber/shared-utils';
import { zodResolver } from '@hookform/resolvers/zod';
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
import { Switch } from '~/components/ui/switch';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

const EVENT_OPTIONS = [
	'content.replied',
	'content.liked',
	'user.followed',
	'user.mentioned',
	'content.watched',
	'post.created',
	'user.created',
	'report.created',
] as const;

/* 事件多选：antd 的 Select mode="tags" 允许自由输入任意事件名，
   但后端只认固定几种，自由输入只会产生永远不触发的配置。
   这里改成固定的 Checkbox 组，语义更准也更好点。 */
const webhookSchema = z.object({
	url: z.string().min(1, '请输入回调 URL'),
	secret: z.string().optional(),
	events: z.array(z.string()).min(1, '请至少选择一个事件'),
	enabled: z.boolean(),
	tls_verify: z.boolean(),
});

type WebhookValues = z.infer<typeof webhookSchema>;

const EMPTY: WebhookValues = {
	url: '',
	secret: '',
	events: ['post.created'],
	enabled: true,
	tls_verify: true,
};

export default function WebhookList() {
	const [page, setPage] = useState(1);
	const [pageSize] = useState(20);
	const queryClient = useQueryClient();

	const { data: res, isPending: loading } = useQuery({
		queryKey: qk.admin.webhooks.list,
		queryFn: () => webhookService.list({ page, page_size: pageSize }),
	});

	const data = res?.data ?? [];
	const total = res?.total ?? 0;

	/** 写操作后让该模块的列表缓存失效。 */
	const invalidate = useCallback(
		() => queryClient.invalidateQueries({ queryKey: qk.admin.webhooks.all }),
		[queryClient],
	);
	const [modalOpen, setModalOpen] = useState(false);
	const [editing, setEditing] = useState<Webhook | null>(null);
	const [saving, setSaving] = useState(false);
	const [confirm, confirmDialog] = useConfirm();

	const form = useForm<WebhookValues>({ resolver: zodResolver(webhookSchema), defaultValues: EMPTY });

	const openCreate = useCallback(() => {
		setEditing(null);
		form.reset(EMPTY);
		setModalOpen(true);
	}, [form]);

	const openEdit = useCallback(
		(record: Webhook) => {
			setEditing(record);
			let events: string[] = [];
			try {
				events = JSON.parse(record.events || '[]');
			} catch {
				events = [];
			}
			form.reset({
				url: record.url,
				secret: '',
				events,
				enabled: record.enabled,
				tls_verify: record.tls_verify ?? true,
			});
			setModalOpen(true);
		},
		[form],
	);

	const handleSubmit = useCallback(
		async (values: WebhookValues) => {
			setSaving(true);
			try {
				if (editing) {
					await webhookService.update(editing.id, values);
				} else {
					await webhookService.create(values);
				}
				toast.success('保存成功');
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

	const handleDelete = useCallback(
		async (id: string) => {
			try {
				await webhookService.delete(id);
				toast.success('删除成功');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '删除失败');
			}
		},
		[invalidate],
	);

	const handlePing = useCallback(async (id: string) => {
		try {
			const res = await webhookService.ping(id);
			toast.success(res?.status === 'success' ? 'Ping 成功' : `Ping 失败：${res?.last_error || ''}`);
		} catch (e: any) {
			toast.error(e.message || 'Ping 失败');
		}
	}, []);

	const columns = useMemo<ColumnDef<Webhook>[]>(
		() => [
			{ accessorKey: 'url', header: 'URL' },
			{
				accessorKey: 'events',
				header: '事件',
				cell: ({ getValue }) => {
					const raw = getValue<string>();
					try {
						return JSON.parse(raw || '[]').join(', ');
					} catch {
						return raw;
					}
				},
			},
			{
				accessorKey: 'enabled',
				header: '状态',
				size: 100,
				cell: ({ getValue }) =>
					getValue<boolean>() ? <Badge variant="primary">启用</Badge> : <Badge variant="outline">停用</Badge>,
			},
			{
				accessorKey: 'created_at',
				header: '创建时间',
				size: 170,
				cell: ({ getValue }) => {
					const v = getValue<string | undefined>();
					return v ? formatTime(v) : '-';
				},
			},
			{
				id: 'actions',
				header: '操作',
				size: 200,
				cell: ({ row }) => (
					<div className="flex items-center gap-1.5">
						<Button variant="outline" size="xs" onClick={() => handlePing(row.original.id)}>
							Ping
						</Button>
						<Button variant="outline" size="xs" onClick={() => openEdit(row.original)}>
							编辑
						</Button>
						<Button
							variant="destructive"
							size="xs"
							onClick={() =>
								confirm({
									title: '删除该 Webhook？',
									description: '该订阅将不再收到任何事件推送，不可恢复。',
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
		[confirm, handlePing, openEdit, handleDelete],
	);

	return (
		<div>
			<PageHeader
				title="Webhook 管理"
				actions={
					<Button size="sm" onClick={openCreate}>
						新建 Webhook
					</Button>
				}
			/>
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

			<Dialog open={modalOpen} onOpenChange={(v) => !saving && setModalOpen(v)}>
				<DialogContent className="max-h-[85vh] overflow-y-auto">
					<DialogHeader>
						<DialogTitle>{editing ? '编辑 Webhook' : '新建 Webhook'}</DialogTitle>
					</DialogHeader>
					<Form {...form}>
						<form onSubmit={form.handleSubmit(handleSubmit)} className="flex flex-col gap-3">
							<FormField
								control={form.control}
								name="url"
								render={({ field }) => (
									<FormItem>
										<FormLabel>回调 URL</FormLabel>
										<FormControl>
											<Input placeholder="https://example.com/hook" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="secret"
								render={({ field }) => (
									<FormItem>
										<FormLabel>签名密钥</FormLabel>
										<FormControl>
											<Input type="password" placeholder="留空则不修改" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="events"
								render={({ field }) => (
									<FormItem>
										<FormLabel>事件</FormLabel>
										<div className="grid grid-cols-2 gap-1.5">
											{EVENT_OPTIONS.map((evt) => {
												const checked = field.value?.includes(evt) ?? false;
												return (
													<label
														key={evt}
														htmlFor={`webhook-event-${evt}`}
														className="flex cursor-pointer items-center gap-2 text-[13px] text-foreground"
													>
														<Checkbox
															id={`webhook-event-${evt}`}
															checked={checked}
															onCheckedChange={(v) => {
																const next = v
																	? [...(field.value ?? []), evt]
																	: (field.value ?? []).filter((x) => x !== evt);
																field.onChange(next);
															}}
														/>
														{evt}
													</label>
												);
											})}
										</div>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="enabled"
								render={({ field }) => (
									<FormItem className="flex flex-row items-center justify-between">
										<FormLabel>启用</FormLabel>
										<FormControl>
											<Switch checked={field.value} onCheckedChange={field.onChange} />
										</FormControl>
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="tls_verify"
								render={({ field }) => (
									<FormItem className="flex flex-row items-center justify-between">
										<FormLabel>校验 TLS</FormLabel>
										<FormControl>
											<Switch checked={field.value} onCheckedChange={field.onChange} />
										</FormControl>
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
