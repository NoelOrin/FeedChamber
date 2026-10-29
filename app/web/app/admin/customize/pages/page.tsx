'use client';

import { type CustomPage, type CustomPageStatus, customizeService } from '@feedchamber/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';

import { Alert, AlertDescription } from '~/components/ui/alert';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable } from '~/components/ui/data-table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { PageHeader } from '~/components/ui/page-header';
import { Textarea } from '~/components/ui/textarea';
import { toast } from '~/lib/toast';

export default function CustomPagesPage() {
	const [open, setOpen] = useState(false);
	const [editing, setEditing] = useState<CustomPage | null>(null);
	const [slug, setSlug] = useState('');
	const [title, setTitle] = useState('');
	const [content, setContent] = useState('');
	const [status, setStatus] = useState<CustomPageStatus>('draft');
	const [saving, setSaving] = useState(false);
	const [confirm, confirmDialog] = useConfirm();
	const queryClient = useQueryClient();

	const { data, isPending } = useQuery({
		queryKey: ['admin', 'customize', 'pages'],
		queryFn: () => customizeService.listCustomPages({ page: 1, pageSize: 100 }),
	});
	const pages = data?.data ?? [];

	const invalidate = useCallback(
		() => queryClient.invalidateQueries({ queryKey: ['admin', 'customize', 'pages'] }),
		[queryClient],
	);

	const openCreate = useCallback(() => {
		setEditing(null);
		setSlug('');
		setTitle('');
		setContent('');
		setStatus('draft');
		setOpen(true);
	}, []);

	const openEdit = useCallback((p: CustomPage) => {
		setEditing(p);
		setSlug(p.slug);
		setTitle(p.title);
		setContent(p.content);
		setStatus(p.status);
		setOpen(true);
	}, []);

	const save = useCallback(async () => {
		if (!slug.trim() || !title.trim()) {
			toast.error('slug 与标题不能为空');
			return;
		}
		setSaving(true);
		try {
			const body = { slug: slug.trim(), title, content, status };
			const res = editing
				? await customizeService.updateCustomPage(editing.id, body)
				: await customizeService.createCustomPage(body);
			if (res?.applied) toast.success('已保存并发布');
			else toast.success(`已保存为草稿${res?.reason ? `：${res.reason}` : ''}`);
			setOpen(false);
			await invalidate();
		} catch (err) {
			toast.error((err as Error).message);
		} finally {
			setSaving(false);
		}
	}, [slug, title, content, status, editing, invalidate]);

	const publish = useCallback(
		async (p: CustomPage) => {
			try {
				await customizeService.publishCustomPage(p.id, p.status !== 'published');
				toast.success(p.status === 'published' ? '已取消发布' : '已发布');
				await invalidate();
			} catch (err) {
				toast.error((err as Error).message);
			}
		},
		[invalidate],
	);

	const remove = useCallback(
		(p: CustomPage) => {
			// useConfirm 是回调式的：确认后执行 onConfirm，不返回 Promise。
			confirm({
				title: '删除页面',
				description: `确定删除「${p.title}」吗？访问 /p/${p.slug} 会变成 404。`,
				danger: true,
				onConfirm: async () => {
					try {
						await customizeService.deleteCustomPage(p.id);
						toast.success('已删除');
						await invalidate();
					} catch (err) {
						toast.error((err as Error).message);
					}
				},
			});
		},
		[confirm, invalidate],
	);

	const columns = useMemo<ColumnDef<CustomPage>[]>(
		() => [
			{
				accessorKey: 'title',
				header: '标题',
				cell: ({ row }) => <span className="text-sm text-foreground">{row.original.title}</span>,
			},
			{
				accessorKey: 'slug',
				header: '地址',
				cell: ({ row }) => (
					<a
						href={`/p/${row.original.slug}`}
						target="_blank"
						rel="noreferrer"
						className="font-mono text-xs text-muted-foreground hover:text-foreground"
					>
						/p/{row.original.slug}
					</a>
				),
			},
			{
				accessorKey: 'status',
				header: '状态',
				cell: ({ row }) =>
					row.original.status === 'published' ? (
						<Badge variant="outline">已发布</Badge>
					) : (
						<Badge variant="outline">草稿</Badge>
					),
			},
			{
				id: 'actions',
				header: '操作',
				cell: ({ row }) => (
					<div className="flex gap-2">
						<Button variant="ghost" size="sm" onClick={() => openEdit(row.original)}>
							编辑
						</Button>
						<Button variant="ghost" size="sm" onClick={() => publish(row.original)}>
							{row.original.status === 'published' ? '取消发布' : '发布'}
						</Button>
						<Button variant="ghost" size="sm" onClick={() => remove(row.original)}>
							删除
						</Button>
					</div>
				),
			},
		],
		[openEdit, publish, remove],
	);

	return (
		<div className="flex flex-col gap-4">
			<PageHeader
				title="自定义页面"
				meta={`${pages.length} 个`}
				actions={<Button onClick={openCreate}>新建页面</Button>}
			/>

			<Alert>
				<AlertDescription>
					slug 不能与站点既有路由冲突（如 admin、timeline、spaces）。草稿状态对访客表现为 404。 通过 MCP
					写入时默认存为草稿，需在此发布。
				</AlertDescription>
			</Alert>

			<DataTable columns={columns} data={pages} loading={isPending} emptyText="还没有自定义页面" />

			<Dialog open={open} onOpenChange={setOpen}>
				<DialogContent className="max-w-2xl">
					<DialogHeader>
						<DialogTitle>{editing ? '编辑页面' : '新建页面'}</DialogTitle>
					</DialogHeader>
					<div className="flex flex-col gap-3">
						<div className="grid grid-cols-2 gap-3">
							<div className="flex flex-col gap-1.5">
								<Label>白名单 slug</Label>
								<Input
									value={slug}
									onChange={(e) => setSlug(e.target.value)}
									placeholder="about"
									className="font-mono text-xs"
								/>
							</div>
							<div className="flex flex-col gap-1.5">
								<Label>标题</Label>
								<Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="关于本站" />
							</div>
						</div>
						<div className="flex flex-col gap-1.5">
							<Label>正文（Markdown）</Label>
							<Textarea
								rows={14}
								className="font-mono text-xs"
								value={content}
								onChange={(e) => setContent(e.target.value)}
							/>
						</div>
					</div>
					<DialogFooter>
						<Button variant="outline" onClick={() => setOpen(false)}>
							取消
						</Button>
						<Button onClick={save} disabled={saving}>
							{saving ? '保存中…' : '保存'}
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
			{confirmDialog}
		</div>
	);
}
