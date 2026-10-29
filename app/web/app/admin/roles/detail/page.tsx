'use client';

import {
	type Permission,
	permissionService,
	type Role,
	roleService,
	type Space,
	spaceService,
} from '@feedchamber/api-client';
import { useQueries, useQuery } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useState } from 'react';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { Checkbox } from '~/components/ui/checkbox';
import { Collapse, CollapseContent, CollapseItem, CollapseTrigger } from '~/components/ui/collapse';
import { Descriptions, DescriptionsItem } from '~/components/ui/descriptions';
import { EmptyState } from '~/components/ui/empty-state';
import { Input } from '~/components/ui/input';
import { Loading } from '~/components/ui/loading';
import { PageHeader } from '~/components/ui/page-header';
import { useNavigate, useSearch } from '~/lib/admin/navigation';
import {
	decodePermValue,
	encodeSpacePermValue,
	groupPermissions,
	permissionTone,
	SPACE_SCOPED_PERMISSION_CODES,
} from '~/lib/admin/permissions';
import { Check, PenLine as Pencil, X } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

function flattenSpaces(list: Space[]): Space[] {
	const out: Space[] = [];
	const walk = (items: any[]) => {
		for (const s of items) {
			out.push(s);
			if (Array.isArray(s.children) && s.children.length) walk(s.children);
		}
	};
	walk(list || []);
	return out;
}

/** 行内编辑字段：展示值 + 编辑按钮，编辑态给 Input + 保存/取消。 */
function InlineEdit({
	value,
	placeholder,
	onSave,
}: {
	value: string;
	placeholder: string;
	onSave: (v: string) => Promise<void>;
}) {
	const [editing, setEditing] = useState(false);
	const [draft, setDraft] = useState(value);
	const [saving, setSaving] = useState(false);

	async function commit() {
		setSaving(true);
		try {
			await onSave(draft);
			setEditing(false);
		} finally {
			setSaving(false);
		}
	}

	if (!editing) {
		return (
			<span className="inline-flex items-center gap-1.5">
				<Badge variant="secondary">{value || '-'}</Badge>
				<Button
					variant="link"
					size="xs"
					onClick={() => {
						setDraft(value);
						setEditing(true);
					}}
				>
					<Pencil className="size-3" />
					修改
				</Button>
			</span>
		);
	}

	return (
		<span className="inline-flex items-center gap-1.5">
			<Input
				className="w-[200px]"
				value={draft}
				onChange={(e) => setDraft(e.target.value)}
				placeholder={placeholder}
				disabled={saving}
				onKeyDown={(e) => {
					if (e.key === 'Enter') commit();
				}}
			/>
			<Button variant="link" size="xs" loading={saving} onClick={commit} aria-label="保存">
				<Check className="size-3" />
			</Button>
			<Button variant="link" size="xs" onClick={() => setEditing(false)} aria-label="取消">
				<X className="size-3" />
			</Button>
		</span>
	);
}

/*
 * 角色详情：全局权限 + 版面级覆盖的统一编辑界面。
 *
 * 原实现把「全局权限」和「每个版面的权限覆盖」编码进同一个多选框
 * （值形如 `<spaceId>__space__<code>`），一次 onChange 里做全量 diff 再写库。
 * 这里保留同样的编码与 diff 逻辑（后端契约不变），只把多选框换成 Checkbox 网格：
 * 权限码固定，勾选比下拉搜索更直观，也能看到每组已选数量。
 */
export default function AdminRoleDetail() {
	const navigate = useNavigate();
	const search = useSearch({ strict: false });
	const roleId = (search as any).id || '';

	const [saving, setSaving] = useState(false);

	/* 本地可编辑草稿：权限矩阵是表单，用户提交前会反复勾选，
	   不能被后台重取覆盖，所以查询结果只是初始值来源。 */
	const [role, setRole] = useState<Role | null>(null);
	const [allPermissions, setAllPermissions] = useState<Permission[]>([]);
	const [rolePerms, setRolePerms] = useState<string[]>([]);
	const [spaceOverrides, setSpaceOverrides] = useState<Record<string, string[]>>({});
	const [spaces, setSpaces] = useState<Space[]>([]);

	/* 三条基础字典并行取；角色本体从角色列表里按 id 挑出来
	   （后端没有单角色详情端点）。 */
	const roleListQuery = useQuery({
		queryKey: qk.admin.roles.list,
		queryFn: async () => (await roleService.list()) ?? [],
	});

	const permissionsQuery = useQuery({
		queryKey: qk.admin.permissions.list,
		queryFn: async () => (await permissionService.list()) ?? [],
	});

	const spacesQuery = useQuery({
		queryKey: qk.admin.spaces.list(),
		queryFn: async () => (await spaceService.tree()) ?? [],
	});

	const foundRole = (roleListQuery.data ?? []).find((r: Role) => r.id === roleId);
	const roleName = foundRole?.name ?? '';

	const rolePermsQuery = useQuery({
		queryKey: qk.admin.roles.permissions(roleName),
		queryFn: () => permissionService.getByRole(roleName),
		enabled: Boolean(roleName),
	});

	const flatSpaces = useMemo(() => flattenSpaces(spacesQuery.data ?? []), [spacesQuery.data]);

	/* 每个版面的覆盖是 N 条查询，用 useQueries 并行发起——
	   原先在同一个 Promise 链里串行 await。

	   combine 是 React Query 的官方派生钩子：它只在底层查询结果变化时重算，
	   返回的引用因此是稳定的。用它直接聚合出「有覆盖的版面」映射，
	   就不必再手写 dataUpdatedAt 拼串之类的依赖技巧，也不会有
	   「overrideQueries 每次渲染都是新数组」的问题。 */
	const serverSpaceOverrides = useQueries({
		queries: flatSpaces.map((space) => ({
			queryKey: qk.admin.spaces.rolePermission(space.id, roleName),
			queryFn: () => permissionService.getSpaceRole(space.id, roleName),
			enabled: Boolean(roleName),
		})),
		combine: (results) => {
			const overrides: Record<string, string[]> = {};
			results.forEach((q, i) => {
				if (q.data?.has_override) overrides[flatSpaces[i].id] = q.data.permissions || [];
			});
			return overrides;
		},
	});

	const loading =
		roleListQuery.isPending || permissionsQuery.isPending || spacesQuery.isPending || rolePermsQuery.isPending;

	useEffect(() => {
		if (foundRole) setRole(foundRole);
	}, [foundRole]);

	useEffect(() => {
		setAllPermissions(permissionsQuery.data ?? []);
	}, [permissionsQuery.data]);

	useEffect(() => {
		setSpaces(flatSpaces);
	}, [flatSpaces]);

	useEffect(() => {
		setRolePerms(rolePermsQuery.data?.permissions ?? []);
	}, [rolePermsQuery.data]);

	/* 服务端返回的覆盖只作为可编辑 state 的初始值来源。 */
	useEffect(() => {
		setSpaceOverrides(serverSpaceOverrides);
	}, [serverSpaceOverrides]);

	const buildSelectedValues = useCallback(
		(perms: Permission[]): string[] => {
			const values: string[] = [];
			for (const p of perms) {
				if (rolePerms.includes(p.code)) values.push(p.code);
			}
			for (const [spaceId, codes] of Object.entries(spaceOverrides)) {
				for (const p of perms) {
					if (codes.includes(p.code)) values.push(encodeSpacePermValue(spaceId, p.code));
				}
			}
			return values;
		},
		[rolePerms, spaceOverrides],
	);

	const handleSelectChange = useCallback(
		async (groupPerms: Permission[], newValues: string[]) => {
			if (!role) return;
			setSaving(true);

			const newGlobal: string[] = [];
			const newSpaceMap: Record<string, string[]> = {};

			for (const v of newValues) {
				const { spaceId, code } = decodePermValue(v);
				if (!spaceId) {
					newGlobal.push(code);
				} else {
					if (!newSpaceMap[spaceId]) newSpaceMap[spaceId] = [];
					newSpaceMap[spaceId].push(code);
				}
			}

			const otherGroupCodes = new Set(groupPerms.map((p) => p.code));
			const keptGlobal = rolePerms.filter((c) => !otherGroupCodes.has(c));
			const mergedGlobal = [...keptGlobal, ...newGlobal];

			const otherGroupCodesArr = groupPerms.map((p) => p.code);
			const keptOverrides: Record<string, string[]> = {};
			for (const [sid, codes] of Object.entries(spaceOverrides)) {
				const kept = codes.filter((c) => !otherGroupCodesArr.includes(c));
				if (kept.length > 0) keptOverrides[sid] = kept;
			}
			for (const [sid, codes] of Object.entries(newSpaceMap)) {
				if (!keptOverrides[sid]) keptOverrides[sid] = [];
				keptOverrides[sid].push(...codes);
			}

			try {
				await permissionService.sync(role.name, mergedGlobal);
				setRolePerms(mergedGlobal);

				const updatedOverrides: Record<string, string[]> = { ...keptOverrides };
				for (const [sid, codes] of Object.entries(keptOverrides)) {
					await permissionService.syncSpaceRole(sid, role.name, codes);
					updatedOverrides[sid] = codes;
				}

				for (const sid of Object.keys(spaceOverrides)) {
					if (!keptOverrides[sid]) {
						await permissionService.clearSpaceRole(sid, role.name);
					}
				}

				setSpaceOverrides(updatedOverrides);
				toast.success('权限保存成功');
			} catch (e: any) {
				toast.error(e.message || '权限保存失败');
			} finally {
				setSaving(false);
			}
		},
		[role, rolePerms, spaceOverrides],
	);

	const handleFieldSave = useCallback(
		async (field: 'name' | 'display_name' | 'description', value: string) => {
			if (!role) return;
			try {
				const updated = await roleService.update(
					role.id,
					field === 'name' ? value : role.name,
					field === 'display_name' ? value : role.display_name || role.name,
					field === 'description' ? value : role.description || '',
				);
				if (updated) setRole({ ...role, ...updated });
				toast.success('已更新');
			} catch (e: any) {
				toast.error(e.message || '更新失败');
			}
		},
		[role],
	);

	if (!roleId) {
		return (
			<div className="flex min-h-[50vh] flex-col items-center justify-center gap-3">
				<EmptyState title="缺少角色 ID" />
				<Button size="sm" onClick={() => navigate({ to: '/admin/roles' })}>
					返回角色列表
				</Button>
			</div>
		);
	}

	const groupedPermissions = groupPermissions(allPermissions);

	return (
		<div>
			<PageHeader
				title={`角色详情${role ? ` — ${role.display_name || role.name}` : ''}`}
				actions={
					<Button variant="outline" size="sm" onClick={() => navigate({ to: '/admin/roles' })}>
						← 返回
					</Button>
				}
			/>

			<Card className="mb-4 p-5">
				{loading ? (
					<Loading />
				) : role ? (
					<Descriptions>
						<DescriptionsItem label="角色 ID" span={2}>
							<code className="bg-muted rounded px-1.5 py-0.5 select-all">{role.id}</code>
						</DescriptionsItem>
						<DescriptionsItem label="显示名称">
							<InlineEdit
								value={role.display_name || role.name}
								placeholder="显示名称"
								onSave={(v) => handleFieldSave('display_name', v)}
							/>
						</DescriptionsItem>
						<DescriptionsItem label="系统名称">
							<InlineEdit value={role.name} placeholder="系统名称" onSave={(v) => handleFieldSave('name', v)} />
						</DescriptionsItem>
						<DescriptionsItem label="描述" span={2}>
							<InlineEdit
								value={role.description || ''}
								placeholder="描述"
								onSave={(v) => handleFieldSave('description', v)}
							/>
						</DescriptionsItem>
						<DescriptionsItem label="默认角色">
							{role.is_default ? <Badge variant="success">是</Badge> : '否'}
						</DescriptionsItem>
					</Descriptions>
				) : null}
			</Card>

			<Card className="mb-4 p-5">
				<h2 className="mb-2 text-[13px] font-medium text-foreground">当前权限</h2>
				<div className="flex flex-wrap gap-1.5">
					{rolePerms.map((code) => {
						const perm = allPermissions.find((p) => p.code === code);
						return (
							<Badge key={code} variant={permissionTone(code)}>
								{perm?.name || code}
							</Badge>
						);
					})}
					{Object.entries(spaceOverrides).map(([spaceId, codes]) =>
						codes.map((code) => {
							const perm = allPermissions.find((p) => p.code === code);
							const space = spaces.find((s) => s.id === spaceId);
							return (
								<Badge key={`${spaceId}:${code}`} variant="primary">
									[{space?.name || spaceId}] {perm?.name || code}
								</Badge>
							);
						}),
					)}
					{rolePerms.length === 0 && Object.keys(spaceOverrides).length === 0 ? (
						<p className="text-muted-foreground text-[13px]">暂无权限</p>
					) : null}
				</div>
			</Card>

			<Card className="p-5">
				<h2 className="mb-3 text-[13px] font-medium text-foreground">分配权限</h2>
				{loading ? (
					<Loading />
				) : (
					<Collapse defaultOpen={groupedPermissions.map(([g]) => g)}>
						{groupedPermissions.map(([group, perms]) => {
							const selected = buildSelectedValues(perms);
							const spaceScoped = perms.filter((p) => SPACE_SCOPED_PERMISSION_CODES.includes(p.code));
							return (
								<CollapseItem key={group} value={group}>
									<CollapseTrigger>
										<span className="flex items-center gap-2">
											{group}
											<span className="text-muted-foreground text-xs tabular-nums">
												({selected.length}/{perms.length + (spaces.length > 0 ? spaceScoped.length * spaces.length : 0)}
												)
											</span>
										</span>
									</CollapseTrigger>
									<CollapseContent>
										<div className="flex flex-col gap-3">
											{/* 全局权限 */}
											<div>
												<p className="text-muted-foreground mb-1.5 flex items-center gap-1.5 text-[11px]">
													<Badge variant="success">全局</Badge>
													对所有版面生效
												</p>
												<div className="grid grid-cols-2 gap-1.5">
													{perms.map((p) => {
														const checked = rolePerms.includes(p.code);
														return (
															<label
																key={p.code}
																htmlFor={`role-perm-global-${p.code}`}
																className="flex cursor-pointer items-center gap-2 text-[13px] text-foreground"
															>
																<Checkbox
																	id={`role-perm-global-${p.code}`}
																	checked={checked}
																	disabled={saving}
																	onCheckedChange={(v) => {
																		const next = v ? [...selected, p.code] : selected.filter((x) => x !== p.code);
																		handleSelectChange(perms, next);
																	}}
																/>
																<span className="truncate" title={p.code}>
																	{p.name}
																</span>
															</label>
														);
													})}
												</div>
											</div>

											{/* 版面级覆盖：仅对受版面校验的权限码提供 */}
											{spaceScoped.length > 0 && spaces.length > 0 ? (
												<div>
													<p className="text-muted-foreground mb-1.5 flex items-center gap-1.5 text-[11px]">
														<Badge variant="primary">版面</Badge>
														按版面覆盖（覆盖全局，子版面继承）
													</p>
													<div className="flex max-h-[320px] flex-col gap-2 overflow-y-auto">
														{spaces.map((space) => (
															<div key={space.id} className="border-border rounded-md border p-2">
																<p className="mb-1.5 text-[12px] font-medium text-foreground">{space.name}</p>
																<div className="grid grid-cols-2 gap-1.5">
																	{spaceScoped.map((p) => {
																		const encoded = encodeSpacePermValue(space.id, p.code);
																		const checked = (spaceOverrides[space.id] || []).includes(p.code);
																		return (
																			<label
																				key={encoded}
																				htmlFor={`role-perm-space-${space.id}-${p.code}`}
																				className="flex cursor-pointer items-center gap-2 text-[12px] text-foreground"
																			>
																				<Checkbox
																					id={`role-perm-space-${space.id}-${p.code}`}
																					checked={checked}
																					disabled={saving}
																					onCheckedChange={(v) => {
																						const next = v
																							? [...selected, encoded]
																							: selected.filter((x) => x !== encoded);
																						handleSelectChange(perms, next);
																					}}
																				/>
																				<span className="truncate" title={p.code}>
																					{p.name}
																				</span>
																			</label>
																		);
																	})}
																</div>
															</div>
														))}
													</div>
												</div>
											) : null}
										</div>
									</CollapseContent>
								</CollapseItem>
							);
						})}
					</Collapse>
				)}
			</Card>
		</div>
	);
}
