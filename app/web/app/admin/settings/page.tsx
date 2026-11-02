'use client';

import { settingService } from '@feedchamber/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useState } from 'react';

import { Button } from '~/components/ui/button';
import { Collapse, CollapseContent, CollapseItem, CollapseTrigger } from '~/components/ui/collapse';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { PageHeader } from '~/components/ui/page-header';
import { Segmented } from '~/components/ui/segmented';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Skeleton } from '~/components/ui/skeleton';
import { Switch } from '~/components/ui/switch';
import { useNavigate, useSearch } from '~/lib/admin/navigation';
import { Save } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';
import { DashboardAccount } from './modules/dashboard-account';
import { DashboardLanguage } from './modules/dashboard-language';
import { DashboardPassword } from './modules/dashboard-password';
import { type SettingItemDef, serviceTabs, settingsGroups } from './settingsConfig';

const serviceTabKeys = serviceTabs.map((t) => t.key) as string[];

/* 纯函数：只读 settingsConfig 里的声明式配置，不碰组件 state。
   放在模块级，这样 useEffect 依赖数组里不必（也不该）出现它们。 */
const isBooleanKey = (key: string) =>
	key.includes('enabled') || key.includes('required') || key.includes('approval') || key.includes('use_ssl');

function buildDefaults() {
	const defaultValues: Record<string, any> = {};
	settingsGroups.forEach((group) => {
		group.items.forEach((item) => {
			defaultValues[item.key] = isBooleanKey(item.key) ? item.value === 'true' : item.value;
		});
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

/*
 * 设置页：40+ 个 key 由 settingsConfig 声明式驱动。
 *
 * 为什么不用 react-hook-form：antd Form 的 getFieldsValue(true) 能读到未挂载
 * Tab 里的字段，而 RHF 只注册已挂载字段。设置项分散在多个 Tab，切 Tab 不挂载
 * 的项就会从提交里消失——那会静默清空用户没看过的设置。这里用受控 state
 * （values + onChange）读写全部 key，与 Tab 是否挂载无关。
 */
export default function AdminSettings() {
	const [ready, setReady] = useState(false);
	const [saving, setSaving] = useState(false);
	const [values, setValues] = useState<Record<string, any> | null>(null);
	const navigate = useNavigate();
	const { tab = 'site' } = useSearch();

	const handleTabChange = useCallback(
		(key: string) => {
			navigate({ to: '/admin/settings', search: { tab: key } });
		},
		[navigate],
	);

	const queryClient = useQueryClient();

	/* 服务端设置是表单的初始值来源。React Query 取到后同步进本地 state：
	   表单编辑期间不应该被后台重取覆盖，所以用 data 变化作为同步触发器，
	   而不是把 values 直接换成 query 数据。 */
	const { data: remoteSettings } = useQuery({
		queryKey: qk.admin.settings.list,
		queryFn: () => settingService.getAll(),
	});

	useEffect(() => {
		if (remoteSettings === undefined) return;
		setValues(remoteSettings ? normalize(remoteSettings) : buildDefaults());
		// ready 只在首次加载后置真，之后保存重新拉取不再显示骨架屏
		setReady(true);
	}, [remoteSettings]);

	/** 保存成功后让设置缓存失效，下次进入页面会重新取。 */
	const invalidate = () => queryClient.invalidateQueries({ queryKey: qk.admin.settings.all });

	function setValue(key: string, next: unknown) {
		setValues((prev) => (prev ? { ...prev, [key]: next } : prev));
	}

	async function handleSave() {
		setSaving(true);
		try {
			const payload: Record<string, string> = {};
			for (const [k, v] of Object.entries(values ?? {})) {
				payload[k] = String(v);
			}
			await settingService.batchUpdate(payload);
			await invalidate();
			toast.success('设置已保存');
		} catch (error: any) {
			toast.error(error.message || '保存失败');
		} finally {
			setSaving(false);
		}
	}

	const isSecretKey = (key: string) =>
		key.includes('secret') || key.includes('password') || key.includes('access_key') || key.includes('secret_key');

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
		if (item.type === 'secret' || isSecretKey(item.key)) {
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

	function renderServiceGroups(tabKey: string) {
		if (!ready) return <Skeleton className="h-64 w-full" />;
		const groups = settingsGroups.filter((g) => g.tab === tabKey);
		return (
			<div className="flex flex-col gap-3">
				{groups.map((group) => (
					<Collapse key={group.name} defaultOpen={[group.name]}>
						<CollapseItem value={group.name}>
							<CollapseTrigger>{group.name}</CollapseTrigger>
							<CollapseContent>
								<div className="grid grid-cols-2 gap-4">
									{group.items.map((item) => (
										<div key={item.key} className="flex flex-col gap-1.5">
											<Label htmlFor={`setting-${item.key}`}>{item.description}</Label>
											{renderControl(item)}
										</div>
									))}
								</div>
							</CollapseContent>
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

	return (
		<div>
			<PageHeader
				title="系统设置"
				actions={
					serviceTabKeys.includes(tab) ? (
						<Button size="sm" loading={saving} onClick={handleSave}>
							<Save className="size-3.5" />
							保存设置
						</Button>
					) : undefined
				}
			/>

			<div className="mb-4 overflow-x-auto">
				<Segmented label="设置分类" value={tab} options={allTabs} onValueChange={handleTabChange} />
			</div>

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
			) : (
				renderServiceGroups(tab)
			)}
		</div>
	);
}
