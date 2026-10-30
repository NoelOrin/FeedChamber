'use client';

import { customizeService, type Snippet, type SnippetKind, type SnippetPosition } from '@feedchamber/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';

import { AppliedBadge } from '~/components/admin/customize/applied-badge';
import { Alert, AlertDescription } from '~/components/ui/alert';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable } from '~/components/ui/data-table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { PageHeader } from '~/components/ui/page-header';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Switch } from '~/components/ui/switch';
import { Textarea } from '~/components/ui/textarea';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

/**
 * 「当前生效的 JS / CSS」汇总。
 *
 * 为什么要有这一块：站长允许注入任意 JS 之后，最需要回答的问题是
 * 「现在站点上到底在跑什么代码」。分散在列表里逐条看，等于让人自己拼。
 */
function ActiveSummary({ snippets }: { snippets: Snippet[] }) {
	const active = snippets.filter((s) => s.enabled);
	if (active.length === 0) {
		return (
			<Alert>
				<AlertDescription>当前没有任何生效中的片段。</AlertDescription>
			</Alert>
		);
	}
	const js = active.filter((s) => s.kind === 'js');
	const css = active.filter((s) => s.kind === 'css');
	const html = active.filter((s) => s.kind === 'html');
	return (
		<Alert>
			<AlertDescription>
				当前生效：{css.length} 段 CSS、{js.length} 段 JS、{html.length} 段 HTML。
				{js.length > 0 ? ' 生效中的 JS 会对全部访客执行。' : ''}
			</AlertDescription>
		</Alert>
	);
}

export default function SnippetsPage() {
	const [open, setOpen] = useState(false);
	const [editing, setEditing] = useState<Snippet | null>(null);
	const [kind, setKind] = useState<SnippetKind>('css');
	const [position, setPosition] = useState<SnippetPosition>('head');
	const [content, setContent] = useState('');
	const [sort, setSort] = useState(0);
	const [enabled, setEnabled] = useState(true);
	const [saving, setSaving] = useState(false);
	const [confirm, confirmDialog] = useConfirm();
	const queryClient = useQueryClient();

	const { data = [], isPending } = useQuery({
		queryKey: ['admin', 'customize', 'snippets'],
		queryFn: async () => (await customizeService.listSnippets()) ?? [],
	});

	const invalidate = useCallback(async () => {
		await queryClient.invalidateQueries({ queryKey: ['admin', 'customize', 'snippets'] });
		// 前台注入读的是公开聚合接口，改完必须让它失效，否则首屏还是旧片段。
		await queryClient.invalidateQueries({ queryKey: qk.customization });
	}, [queryClient]);

	const openCreate = useCallback(() => {
		setEditing(null);
		setKind('css');
		setPosition('head');
		setContent('');
		setSort(0);
		setEnabled(true);
		setOpen(true);
	}, []);

	const openEdit = useCallback((s: Snippet) => {
		setEditing(s);
		setKind(s.kind);
		setPosition(s.position);
		setContent(s.content);
		setSort(s.sort);
		setEnabled(s.enabled);
		setOpen(true);
	}, []);

	const handleSave = useCallback(async () => {
		if (!content.trim()) {
			toast.error('内容不能为空');
			return;
		}
		setSaving(true);
		try {
			// html 一律 body_end：与后端 EffectivePosition 保持一致，
			// 前端不要提交一个后端会悄悄纠正的值。
			const effectivePosition = kind === 'html' ? 'body_end' : position;
			const body = { kind, position: effectivePosition, content, sort, enabled };
			const res = editing
				? await customizeService.updateSnippet(editing.id, body)
				: await customizeService.createSnippet(body);
			// applied=false 表示后端把它存成了草稿，必须明确告诉用户，
			// 不能笼统提示「保存成功」。
			if (res?.applied) toast.success('已保存并生效');
			else toast.success(`已保存为草稿${res?.reason ? `：${res.reason}` : ''}`);
			setOpen(false);
			await invalidate();
		} catch (err) {
			toast.error((err as Error).message);
		} finally {
			setSaving(false);
		}
	}, [content, kind, position, sort, enabled, editing, invalidate]);

	const toggle = useCallback(
		async (s: Snippet, next: boolean) => {
			try {
				await customizeService.toggleSnippet(s.id, next);
				await invalidate();
			} catch (err) {
				toast.error((err as Error).message);
			}
		},
		[invalidate],
	);

	const remove = useCallback(
		(s: Snippet) => {
			// useConfirm 是回调式的：确认后执行 onConfirm，不返回 Promise。
			confirm({
				title: '删除片段',
				description: `确定删除这段 ${s.kind.toUpperCase()} 吗？此操作不可撤销。`,
				danger: true,
				onConfirm: async () => {
					try {
						await customizeService.deleteSnippet(s.id);
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

	const columns = useMemo<ColumnDef<Snippet>[]>(
		() => [
			{
				accessorKey: 'kind',
				header: '类型',
				cell: ({ row }) => <Badge variant="outline">{row.original.kind.toUpperCase()}</Badge>,
			},
			{
				accessorKey: 'position',
				header: '位置',
				cell: ({ row }) => (row.original.position === 'head' ? 'head' : 'body_end'),
			},
			{
				id: 'preview',
				header: '内容预览',
				cell: ({ row }) => (
					<code className="line-clamp-2 max-w-[380px] break-all font-mono text-xs text-muted-foreground">
						{row.original.content.slice(0, 120)}
					</code>
				),
			},
			{
				accessorKey: 'enabled',
				header: '状态',
				cell: ({ row }) => (
					<div className="flex items-center gap-2">
						<Switch checked={row.original.enabled} onCheckedChange={(v) => toggle(row.original, v)} />
						<AppliedBadge applied={row.original.enabled} />
					</div>
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
						<Button variant="ghost" size="sm" onClick={() => remove(row.original)}>
							删除
						</Button>
					</div>
				),
			},
		],
		[toggle, openEdit, remove],
	);

	return (
		<div className="flex flex-col gap-4">
			<PageHeader
				title="自定义代码"
				meta={`${data.filter((s) => s.enabled).length} / ${data.length} 生效`}
				actions={<Button onClick={openCreate}>新增片段</Button>}
			/>

			<ActiveSummary snippets={data} />

			{data.some((s) => s.enabled && s.kind !== 'css') ? (
				<Alert>
					<AlertDescription>
						JS 与 HTML 片段等价于对全站访客执行任意脚本。通过 MCP / API Key 写入时会被强制保存为草稿，
						需在此手工启用；每次写入都记录审计日志。
					</AlertDescription>
				</Alert>
			) : null}

			<DataTable columns={columns} data={data} loading={isPending} emptyText="还没有自定义片段" />

			<Dialog open={open} onOpenChange={setOpen}>
				<DialogContent className="max-w-2xl">
					<DialogHeader>
						<DialogTitle>{editing ? '编辑片段' : '新增片段'}</DialogTitle>
					</DialogHeader>
					<div className="flex flex-col gap-3">
						<div className="grid grid-cols-2 gap-3">
							<div className="flex flex-col gap-1.5">
								<Label>类型</Label>
								<Select value={kind} onValueChange={(v) => setKind(v as SnippetKind)}>
									<SelectTrigger>
										<SelectValue />
									</SelectTrigger>
									<SelectContent>
										<SelectItem value="css">CSS（直接生效）</SelectItem>
										<SelectItem value="js">JS（高风险）</SelectItem>
										<SelectItem value="html">HTML（高风险）</SelectItem>
									</SelectContent>
								</Select>
							</div>
							<div className="flex flex-col gap-1.5">
								<Label>注入位置</Label>
								{/*
								 * html 片段固定落 body_end（head 里的裸标记会在 <body> 之前
								 * 被解析，行为不可预期）。这里不提供选择，避免造出
								 * 「存了 head、永远不渲染」的状态。
								 */}
								<Select
									value={kind === 'html' ? 'body_end' : position}
									disabled={kind === 'html'}
									onValueChange={(v) => setPosition(v as SnippetPosition)}
								>
									<SelectTrigger>
										<SelectValue />
									</SelectTrigger>
									<SelectContent>
										<SelectItem value="head">head</SelectItem>
										<SelectItem value="body_end">body_end</SelectItem>
									</SelectContent>
								</Select>
							</div>
						</div>
						<div className="flex flex-col gap-1.5">
							<Label>内容</Label>
							<Textarea
								rows={12}
								className="font-mono text-xs"
								placeholder={kind === 'css' ? '/* 自定义样式 */' : '// 自定义脚本'}
								value={content}
								onChange={(e) => setContent(e.target.value)}
							/>
						</div>
						<div className="grid grid-cols-2 items-end gap-3">
							<div className="flex flex-col gap-1.5">
								<Label>排序（小在前）</Label>
								<Input type="number" value={sort} onChange={(e) => setSort(Number(e.target.value) || 0)} />
							</div>
							<div className="flex items-center gap-2 pb-2">
								<Switch checked={enabled} onCheckedChange={setEnabled} />
								<span className="text-sm">立即生效</span>
							</div>
						</div>
					</div>
					<DialogFooter>
						<Button variant="outline" onClick={() => setOpen(false)}>
							取消
						</Button>
						<Button onClick={handleSave} disabled={saving}>
							{saving ? '保存中…' : '保存'}
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
			{confirmDialog}
		</div>
	);
}
