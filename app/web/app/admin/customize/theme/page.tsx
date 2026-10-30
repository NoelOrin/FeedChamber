'use client';

import { customizeService, type ThemeTokenMode } from '@feedchamber/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useState } from 'react';

import { Alert, AlertDescription } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { PageHeader } from '~/components/ui/page-header';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

/**
 * 主题 token 覆盖。
 *
 * 这是「不改 CSS 也能改外观」的入口：只暴露 CSS 变量名与值，站长不需要
 * 写选择器。任意 CSS 是 /admin/customize/snippets 的职责，两者刻意分开——
 * 这个页面能安全地开放给不熟 CSS 的运营者。
 */
const COMMON_TOKENS = [
	{ token: '--primary', label: '主色' },
	{ token: '--background', label: '背景色' },
	{ token: '--foreground', label: '文字色' },
	{ token: '--accent', label: '强调色' },
	{ token: '--border', label: '边框色' },
	{ token: '--radius', label: '圆角' },
];

type Draft = { token: string; value: string; mode: ThemeTokenMode };

export default function ThemeTokensPage() {
	const [drafts, setDrafts] = useState<Draft[]>([]);
	// 已删除但尚未落库的 token：保存时真正调用 deleteThemeToken。
	const [removed, setRemoved] = useState<string[]>([]);
	const [saving, setSaving] = useState(false);
	const queryClient = useQueryClient();
	const [confirm, confirmDialog] = useConfirm();

	const query = useQuery({
		queryKey: ['admin', 'customize', 'tokens'],
		queryFn: async () => (await customizeService.listThemeTokens()) ?? [],
	});

	// 只认「已成功取到数据」的状态。此前用 `data = []` 的默认值 + [data] 依赖：
	// pending / error 期间每次渲染都会产生新数组 → effect 每次都写 state → 自旋。
	// 只在成功时同步草稿，且依赖项本身必须稳定。
	const { isSuccess, data: serverTokens } = query;
	useEffect(() => {
		if (!isSuccess) return;
		setDrafts((serverTokens ?? []).map((t) => ({ token: t.token, value: t.value, mode: t.mode })));
		setRemoved([]);
		// 依赖 serverTokens：TanStack 对同一份数据保持引用稳定，重取后是新数组，
		// 正好表达「服务端数据换了一份」。不要依赖解构默认值（那是每次新数组 → 自旋）。
	}, [isSuccess, serverTokens]);

	const invalidate = useCallback(async () => {
		await queryClient.invalidateQueries({ queryKey: ['admin', 'customize', 'tokens'] });
		await queryClient.invalidateQueries({ queryKey: qk.customization });
	}, [queryClient]);

	const update = useCallback((index: number, patch: Partial<Draft>) => {
		setDrafts((prev) => prev.map((d, i) => (i === index ? { ...d, ...patch } : d)));
	}, []);

	const addRow = useCallback((token = '') => {
		setDrafts((prev) => [...prev, { token, value: '', mode: 'all' }]);
	}, []);

	const removeRow = useCallback((index: number) => {
		setDrafts((prev) => {
			const target = prev[index];
			// 只有服务端确实存在过的 token 才需要删除请求。
			if (target?.token) setRemoved((list) => (list.includes(target.token) ? list : [...list, target.token]));
			return prev.filter((_, i) => i !== index);
		});
	}, []);

	const save = useCallback(async () => {
		const items = drafts
			.map((d) => ({ ...d, token: d.token.trim(), value: d.value.trim() }))
			.filter((d) => d.token && d.value);
		// 删除必须单独发请求：`tokens/set` 只按 token 名 upsert，
		// 「从草稿里去掉」不等于「服务端删除」——之前就是这样，
		// 删掉的覆盖在刷新后又回来了。
		const toDelete = removed.filter((token) => !items.some((it) => it.token === token));
		if (items.length === 0 && toDelete.length === 0) {
			toast.error('至少填写一条有效的 token 覆盖');
			return;
		}
		setSaving(true);
		try {
			if (items.length > 0) await customizeService.setThemeTokens(items);
			for (const token of toDelete) await customizeService.deleteThemeToken(token);
			toast.success(toDelete.length > 0 ? `已保存，并移除 ${toDelete.length} 项覆盖` : '已保存并生效');
			await invalidate();
		} catch (err) {
			toast.error((err as Error).message || '保存失败');
		} finally {
			setSaving(false);
		}
	}, [drafts, removed, invalidate]);

	// 「恢复默认」是破坏性操作：清空全部覆盖，且不可撤销。
	const reset = useCallback(() => {
		confirm({
			title: '恢复默认外观',
			description: `将清空全部 ${drafts.length} 项主题覆盖并立即生效，此操作不可撤销。`,
			confirmText: '清空全部',
			danger: true,
			onConfirm: async () => {
				try {
					await customizeService.clearThemeTokens();
					toast.success('已清空全部覆盖，恢复默认外观');
					await invalidate();
				} catch (err) {
					toast.error((err as Error).message || '操作失败');
				}
			},
		});
	}, [confirm, drafts.length, invalidate]);

	return (
		<div className="flex flex-col gap-4">
			<PageHeader
				title="主题覆盖"
				meta={`${drafts.length} 项覆盖`}
				actions={
					<div className="flex gap-2">
						<Button variant="outline" onClick={reset}>
							恢复默认
						</Button>
						<Button onClick={save} disabled={saving}>
							{saving ? '保存中…' : '保存'}
						</Button>
					</div>
				}
			/>

			<Alert>
				<AlertDescription>
					常用 token：主色 <code className="font-mono text-xs">--primary</code>、背景{' '}
					<code className="font-mono text-xs">--background</code>、圆角{' '}
					<code className="font-mono text-xs">--radius</code>。值可以是任何合法 CSS 值（如{' '}
					<code className="font-mono text-xs">#7c3aed</code> 或 <code className="font-mono text-xs">0.75rem</code>）。
				</AlertDescription>
			</Alert>

			<Card className="gap-3 p-4">
				<div className="flex flex-wrap gap-2">
					{COMMON_TOKENS.map((t) => (
						<Button key={t.token} variant="outline" size="sm" onClick={() => addRow(t.token)}>
							+ {t.label}
						</Button>
					))}
				</div>

				{query.isPending ? (
					<p className="text-sm text-muted-foreground">加载中…</p>
				) : query.isError ? (
					<Alert variant="destructive">
						<AlertDescription>
							读取主题覆盖失败：{query.error instanceof Error ? query.error.message : '未知错误'}。
							请刷新重试，或检查是否有查看站点自定义的权限。
						</AlertDescription>
					</Alert>
				) : drafts.length === 0 ? (
					<p className="text-sm text-muted-foreground">还没有任何覆盖，站点使用默认外观。</p>
				) : (
					<div className="flex flex-col gap-2">
						{drafts.map((d, i) => (
							// biome-ignore lint/suspicious/noArrayIndexKey: 行是可增删的草稿，没有稳定 id
							<div key={i} className="grid grid-cols-[1fr_1fr_140px_auto] items-end gap-2">
								<div className="flex flex-col gap-1">
									<Label className="text-xs">token</Label>
									<Input
										className="font-mono text-xs"
										value={d.token}
										placeholder="--primary"
										onChange={(e) => update(i, { token: e.target.value })}
									/>
								</div>
								<div className="flex flex-col gap-1">
									<Label className="text-xs">值</Label>
									<Input
										className="font-mono text-xs"
										value={d.value}
										placeholder="#7c3aed"
										onChange={(e) => update(i, { value: e.target.value })}
									/>
								</div>
								<div className="flex flex-col gap-1">
									<Label className="text-xs">模式</Label>
									<Select value={d.mode} onValueChange={(v) => update(i, { mode: v as ThemeTokenMode })}>
										<SelectTrigger>
											<SelectValue />
										</SelectTrigger>
										<SelectContent>
											<SelectItem value="all">明暗都适用</SelectItem>
											<SelectItem value="light">仅明亮</SelectItem>
											<SelectItem value="dark">仅暗色</SelectItem>
										</SelectContent>
									</Select>
								</div>
								<Button variant="ghost" size="sm" onClick={() => removeRow(i)}>
									删除
								</Button>
							</div>
						))}
					</div>
				)}

				<Button variant="outline" size="sm" className="self-start" onClick={() => addRow()}>
					+ 自定义 token
				</Button>
			</Card>
			{confirmDialog}
		</div>
	);
}
