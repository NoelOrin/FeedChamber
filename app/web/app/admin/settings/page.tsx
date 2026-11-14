'use client';

import { settingService } from '@feedchamber/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useState } from 'react';

import { Button } from '~/components/ui/button';
import { Collapse, CollapseContent, CollapseItem, CollapseTrigger } from '~/components/ui/collapse';
import { ConfirmDialog } from '~/components/ui/confirm-dialog';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { PageHeader } from '~/components/ui/page-header';
import { Segmented } from '~/components/ui/segmented';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Skeleton } from '~/components/ui/skeleton';
import { Switch } from '~/components/ui/switch';
import { useNavigate, useSearch } from '~/lib/admin/navigation';
import { Save, Search } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';
import { DashboardAccount } from './modules/dashboard-account';
import { DashboardLanguage } from './modules/dashboard-language';
import { DashboardPassword } from './modules/dashboard-password';
import { EmailTestPanel } from './modules/email-test-panel';
import { type SettingItemDef, serviceTabs, settingsGroups } from './settingsConfig';

const serviceTabKeys = serviceTabs.map((t) => t.key) as string[];

/* 纯函数：只读 settingsConfig 里的声明式配置，不碰组件 state。
   放在模块级，这样 useEffect 依赖数组里不必（也不该）出现它们。 */
const isBooleanKey = (key: string) =>
	key.includes('enabled') || key.includes('required') || key.includes('approval') || key.includes('use_ssl');

/** 某分组「实际包含的设置项」：有 subgroups 时取所有子分组 items，否则取 items。 */
function groupItems(group: (typeof settingsGroups)[number]): SettingItemDef[] {
	if (group.subgroups && group.subgroups.length > 0) {
		return group.subgroups.flatMap((s) => s.items);
	}
	return group.items;
}

/** 所有分组（含子分组）的 key 全集，用于 buildDefaults。 */
function allItems(): SettingItemDef[] {
	return settingsGroups.flatMap(groupItems);
}

function buildDefaults() {
	const defaultValues: Record<string, any> = {};
	allItems().forEach((item) => {
		defaultValues[item.key] = isBooleanKey(item.key) ? item.value === 'true' : item.value;
	});
	return defaultValues;
}

function normalize(data: any): Record<string, any> {
	const settingsMap: Record<string, any> = {};
	if (Array.isArray(data)) {
		data.forEach((item: any) => {
			settingsMap[item.key] = item.value;
		});
	} else {
		Object.assign(settingsMap, data);
	}
	for (const [k, v] of Object.entries(settingsMap)) {
		if (isBooleanKey(k) && typeof v === 'string') {
			settingsMap[k] = v === 'true';
		}
	}
	// 与默认值合并，确保后端未返回的字段也能正确回填（尤其布尔开关）
	return { ...buildDefaults(), ...settingsMap };
}

/** Tab 名映射，供搜索结果标注所属 Tab。 */
const tabLabelOf = (tab: string) => serviceTabs.find((t) => t.key === tab)?.label ?? tab;

/*
 * 设置页：40+ 个 key 由 settingsConfig 声明式驱动。
 *
 * 为什么不整页一个保存：每个分组（OAuth 每个平台）独立保存，避免一次误提交
 * 全部字段。dirty 跟踪到「分组」粒度，切换 Tab 时若有未保存改动弹二次确认。
 *
 * 为什么不用 react-hook-form：设置项分散在多个 Tab，切 Tab 不挂载的项就会从
 * 提交里消失。这里用受控 state（values + onChange）读写全部 key，与 Tab 是否
 * 挂载无关；dirty 也基于同一份 values 与「已保存快照」比较得出。
 */
export default function AdminSettings() {
	const [ready, setReady] = useState(false);
	/** 服务端已保存的值快照：用于判定某分组是否 dirty 与「放弃改动」时回滚。 */
	const [saved, setSaved] = useState<Record<string, any> | null>(null);
	const [values, setValues] = useState<Record<string, any> | null>(null);
	const [savingGroup, setSavingGroup] = useState<string | null>(null);
	const [search, setSearch] = useState('');
	const [pendingTab, setPendingTab] = useState<string | null>(null);
	const navigate = useNavigate();
	const { tab = 'site' } = useSearch();

	const queryClient = useQueryClient();

	const { data: remoteSettings } = useQuery({
		queryKey: qk.admin.settings.list,
		queryFn: () => settingService.getAll(),
	});

	useEffect(() => {
		if (remoteSettings === undefined) return;
		const norm = remoteSettings ? normalize(remoteSettings) : buildDefaults();
		setSaved(norm);
		setValues(norm);
		setReady(true);
	}, [remoteSettings]);

	const invalidate = () => queryClient.invalidateQueries({ queryKey: qk.admin.settings.all });

	function setValue(key: string, next: unknown) {
		setValues((prev) => (prev ? { ...prev, [key]: next } : prev));
	}

	/** 某分组（或子分组）的 keys 中是否有与已保存值不同的项。 */
	const isGroupDirty = useCallback(
		(items: SettingItemDef[]): boolean => {
			if (!values || !saved) return false;
			return items.some((it) => values[it.key] !== saved[it.key]);
		},
		[values, saved],
	);

	/** 当前 Tab 下是否有任何未保存改动。 */
	const hasUnsavedInTab = useCallback(
		(tabKey: string): boolean => {
			const groups = settingsGroups.filter((g) => g.tab === tabKey);
			return groups.some((g) => {
				if (g.subgroups && g.subgroups.length > 0) {
					return g.subgroups.some((s) => isGroupDirty(s.items));
				}
				return isGroupDirty(g.items);
			});
		},
		[isGroupDirty],
	);

	/** 保存一组设置（只提交本组 key）。 */
	async function saveGroup(groupId: string, items: SettingItemDef[]) {
		if (!values) return;
		setSavingGroup(groupId);
		try {
			const payload: Record<string, string> = {};
			for (const it of items) payload[it.key] = String(values[it.key]);
			await settingService.batchUpdate(payload);
			// 更新已保存快照：仅把本组 key 同步过去，其它组保持原样。
			setSaved((prev) => {
				if (!prev) return prev;
				const next = { ...prev };
				for (const it of items) next[it.key] = values[it.key];
				return next;
			});
			await invalidate();
			toast.success('已保存');
		} catch (error: any) {
			toast.error(error.message || '保存失败');
		} finally {
			setSavingGroup(null);
		}
	}

	/** 切换 Tab：有未保存改动则弹确认。 */
	function requestTabChange(nextTab: string) {
		if (nextTab === tab) return;
		if (hasUnsavedInTab(tab)) {
			setPendingTab(nextTab);
			return;
		}
		navigate({ to: '/admin/settings', search: { tab: nextTab } });
	}

	function confirmLeave() {
		if (pendingTab) {
			// 放弃当前 Tab 的未保存改动：把当前 Tab 的 key 回滚到已保存快照。
			if (values && saved) {
				const revert: Record<string, any> = { ...values };
				settingsGroups
					.filter((g) => g.tab === tab)
					.forEach((g) => {
						groupItems(g).forEach((it) => {
							revert[it.key] = saved[it.key];
						});
					});
				setValues(revert);
			}
			navigate({ to: '/admin/settings', search: { tab: pendingTab } });
		}
		setPendingTab(null);
	}

	function renderControl(item: SettingItemDef) {
		const value = values?.[item.key];
		if (item.type === 'select' && item.options) {
			return (
				<Select value={String(value ?? '')} onValueChange={(v) => setValue(item.key, v)}>
					<SelectTrigger className="w-full">
						<SelectValue placeholder="选择" />
					</SelectTrigger>
					<SelectContent>
						{item.options.map((opt) => (
							<SelectItem key={opt.value} value={opt.value}>
								{opt.label}
							</SelectItem>
						))}
					</SelectContent>
				</Select>
			);
		}
		if (item.type === 'boolean' || isBooleanKey(item.key)) {
			return <Switch checked={Boolean(value)} onCheckedChange={(v) => setValue(item.key, v)} />;
		}
		if (item.type === 'secret') {
			return (
				<Input
					type="password"
					placeholder={item.description}
					value={String(value ?? '')}
					onChange={(e) => setValue(item.key, e.target.value)}
				/>
			);
		}
		return (
			<Input
				placeholder={item.description}
				value={String(value ?? '')}
				onChange={(e) => setValue(item.key, e.target.value)}
			/>
		);
	}

	function renderFieldGrid(items: SettingItemDef[]) {
		return (
			<div className="grid grid-cols-2 gap-4">
				{items.map((item) => (
					<div key={item.key} className="flex flex-col gap-1.5">
						<Label htmlFor={`setting-${item.key}`}>{item.description}</Label>
						{renderControl(item)}
					</div>
				))}
			</div>
		);
	}

	/** 一个独立保存的区块（分组 / 子分组 / 搜索结果分组），底部带保存按钮；可选 group.footer 附加面板。 */
	function renderSavableBlock(groupId: string, items: SettingItemDef[], footer?: 'email-test') {
		const dirty = isGroupDirty(items);
		const saving = savingGroup === groupId;
		return (
			<div className="flex flex-col gap-3">
				{renderFieldGrid(items)}
				{footer === 'email-test' ? <EmailTestPanel /> : null}
				<div className="flex justify-end">
					<Button size="sm" disabled={!dirty || saving} loading={saving} onClick={() => saveGroup(groupId, items)}>
						<Save className="size-3.5" />
						保存
					</Button>
				</div>
			</div>
		);
	}

	function renderServiceGroups(tabKey: string) {
		if (!ready) return <Skeleton className="h-64 w-full" />;
		const groups = settingsGroups.filter((g) => g.tab === tabKey);
		return (
			<div className="flex flex-col gap-3">
				{groups.map((group) => {
					// 有子分组：父层按子分组各自折叠 + 各自保存（OAuth 按平台）。
					if (group.subgroups && group.subgroups.length > 0) {
						return (
							<div key={group.name} className="flex flex-col gap-2">
								<h2 className="text-[13px] font-medium text-foreground">{group.name}</h2>
								<Collapse defaultOpen={group.subgroups.map((s) => `${group.name}/${s.name}`)}>
									{group.subgroups.map((sub) => {
										const value = `${group.name}/${sub.name}`;
										return (
											<CollapseItem key={value} value={value}>
												<CollapseTrigger>{sub.name}</CollapseTrigger>
												<CollapseContent>{renderSavableBlock(`${group.name}/${sub.name}`, sub.items)}</CollapseContent>
											</CollapseItem>
										);
									})}
								</Collapse>
							</div>
						);
					}
					return (
						<Collapse key={group.name} defaultOpen={[group.name]}>
							<CollapseItem value={group.name}>
								<CollapseTrigger>{group.name}</CollapseTrigger>
								<CollapseContent>{renderSavableBlock(group.name, group.items, group.footer)}</CollapseContent>
							</CollapseItem>
						</Collapse>
					);
				})}
			</div>
		);
	}

	/** 搜索结果：跨所有 Tab 按 key / description 匹配，按所属分组聚合展示（带保存按钮）。 */
	const searchResults = useMemo(() => {
		const kw = search.trim().toLowerCase();
		if (!kw) return null;
		type Hit = { groupId: string; title: string; tabKey: string; items: SettingItemDef[] };
		const hits: Hit[] = [];
		for (const group of settingsGroups) {
			if (group.subgroups && group.subgroups.length > 0) {
				for (const sub of group.subgroups) {
					const matched = sub.items.filter(
						(it) => it.key.toLowerCase().includes(kw) || (it.description ?? '').toLowerCase().includes(kw),
					);
					if (matched.length > 0) {
						hits.push({
							groupId: `${group.name}/${sub.name}`,
							title: `${group.name} · ${sub.name}`,
							tabKey: group.tab,
							items: matched,
						});
					}
				}
			} else {
				const matched = group.items.filter(
					(it) => it.key.toLowerCase().includes(kw) || (it.description ?? '').toLowerCase().includes(kw),
				);
				if (matched.length > 0) {
					hits.push({ groupId: group.name, title: group.name, tabKey: group.tab, items: matched });
				}
			}
		}
		return hits;
	}, [search]);

	function renderSearchResults() {
		if (!ready) return <Skeleton className="h-64 w-full" />;
		if (!searchResults || searchResults.length === 0) {
			return <p className="text-sm text-muted-foreground">没有匹配的设置项。</p>;
		}
		return (
			<div className="flex flex-col gap-3">
				{searchResults.map((hit) => (
					<Collapse key={hit.groupId} defaultOpen={[hit.groupId]}>
						<CollapseItem value={hit.groupId}>
							<CollapseTrigger>
								{hit.title}
								<span className="ml-2 text-xs font-normal text-muted-foreground">（{tabLabelOf(hit.tabKey)}）</span>
							</CollapseTrigger>
							<CollapseContent>{renderSavableBlock(hit.groupId, hit.items)}</CollapseContent>
						</CollapseItem>
					</Collapse>
				))}
			</div>
		);
	}

	const allTabs = [
		...serviceTabs.map((t) => ({ value: t.key, label: t.label })),
		{ value: 'dashboard', label: '后台设置' },
	];
	const isServiceTab = serviceTabKeys.includes(tab);

	return (
		<div>
			<PageHeader title="系统设置" />

			{isServiceTab && (
				<div className="mb-4 flex flex-col gap-3">
					<div className="overflow-x-auto">
						<Segmented label="设置分类" value={tab} options={allTabs} onValueChange={requestTabChange} />
					</div>
					<div className="relative max-w-sm">
						<Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
						<Input
							className="pl-8"
							placeholder="搜索设置项（跨所有分类）"
							value={search}
							onChange={(e) => setSearch(e.target.value)}
						/>
					</div>
				</div>
			)}

			{tab === 'dashboard' ? (
				<div className="flex max-w-[520px] flex-col gap-6">
					<section>
						<h2 className="mb-2 text-[13px] font-medium text-foreground">语言设置</h2>
						<DashboardLanguage />
					</section>
					<section>
						<h2 className="mb-2 text-[13px] font-medium text-foreground">账号信息</h2>
						<DashboardAccount />
					</section>
					<section>
						<h2 className="mb-2 text-[13px] font-medium text-foreground">修改密码</h2>
						<DashboardPassword />
					</section>
				</div>
			) : search.trim() ? (
				renderSearchResults()
			) : (
				renderServiceGroups(tab)
			)}

			<ConfirmDialog
				open={pendingTab !== null}
				onOpenChange={(open) => {
					if (!open) setPendingTab(null);
				}}
				title="有未保存的更改"
				description="切换分类将丢弃当前未保存的修改，确定离开吗？"
				confirmText="仍要离开"
				onConfirm={confirmLeave}
			/>
		</div>
	);
}
