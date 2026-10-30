'use client';

import { customizeService, type HomeModule, type HomeModuleKind, type NavVisibility } from '@feedchamber/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useState } from 'react';

import { Alert, AlertDescription } from '~/components/ui/alert';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { PageHeader } from '~/components/ui/page-header';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Switch } from '~/components/ui/switch';
import { Textarea } from '~/components/ui/textarea';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

const KIND_LABEL: Record<HomeModuleKind, string> = {
	latest: '最新帖子',
	hot: '热门帖子',
	spaces: '版面入口',
	html: '自定义 HTML',
};

export default function HomeModulesPage() {
	const [open, setOpen] = useState(false);
	const [editing, setEditing] = useState<HomeModule | null>(null);
	const [kind, setKind] = useState<HomeModuleKind>('latest');
	const [title, setTitle] = useState('');
	const [sort, setSort] = useState(0);
	const [visibility, setVisibility] = useState<NavVisibility>('public');
	const [minLevel, setMinLevel] = useState(1);
	const [config, setConfig] = useState('');
	const [enabled, setEnabled] = useState(true);
	const [saving, setSaving] = useState(false);
	const [confirm, confirmDialog] = useConfirm();
	const queryClient = useQueryClient();

	const query = useQuery({
		queryKey: ['admin', 'customize', 'modules'],
		queryFn: async () => (await customizeService.listHomeModules()) ?? [],
	});
	const data = query.data ?? [];
	const isPending = query.isPending;
	const isError = query.isError;
	const errorMessage = query.error instanceof Error ? query.error.message : '未知错误';

	const invalidate = useCallback(async () => {
		await queryClient.invalidateQueries({ queryKey: ['admin', 'customize', 'modules'] });
		await queryClient.invalidateQueries({ queryKey: qk.customization });
	}, [queryClient]);

	const openCreate = useCallback(() => {
		setEditing(null);
		setKind('latest');
		setTitle('');
		setSort(0);
		setVisibility('public');
		setMinLevel(1);
		setConfig('');
		setEnabled(true);
		setOpen(true);
	}, []);

	const openEdit = useCallback((m: HomeModule) => {
		setEditing(m);
		setKind(m.kind);
		setTitle(m.title ?? '');
		setSort(m.sort);
		setVisibility(m.visibility);
		setMinLevel(m.min_level || 1);
		setConfig(m.config ?? '');
		setEnabled(m.enabled);
		setOpen(true);
	}, []);

	const save = useCallback(async () => {
		setSaving(true);
		try {
			const body = { kind, title, sort, visibility, min_level: minLevel, config, enabled };
			const res = editing
				? await customizeService.updateHomeModule(editing.id, body)
				: await customizeService.createHomeModule(body);
			if (res?.applied) toast.success('已保存并生效');
			else toast.success(`已保存为草稿${res?.reason ? `：${res.reason}` : ''}`);
			setOpen(false);
			await invalidate();
		} catch (err) {
			toast.error((err as Error).message);
		} finally {
			setSaving(false);
		}
	}, [kind, title, sort, visibility, minLevel, config, enabled, editing, invalidate]);

	const toggle = useCallback(
		async (m: HomeModule, next: boolean) => {
			try {
				await customizeService.updateHomeModule(m.id, {
					kind: m.kind,
					title: m.title,
					sort: m.sort,
					visibility: m.visibility,
					min_level: m.min_level,
					config: m.config,
					enabled: next,
				});
				await invalidate();
			} catch (err) {
				toast.error((err as Error).message);
			}
		},
		[invalidate],
	);

	const remove = useCallback(
		(m: HomeModule) => {
			// useConfirm 是回调式的：确认后执行 onConfirm，不返回 Promise。
			confirm({
				title: '删除首页模块',
				description: `确定删除「${m.title || KIND_LABEL[m.kind]}」吗？`,
				danger: true,
				onConfirm: async () => {
					try {
						await customizeService.deleteHomeModule(m.id);
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

	const sorted = [...data].sort((a, b) => a.sort - b.sort);

	return (
		<div className="flex flex-col gap-4">
			<PageHeader
				title="首页模块"
				meta={`${data.length} 个`}
				actions={<Button onClick={openCreate}>新增模块</Button>}
			/>

			<Alert>
				<AlertDescription>
					配置为空时首页使用内置默认布局。HTML 模块等价于一段可执行标记，通过 MCP 写入时会先存为草稿。
				</AlertDescription>
			</Alert>

			{isError ? (
				<Alert variant="destructive">
					<AlertDescription>读取失败：{errorMessage}。请刷新重试或检查权限。</AlertDescription>
				</Alert>
			) : isPending ? (
				<p className="text-sm text-muted-foreground">加载中…</p>
			) : sorted.length === 0 ? (
				<p className="text-sm text-muted-foreground">还没有自定义首页模块，前台使用默认布局。</p>
			) : (
				<div className="flex flex-col gap-2">
					{sorted.map((m) => (
						<div key={m.id} className="flex items-center gap-3 rounded-md border border-border px-3 py-2">
							<span className="w-10 shrink-0 text-xs text-muted-foreground tabular-nums">{m.sort}</span>
							<span className="min-w-0 flex-1 truncate text-sm text-foreground">{m.title || KIND_LABEL[m.kind]}</span>
							<Badge variant="outline">{KIND_LABEL[m.kind]}</Badge>
							{m.kind === 'html' ? <Badge variant="outline">高风险</Badge> : null}
							<Switch checked={m.enabled} onCheckedChange={(v) => toggle(m, v)} />
							<Button variant="ghost" size="sm" onClick={() => openEdit(m)}>
								编辑
							</Button>
							<Button variant="ghost" size="sm" onClick={() => remove(m)}>
								删除
							</Button>
						</div>
					))}
				</div>
			)}

			<Dialog open={open} onOpenChange={setOpen}>
				<DialogContent className="max-w-2xl">
					<DialogHeader>
						<DialogTitle>{editing ? '编辑首页模块' : '新增首页模块'}</DialogTitle>
					</DialogHeader>
					<div className="flex flex-col gap-3">
						<div className="grid grid-cols-2 gap-3">
							<div className="flex flex-col gap-1.5">
								<Label>类型</Label>
								<Select value={kind} onValueChange={(v) => setKind(v as HomeModuleKind)}>
									<SelectTrigger>
										<SelectValue />
									</SelectTrigger>
									<SelectContent>
										<SelectItem value="latest">最新帖子</SelectItem>
										<SelectItem value="hot">热门帖子</SelectItem>
										<SelectItem value="spaces">版面入口</SelectItem>
										<SelectItem value="html">自定义 HTML</SelectItem>
									</SelectContent>
								</Select>
							</div>
							<div className="flex flex-col gap-1.5">
								<Label>标题（可选）</Label>
								<Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="推荐阅读" />
							</div>
						</div>
						<div className="grid grid-cols-3 gap-3">
							<div className="flex flex-col gap-1.5">
								<Label>排序（小在前）</Label>
								<Input type="number" value={sort} onChange={(e) => setSort(Number(e.target.value) || 0)} />
							</div>
							<div className="flex flex-col gap-1.5">
								<Label>可见性</Label>
								<Select value={visibility} onValueChange={(v) => setVisibility(v as NavVisibility)}>
									<SelectTrigger>
										<SelectValue />
									</SelectTrigger>
									<SelectContent>
										<SelectItem value="public">所有人</SelectItem>
										<SelectItem value="login">仅登录用户</SelectItem>
										<SelectItem value="level">指定等级以上</SelectItem>
									</SelectContent>
								</Select>
							</div>
							{visibility === 'level' ? (
								<div className="flex flex-col gap-1.5">
									<Label>最低等级</Label>
									<Input
										type="number"
										min={1}
										value={minLevel}
										onChange={(e) => setMinLevel(Number(e.target.value) || 1)}
									/>
								</div>
							) : null}
						</div>
						<div className="flex flex-col gap-1.5">
							<Label>参数（JSON，可选）</Label>
							<Textarea
								rows={6}
								className="font-mono text-xs"
								placeholder={kind === 'html' ? '{"html":"<div>自定义内容</div>"}' : '{"limit": 5}'}
								value={config}
								onChange={(e) => setConfig(e.target.value)}
							/>
						</div>
						<div className="flex items-center gap-2">
							<Switch checked={enabled} onCheckedChange={setEnabled} />
							<span className="text-sm">立即生效</span>
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
