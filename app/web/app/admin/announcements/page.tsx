'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { type Announcement, announcementService } from '@orinforum/api-client';
import { formatTime } from '@orinforum/shared-utils';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable } from '~/components/ui/data-table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '~/components/ui/form';
import { Input } from '~/components/ui/input';
import { PageHeader } from '~/components/ui/page-header';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Switch } from '~/components/ui/switch';
import { Textarea } from '~/components/ui/textarea';
import { Plus, RefreshCw } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

const displayTypeMap: Record<number, string> = { 1: 'Toast', 2: 'Alert', 3: 'Modal', 4: 'iframe 嵌入' };
const STATUS: Record<string, { text: string; variant: 'outline' | 'primary' }> = {
	draft: { text: '草稿', variant: 'outline' },
	published: { text: '已发布', variant: 'primary' },
};

/* display_type=4 时 content 是 iframe src（URL），其余是 Markdown 正文。
   校验按类型分支：URL 形态要求 http(s)，Markdown 形态只要求非空。 */
const schema = z
	.object({
		title: z.string().min(1, '请输入公告标题'),
		content: z.string(),
		status: z.enum(['draft', 'published']),
		is_pinned: z.boolean(),
		display_type: z.number().int(),
		sort: z.string().optional(),
		position: z.string().optional(),
		expires_at: z.string().optional(),
	})
	.superRefine((v, ctx) => {
		if (v.display_type === 4) {
			if (!v.content.trim()) {
				ctx.addIssue({ code: 'custom', message: '请输入嵌入页面 URL', path: ['content'] });
			} else if (!/^https?:\/\//.test(v.content.trim())) {
				ctx.addIssue({ code: 'custom', message: '必须是 http(s) 开头的 URL', path: ['content'] });
			}
		} else if (!v.content.trim()) {
			ctx.addIssue({ code: 'custom', message: '请输入公告内容', path: ['content'] });
		}
	});

type Values = z.infer<typeof schema>;

const EMPTY: Values = {
	title: '',
	content: '',
	status: 'draft',
	is_pinned: false,
	display_type: 1,
	sort: '0',
	position: '',
	expires_at: '',
};

const POSITIONS = [
	{ value: 'topRight', label: '右上角' },
	{ value: 'topLeft', label: '左上角' },
	{ value: 'bottomRight', label: '右下角' },
	{ value: 'bottomLeft', label: '左下角' },
	{ value: 'top', label: '顶部' },
	{ value: 'bottom', label: '底部' },
];

export default function AnnouncementList() {
	const [page, setPage] = useState(1);
	const [pageSize] = useState(20);
	const [modalOpen, setModalOpen] = useState(false);
	const [editRecord, setEditRecord] = useState<Announcement | null>(null);
	const [submitting, setSubmitting] = useState(false);
	const [confirm, confirmDialog] = useConfirm();
	const queryClient = useQueryClient();

	const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: EMPTY });
	const displayType = form.watch('display_type');
	const isIframe = displayType === 4;

	const { data: res, isPending: loading } = useQuery({
		queryKey: qk.admin.announcements.list({ page, pageSize }),
		queryFn: () => announcementService.list({ page, page_size: pageSize }),
	});

	const data = res?.items ?? [];
	const total = res?.total ?? 0;

	/** 写操作后让该模块的列表缓存失效。 */
	const invalidate = useCallback(
		() => queryClient.invalidateQueries({ queryKey: qk.admin.announcements.all }),
		[queryClient],
	);

	function openCreate() {
		setEditRecord(null);
		form.reset(EMPTY);
		setModalOpen(true);
	}

	const openEdit = useCallback(
		(record: Announcement) => {
			setEditRecord(record);
			form.reset({
				title: record.title ?? '',
				content: record.content ?? '',
				status: (record.status as 'draft' | 'published') ?? 'draft',
				is_pinned: Boolean(record.is_pinned),
				display_type: record.display_type ?? 1,
				sort: String(record.sort ?? 0),
				position: record.position ?? '',
				expires_at: record.expires_at ?? '',
			});
			setModalOpen(true);
		},
		[form],
	);

	async function handleSubmit(values: Values) {
		setSubmitting(true);
		try {
			const payload: any = {
				title: values.title,
				content: values.content,
				status: values.status,
				is_pinned: values.is_pinned,
				display_type: values.display_type,
				sort: values.sort ? Number(values.sort) : 0,
				position: values.position || undefined,
				expires_at: values.expires_at || undefined,
			};
			if (editRecord) {
				await announcementService.update(editRecord.id, payload);
				toast.success('更新成功');
			} else {
				await announcementService.create(payload);
				toast.success('创建成功');
			}
			setModalOpen(false);
			form.reset(EMPTY);
			setEditRecord(null);
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
				await announcementService.delete(id);
				toast.success('删除成功');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '删除失败');
			}
		},
		[invalidate],
	);

	const handlePublish = useCallback(
		async (id: string, publish: boolean) => {
			try {
				if (publish) {
					await announcementService.publish(id);
					toast.success('发布成功');
				} else {
					await announcementService.unpublish(id);
					toast.success('已撤回');
				}
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '操作失败');
			}
		},
		[invalidate],
	);

	const columns = useMemo<ColumnDef<Announcement>[]>(
		() => [
			{ accessorKey: 'title', header: '标题', size: 200 },
			{
				accessorKey: 'status',
				header: '状态',
				size: 90,
				cell: ({ getValue }) => {
					const cfg = STATUS[getValue<string>()];
					return cfg ? <Badge variant={cfg.variant}>{cfg.text}</Badge> : getValue<string>();
				},
			},
			{
				accessorKey: 'is_pinned',
				header: '置顶',
				size: 70,
				cell: ({ getValue }) =>
					getValue<boolean>() ? <Badge variant="primary">是</Badge> : <Badge variant="outline">否</Badge>,
			},
			{ accessorKey: 'sort', header: '排序', size: 60 },
			{
				accessorKey: 'display_type',
				header: '展示类型',
				size: 90,
				cell: ({ getValue }) => displayTypeMap[getValue<number>()] || getValue<number>(),
			},
			{ accessorKey: 'view_count', header: '浏览', size: 60 },
			{
				accessorKey: 'expires_at',
				header: '过期时间',
				size: 170,
				cell: ({ getValue }) => {
					const v = getValue<string>();
					return v ? formatTime(v) : '-';
				},
			},
			{
				accessorKey: 'created_at',
				header: '创建时间',
				size: 170,
				cell: ({ getValue }) => formatTime(getValue<string>()),
			},
			{
				id: 'actions',
				header: '操作',
				size: 320,
				cell: ({ row }) => {
					const record = row.original;
					return (
						<div className="flex items-center gap-1.5">
							<Button variant="outline" size="xs" onClick={() => openEdit(record)}>
								编辑
							</Button>
							{record.status === 'draft' ? (
								<Button size="xs" onClick={() => handlePublish(record.id, true)}>
									发布
								</Button>
							) : (
								<Button variant="outline" size="xs" onClick={() => handlePublish(record.id, false)}>
									撤回
								</Button>
							)}
							<Button
								variant="destructive"
								size="xs"
								onClick={() =>
									confirm({
										title: '删除该公告？',
										description: '公告会从站点上移除，且不可恢复。',
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
		[confirm, openEdit, handlePublish, handleDelete],
	);

	return (
		<div>
			<PageHeader
				title="公告管理"
				actions={
					<div className="flex items-center gap-2">
						<Button size="sm" onClick={openCreate}>
							<Plus className="size-3.5" />
							新建公告
						</Button>
						<Button variant="outline" size="sm" onClick={() => invalidate()}>
							<RefreshCw className="size-3.5" />
							刷新
						</Button>
					</div>
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

			<Dialog open={modalOpen} onOpenChange={(v) => !submitting && setModalOpen(v)}>
				<DialogContent className="max-h-[85vh] max-w-[700px] overflow-y-auto">
					<DialogHeader>
						<DialogTitle>{editRecord ? '编辑公告' : '新建公告'}</DialogTitle>
					</DialogHeader>
					<Form {...form}>
						<form onSubmit={form.handleSubmit(handleSubmit)} className="flex flex-col gap-3">
							<FormField
								control={form.control}
								name="title"
								render={({ field }) => (
									<FormItem>
										<FormLabel>标题</FormLabel>
										<FormControl>
											<Input placeholder="请输入公告标题" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="content"
								render={({ field }) => (
									<FormItem>
										<FormLabel>{isIframe ? '嵌入 URL（iframe src）' : '内容（支持 Markdown）'}</FormLabel>
										<FormControl>
											<Textarea
												rows={isIframe ? 2 : 6}
												placeholder={isIframe ? 'https://example.com/page' : '请输入公告内容，支持 Markdown 格式'}
												{...field}
											/>
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<div className="grid grid-cols-2 gap-3">
								<FormField
									control={form.control}
									name="status"
									render={({ field }) => (
										<FormItem>
											<FormLabel>状态</FormLabel>
											<Select value={field.value} onValueChange={field.onChange}>
												<SelectTrigger className="w-full">
													<SelectValue />
												</SelectTrigger>
												<SelectContent>
													<SelectItem value="draft">草稿</SelectItem>
													<SelectItem value="published">已发布</SelectItem>
												</SelectContent>
											</Select>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={form.control}
									name="display_type"
									render={({ field }) => (
										<FormItem>
											<FormLabel>展示类型</FormLabel>
											<Select value={String(field.value)} onValueChange={(v) => field.onChange(Number(v))}>
												<SelectTrigger className="w-full">
													<SelectValue />
												</SelectTrigger>
												<SelectContent>
													<SelectItem value="1">Toast</SelectItem>
													<SelectItem value="2">Alert</SelectItem>
													<SelectItem value="3">Modal</SelectItem>
													<SelectItem value="4">iframe 嵌入</SelectItem>
												</SelectContent>
											</Select>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={form.control}
									name="sort"
									render={({ field }) => (
										<FormItem>
											<FormLabel>排序</FormLabel>
											<FormControl>
												<Input type="number" min={0} {...field} />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={form.control}
									name="expires_at"
									render={({ field }) => (
										<FormItem>
											<FormLabel>过期时间</FormLabel>
											<FormControl>
												<Input type="datetime-local" {...field} />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
							</div>
							<FormField
								control={form.control}
								name="position"
								render={({ field }) => (
									<FormItem>
										<FormLabel>Toast 位置</FormLabel>
										<Select
											value={field.value || '__none__'}
											onValueChange={(v) => field.onChange(v === '__none__' ? '' : v)}
										>
											<SelectTrigger className="w-full">
												<SelectValue placeholder="不指定" />
											</SelectTrigger>
											<SelectContent>
												<SelectItem value="__none__">不指定</SelectItem>
												{POSITIONS.map((p) => (
													<SelectItem key={p.value} value={p.value}>
														{p.label}
													</SelectItem>
												))}
											</SelectContent>
										</Select>
										<FormDescription>仅 display_type=Toast 时生效</FormDescription>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="is_pinned"
								render={({ field }) => (
									<FormItem className="flex flex-row items-center justify-between">
										<FormLabel>置顶</FormLabel>
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
								<Button type="submit" size="sm" loading={submitting}>
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
