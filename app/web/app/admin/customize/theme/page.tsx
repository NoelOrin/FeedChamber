'use client';

import { customizeService, type ThemeTokenMode } from '@feedchamber/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useState } from 'react';

import { Alert, AlertDescription } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
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
	const [saving, setSaving] = useState(false);
	const queryClient = useQueryClient();

	const { data = [], isPending } = useQuery({
		queryKey: ['admin', 'customize', 'tokens'],
		queryFn: async () => (await customizeService.listThemeTokens()) ?? [],
	});

	// 服务端数据到达后同步到本地草稿（避免受控输入被反复覆盖）。
	useEffect(() => {
		setDrafts(data.map((t) => ({ token: t.token, value: t.value, mode: t.mode })));
	}, [data]);

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
		setDrafts((prev) => prev.filter((_, i) => i !== index));
	}, []);

	const save = useCallback(async () => {
		const items = drafts
			.map((d) => ({ ...d, token: d.token.trim(), value: d.value.trim() }))
			.filter((d) => d.token && d.value);
		if (items.length === 0) {
			toast.error('至少填写一条有效的 token 覆盖');
			return;
		}
		setSaving(true);
		try {
			// 覆盖是「按 token 名 upsert」，因此提交的每一条都会生效或被更新。
			await customizeService.setThemeTokens(items);
			toast.success('已保存并生效');
			await invalidate();
		} catch (err) {
			toast.error((err as Error).message);
		} finally {
			setSaving(false);
		}
	}, [drafts, invalidate]);

	const reset = useCallback(async () => {
		try {
			await customizeService.clearThemeTokens();
			toast.success('已清空全部覆盖，恢复默认外观');
			await invalidate();
		} catch (err) {
			toast.error((err as Error).message);
		}
	}, [invalidate]);

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

				{isPending ? (
					<p className="text-sm text-muted-foreground">加载中…</p>
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
		</div>
	);
}
