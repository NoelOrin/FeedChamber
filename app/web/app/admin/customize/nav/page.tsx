'use client';

import { customizeService, type NavItem, type NavVisibility } from '@feedchamber/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useMemo, useState } from 'react';

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
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

const VISIBILITY_LABEL: Record<string, string> = {
	public: '所有人',
	login: '仅登录用户',
	level: '指定等级以上',
};

export default function NavItemsPage() {
	const [open, setOpen] = useState(false);
	const [editing, setEditing] = useState<NavItem | null>(null);
	const [label, setLabel] = useState('');
	const [href, setHref] = useState('');
	const [icon, setIcon] = useState('arrow');
	const [sort, setSort] = useState(0);
	const [visibility, setVisibility] = useState<NavVisibility>('public');
	const [minLevel, setMinLevel] = useState(1);
	const [enabled, setEnabled] = useState(true);
	const [saving, setSaving] = useState(false);
	const [confirm, confirmDialog] = useConfirm();
	const queryClient = useQueryClient();

	const { data = [], isPending } = useQuery({
		queryKey: ['admin', 'customize', 'nav'],
		queryFn: async () => (await customizeService.listNavItems()) ?? [],
	});

	const invalidate = useCallback(async () => {
		await queryClient.invalidateQueries({ queryKey: ['admin', 'customize', 'nav'] });
		await queryClient.invalidateQueries({ queryKey: qk.customization });
	}, [queryClient]);

	const openCreate = useCallback(() => {
		setEditing(null);
		setLabel('');
		setHref('');
		setIcon('arrow');
		setSort(0);
		setVisibility('public');
		setMinLevel(1);
		setEnabled(true);
		setOpen(true);
	}, []);

	const openEdit = useCallback((n: NavItem) => {
		setEditing(n);
		setLabel(n.label);
		setHref(n.href);
		setIcon(n.icon ?? 'arrow');
		setSort(n.sort);
		setVisibility(n.visibility);
		setMinLevel(n.min_level || 1);
		setEnabled(n.enabled);
		setOpen(true);
	}, []);

	const save = useCallback(async () => {
		if (!label.trim() || !href.trim()) {
			toast.error('名称与链接不能为空');
			return;
		}
		setSaving(true);
		try {
			const body = { label, href, icon, sort, visibility, min_level: minLevel, enabled };
			const res = editing
				? await customizeService.updateNavItem(editing.id, body)
				: await customizeService.createNavItem(body);
			if (res?.applied) toast.success('已保存并生效');
			else toast.success(`已保存为草稿${res?.reason ? `：${res.reason}` : ''}`);
			setOpen(false);
			await invalidate();
		} catch (err) {
			toast.error((err as Error).message);
		} finally {
			setSaving(false);
		}
	}, [label, href, icon, sort, visibility, minLevel, enabled, editing, invalidate]);

	const toggle = useCallback(
		async (n: NavItem) => {
			try {
				await customizeService.updateNavItem(n.id, {
					label: n.label,
					href: n.href,
					icon: n.icon,
					sort: n.sort,
					visibility: n.visibility,
					min_level: n.min_level,
					enabled: !n.enabled,
				});
				await invalidate();
			} catch (err) {
				toast.error((err as Error).message);
			}
		},
		[invalidate],
	);

	const remove = useCallback(
		(n: NavItem) => {
			// useConfirm 是回调式的：确认后执行 onConfirm，不返回 Promise。
			confirm({
				title: '删除导航项',
				description: `确定删除「${n.label}」吗？`,
				danger: true,
				onConfirm: async () => {
					try {
						await customizeService.deleteNavItem(n.id);
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

	const sorted = useMemo(() => [...data].sort((a, b) => a.sort - b.sort), [data]);

	return (
		<div className="flex flex-col gap-4">
			<PageHeader
				title="导航与入口"
				meta={`${data.length} 项`}
				actions={<Button onClick={openCreate}>新增导航项</Button>}
			/>

			<Alert>
				<AlertDescription>
					导航配置为空时，前台回退到内置默认入口，不会出现空白导航。通过 MCP 写入时会先存为草稿，需在此启用。
				</AlertDescription>
			</Alert>

			{isPending ? (
				<p className="text-sm text-muted-foreground">加载中…</p>
			) : sorted.length === 0 ? (
				<p className="text-sm text-muted-foreground">还没有自定义导航项，前台使用默认入口。</p>
			) : (
				<div className="flex flex-col gap-2">
					{sorted.map((n) => (
						<div key={n.id} className="flex items-center gap-3 rounded-md border border-border px-3 py-2">
							<span className="w-10 shrink-0 text-xs text-muted-foreground tabular-nums">{n.sort}</span>
							<span className="min-w-0 flex-1 truncate text-sm text-foreground">{n.label}</span>
							<span className="hidden max-w-[220px] shrink-0 truncate font-mono text-xs text-muted-foreground sm:block">
								{n.href}
							</span>
							<Badge variant="outline">{VISIBILITY_LABEL[n.visibility] ?? n.visibility}</Badge>
							{n.visibility === 'level' ? <Badge variant="outline">L{n.min_level}+</Badge> : null}
							<Switch checked={n.enabled} onCheckedChange={() => toggle(n)} />
							<Button variant="ghost" size="sm" onClick={() => openEdit(n)}>
								编辑
							</Button>
							<Button variant="ghost" size="sm" onClick={() => remove(n)}>
								删除
							</Button>
						</div>
					))}
				</div>
			)}

			<Dialog open={open} onOpenChange={setOpen}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>{editing ? '编辑导航项' : '新增导航项'}</DialogTitle>
					</DialogHeader>
					<div className="flex flex-col gap-3">
						<div className="flex flex-col gap-1.5">
							<Label>名称</Label>
							<Input value={label} onChange={(e) => setLabel(e.target.value)} placeholder="关于本站" />
						</div>
						<div className="flex flex-col gap-1.5">
							<Label>链接</Label>
							<Input value={href} onChange={(e) => setHref(e.target.value)} placeholder="/p/about" />
						</div>
						<div className="grid grid-cols-2 gap-3">
							<div className="flex flex-col gap-1.5">
								<Label>图标</Label>
								<Select value={icon} onValueChange={setIcon}>
									<SelectTrigger>
										<SelectValue />
									</SelectTrigger>
									<SelectContent>
										<SelectItem value="layers">图层</SelectItem>
										<SelectItem value="message">消息</SelectItem>
										<SelectItem value="trophy">奖杯</SelectItem>
										<SelectItem value="folder">文件夹</SelectItem>
										<SelectItem value="arrow">箭头</SelectItem>
									</SelectContent>
								</Select>
							</div>
							<div className="flex flex-col gap-1.5">
								<Label>排序（小在前）</Label>
								<Input type="number" value={sort} onChange={(e) => setSort(Number(e.target.value) || 0)} />
							</div>
						</div>
						<div className="grid grid-cols-2 gap-3">
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
