'use client';

import { type Permission, permissionService, type Role, roleService, spaceService } from '@feedchamber/api-client';
import { useEffect, useState } from 'react';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Checkbox } from '~/components/ui/checkbox';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Label } from '~/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Separator } from '~/components/ui/separator';
import { toast } from '~/lib/toast';

interface Props {
	spaceId: string;
	spaceName: string;
	open: boolean;
	onClose: () => void;
}

/*
 * 版面权限覆盖（NGA 层级继承）。
 *
 * 三态语义：
 *   - 本级已覆盖：该版面有自己的 permissions 行，完全替换全局
 *   - 继承：无本级行，取最近祖先的覆盖
 *   - 未覆盖：整条链上都没有覆盖，回退全局角色权限
 *
 * antd 的 Select mode="multiple" 换成 Checkbox 网格（与 groups 的权限弹窗一致）：
 * 权限码固定，勾选比下拉搜索直观，也能显示已选数量。
 */
export default function SpacePermissionModal({ spaceId, spaceName, open, onClose }: Props) {
	const [roles, setRoles] = useState<Role[]>([]);
	const [permissions, setPermissions] = useState<Permission[]>([]);
	const [role, setRole] = useState<string>('');
	const [hasOverride, setHasOverride] = useState(false);
	const [selected, setSelected] = useState<string[]>([]);
	const [loading, setLoading] = useState(false);
	const [effective, setEffective] = useState<string[]>([]);
	const [sourceId, setSourceId] = useState<string>('');
	const [inherited, setInherited] = useState(false);
	const [sourceName, setSourceName] = useState<string>('');

	useEffect(() => {
		if (!open) return;
		setRole('');
		setHasOverride(false);
		setSelected([]);
		setEffective([]);
		setSourceId('');
		setInherited(false);
		setSourceName('');
		Promise.all([roleService.list(), permissionService.list()])
			.then(([r, p]) => {
				setRoles(r || []);
				setPermissions(p || []);
			})
			.catch(() => toast.error('加载角色/权限失败'));
	}, [open]);

	async function loadOverride(r: string) {
		setRole(r);
		if (!r) return;
		setLoading(true);
		try {
			const [raw, eff] = await Promise.all([
				permissionService.getSpaceRole(spaceId, r),
				permissionService.getEffectiveSpaceRole(spaceId, r).catch(() => null),
			]);
			setHasOverride(raw?.has_override ?? false);
			setSelected(raw?.permissions ?? []);
			if (eff) {
				setEffective(eff.permissions ?? []);
				setSourceId(eff.source_space_id ?? '');
				setInherited(eff.inherited ?? false);
				if (eff.source_space_id && eff.source_space_id !== spaceId) {
					spaceService
						.tree()
						.then((tree: any) => {
							const find = (nodes: any[], id: string): any => {
								for (const n of nodes) {
									if (n.id === id) return n;
									if (n.children) {
										const f = find(n.children, id);
										if (f) return f;
									}
								}
								return null;
							};
							const src = find(tree || [], eff.source_space_id);
							if (src) setSourceName(src.name);
							else setSourceName(eff.source_space_id.slice(0, 6));
						})
						.catch(() => setSourceName(eff.source_space_id.slice(0, 6)));
				} else {
					setSourceName('');
				}
			}
		} catch {
			toast.error('加载覆盖配置失败');
		} finally {
			setLoading(false);
		}
	}

	async function handleSave() {
		if (!role) {
			toast.error('请选择角色');
			return;
		}
		if (selected.length === 0) {
			toast.error('覆盖权限不能为空；如需回退全局请点「清除覆盖」');
			return;
		}
		try {
			await permissionService.syncSpaceRole(spaceId, role, selected);
			setHasOverride(true);
			const eff = await permissionService.getEffectiveSpaceRole(spaceId, role).catch(() => null);
			if (eff) {
				setEffective(eff.permissions ?? []);
				setSourceId(eff.source_space_id ?? '');
				setInherited(eff.inherited ?? false);
			}
			toast.success('版块权限覆盖已保存 — 子版面将自动继承（NGA）');
		} catch (e: any) {
			toast.error(e.message || '保存失败');
		}
	}

	async function handleClear() {
		if (!role) return;
		try {
			await permissionService.clearSpaceRole(spaceId, role);
			setHasOverride(false);
			setSelected([]);
			const eff = await permissionService.getEffectiveSpaceRole(spaceId, role).catch(() => null);
			if (eff) {
				setEffective(eff.permissions ?? []);
				setSourceId(eff.source_space_id ?? '');
				setInherited(eff.inherited ?? false);
			}
			toast.success('已清除覆盖，回退继承/全局');
		} catch (e: any) {
			toast.error(e.message || '清除失败');
		}
	}

	function handleCopyEffective() {
		if (effective.length === 0) {
			toast.error('无有效权限可复制');
			return;
		}
		setSelected([...effective]);
		toast.success('已将有效权限填入编辑区，点保存即可转为当前版面的独立覆盖');
	}

	return (
		<Dialog open={open} onOpenChange={(v) => !v && onClose()}>
			<DialogContent className="max-h-[85vh] max-w-[680px] overflow-y-auto">
				<DialogHeader>
					<DialogTitle>版块权限 — {spaceName} · NGA 继承</DialogTitle>
				</DialogHeader>

				<div className="flex flex-col gap-3">
					<div className="border-info/30 bg-info/5 text-muted-foreground rounded-md border p-3 text-[12px]">
						NGA 层级继承：子版面未设覆盖时自动继承父版面的权限；父版面覆盖会完全替换全局，子版面继承最近的祖先覆盖。
					</div>

					<div className="flex flex-wrap items-center gap-2">
						<Label htmlFor="perm-role">选择角色：</Label>
						<Select value={role} onValueChange={loadOverride}>
							<SelectTrigger id="perm-role" className="w-60">
								<SelectValue placeholder="选择角色" />
							</SelectTrigger>
							<SelectContent>
								{roles.map((r) => (
									<SelectItem key={r.name} value={r.name}>
										{r.name}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
						{role ? (
							<Badge variant={hasOverride ? 'warning' : inherited ? 'primary' : 'outline'}>
								{hasOverride
									? '本级已覆盖'
									: inherited
										? `继承自 ${sourceName || sourceId.slice(0, 6)}`
										: '未覆盖（全局）'}
							</Badge>
						) : null}
					</div>

					{role ? (
						<>
							<div className="border-border bg-muted/50 rounded-lg border p-3">
								<p className="mb-2 text-[12px] font-medium text-foreground">有效权限（实际生效，含继承）：</p>
								{effective.length ? (
									<div className="flex flex-wrap gap-1.5">
										{effective.map((c) => (
											<Badge key={c} variant="primary">
												{c}
											</Badge>
										))}
									</div>
								) : (
									<p className="text-muted-foreground text-[12px]">无（回退全局空）</p>
								)}
								{inherited && sourceId ? (
									<div className="text-muted-foreground mt-2 text-[11px]">
										继承自：
										<code className="bg-muted rounded px-1 py-0.5">{sourceName || sourceId}</code>（{sourceId}）
										<Button variant="link" size="xs" className="ml-2 text-[11px]" onClick={handleCopyEffective}>
											复制为本级覆盖
										</Button>
									</div>
								) : null}
								{!hasOverride && !inherited ? (
									<p className="text-muted-foreground mt-1.5 text-[11px]">当前未设覆盖，直接使用全局角色权限</p>
								) : null}
							</div>

							<Separator />

							<div>
								<p className="mb-2 text-[12px] font-medium text-foreground">
									本级覆盖编辑（覆盖式，完全替换；留空则继承）：
								</p>
								<div className="border-border max-h-[40vh] overflow-y-auto rounded-md border p-3">
									{loading ? (
										<p className="text-muted-foreground text-[13px]">加载中…</p>
									) : (
										<div className="grid grid-cols-2 gap-1.5">
											{permissions.map((p) => {
												const checked = selected.includes(p.code);
												return (
													<label
														key={p.code}
														htmlFor={`space-perm-${spaceId}-${p.code}`}
														className="flex cursor-pointer items-center gap-2 text-[13px] text-foreground"
													>
														<Checkbox
															id={`space-perm-${spaceId}-${p.code}`}
															checked={checked}
															onCheckedChange={(v) =>
																setSelected((prev) => (v ? [...prev, p.code] : prev.filter((c) => c !== p.code)))
															}
														/>
														<span className="truncate" title={`${p.name} (${p.code})`}>
															{p.code}
														</span>
													</label>
												);
											})}
										</div>
									)}
								</div>
								<p className="text-muted-foreground mt-1.5 text-[11px]">
									提示：保存后本版面及未设覆盖的子孙版面将使用此覆盖；清除后回退到继承/全局。已选 {selected.length} 项。
								</p>
							</div>

							<div className="flex flex-wrap gap-2">
								<Button size="sm" onClick={handleSave}>
									保存本级覆盖
								</Button>
								<Button variant="destructive" size="sm" disabled={!hasOverride} onClick={handleClear}>
									清除本级（回退继承）
								</Button>
								<Button variant="outline" size="sm" onClick={() => loadOverride(role)}>
									刷新
								</Button>
							</div>
						</>
					) : null}
				</div>

				<DialogFooter>
					<Button variant="outline" size="sm" onClick={onClose}>
						关闭
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
