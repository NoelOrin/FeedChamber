'use client';

import {
	type AdminUser,
	adminService,
	roleService,
	type TrustLevel,
	trustService,
	userService,
	type XPAdjustment,
} from '@orinforum/api-client';
import { formatTime } from '@orinforum/shared-utils';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useMemo, useState } from 'react';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable } from '~/components/ui/data-table';
import { Descriptions, DescriptionsItem } from '~/components/ui/descriptions';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { EmptyState } from '~/components/ui/empty-state';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { Loading } from '~/components/ui/loading';
import { PageHeader } from '~/components/ui/page-header';
import { Segmented } from '~/components/ui/segmented';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import { useNavigate, useSearch } from '~/lib/admin/navigation';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

type TabKey = 'info' | 'trust' | 'posts' | 'comments' | 'reports';

export default function AdminUserDetail() {
	const navigate = useNavigate();
	const search = useSearch({ strict: false });
	const userId = (search as any).id || '';

	const [roleModal, setRoleModal] = useState(false);
	const [selectedRole, setSelectedRole] = useState('');
	const [selectedLevel, setSelectedLevel] = useState('');
	const [adjustDelta, setAdjustDelta] = useState('');
	const [adjustReason, setAdjustReason] = useState('');
	const [tab, setTab] = useState<TabKey>('info');
	const [confirm, confirmDialog] = useConfirm();

	const queryClient = useQueryClient();

	/* 三条独立查询各管一件事：用户资料、活动记录、角色字典。
	   原先三个 useEffect 各写一份 ignore 标记做竞态守卫。 */
	const { data: user = null, isPending: loading } = useQuery({
		queryKey: qk.admin.users.detail(userId),
		queryFn: async () => {
			const res = await userService.getById(userId);
			return (res as AdminUser) ?? null;
		},
		enabled: Boolean(userId),
	});

	const { data: activities = null, isPending: activitiesLoading } = useQuery({
		queryKey: qk.admin.users.activity(userId),
		queryFn: () => adminService.getUserActivities(userId, { page: 1, page_size: 10 }),
		enabled: Boolean(userId),
	});

	const { data: roles = [] } = useQuery({
		queryKey: qk.admin.roles.list,
		queryFn: async () => (await roleService.list()) ?? [],
	});

	/* XP / 等级：只在「成长」页取数，避免打开用户详情就多打三次接口。 */
	const { data: xp, isPending: xpLoading } = useQuery({
		queryKey: qk.admin.trust.userBreakdown(userId),
		queryFn: () => trustService.getUserXP(userId),
		enabled: Boolean(userId) && tab === 'trust',
	});

	const { data: adjustments } = useQuery({
		queryKey: qk.admin.trust.userAdjustments(userId),
		queryFn: () => trustService.listAdjustments(userId, { page: 1, page_size: 50 }),
		enabled: Boolean(userId) && tab === 'trust',
	});

	const { data: levels = [] } = useQuery<TrustLevel[]>({
		queryKey: qk.admin.trust.levels,
		queryFn: () => trustService.listLevels(),
	});

	/** 写操作后让该用户的详情与列表缓存失效。 */
	const invalidate = () => queryClient.invalidateQueries({ queryKey: qk.admin.users.all });

	const roleDisplay = useMemo(() => Object.fromEntries(roles.map((r) => [r.name, r.display_name])), [roles]);

	const postColumns = useMemo<ColumnDef<any>[]>(
		() => [
			{ accessorKey: 'title', header: '标题' },
			{
				accessorKey: 'status',
				header: '状态',
				size: 80,
				cell: ({ getValue }) => <Badge variant="outline">{getValue<string>()}</Badge>,
			},
			{
				accessorKey: 'created_at',
				header: '创建时间',
				size: 160,
				cell: ({ getValue }) => formatTime(getValue<string>()),
			},
			{
				id: 'action',
				header: '操作',
				size: 100,
				cell: ({ row }) => (
					<Button
						variant="link"
						size="xs"
						onClick={() => navigate({ to: '/admin/posts/detail', search: { id: row.original.id } })}
					>
						查看
					</Button>
				),
			},
		],
		[navigate],
	);

	const commentColumns = useMemo<ColumnDef<any>[]>(
		() => [
			{ accessorKey: 'body', header: '内容' },
			{ accessorKey: 'like_count', header: '点赞', size: 60 },
			{
				accessorKey: 'created_at',
				header: '创建时间',
				size: 160,
				cell: ({ getValue }) => formatTime(getValue<string>()),
			},
			{
				id: 'action',
				header: '操作',
				size: 100,
				cell: ({ row }) => (
					<Button
						variant="link"
						size="xs"
						onClick={() => navigate({ to: '/admin/comments/detail', search: { id: row.original.id } })}
					>
						查看
					</Button>
				),
			},
		],
		[navigate],
	);

	const reportColumns = useMemo<ColumnDef<any>[]>(
		() => [
			{ accessorKey: 'target_type', header: '目标类型', size: 80 },
			{ accessorKey: 'reason', header: '原因' },
			{
				accessorKey: 'status',
				header: '状态',
				size: 80,
				cell: ({ getValue }) => (
					<Badge variant={getValue<string>() === 'pending' ? 'warning' : 'primary'}>{getValue<string>()}</Badge>
				),
			},
			{
				accessorKey: 'created_at',
				header: '创建时间',
				size: 160,
				cell: ({ getValue }) => formatTime(getValue<string>()),
			},
		],
		[],
	);

	if (!userId) {
		return (
			<div className="flex min-h-[50vh] flex-col items-center justify-center gap-3">
				<EmptyState title="缺少用户 ID" />
				<Button size="sm" onClick={() => navigate({ to: '/admin/users' })}>
					返回用户列表
				</Button>
			</div>
		);
	}

	async function handleChangeRole() {
		if (!user || !selectedRole) return;
		try {
			await userService.updateRole(user.id, selectedRole);
			toast.success('角色更新成功');
			setRoleModal(false);
			invalidate();
		} catch (e: any) {
			toast.error(e.message || '角色更新失败');
		}
	}

	function handleBan() {
		if (!user) return;
		confirm({
			title: '禁言该用户？',
			description: `用户 ${user.username} 将无法发帖与评论，直到解禁。`,
			danger: true,
			confirmText: '禁言',
			onConfirm: async () => {
				try {
					await userService.ban(user.id, {});
					toast.success('禁言成功');
				} catch (e: any) {
					toast.error(e.message || '禁言失败');
				}
			},
		});
	}

	async function handleUnban() {
		if (!user) return;
		try {
			await userService.unban(user.id);
			toast.success('解禁成功');
		} catch (e: any) {
			toast.error(e.message || '解禁失败');
		}
	}

	async function handleSetLevel(manual: boolean) {
		if (!user || !selectedLevel) return;
		try {
			await trustService.setUserLevel(user.id, Number(selectedLevel), manual);
			toast.success(manual ? '等级已锁定为手工值' : '已解除手工锁定');
			queryClient.invalidateQueries({ queryKey: qk.admin.trust.userBreakdown(user.id) });
			queryClient.invalidateQueries({ queryKey: qk.admin.users.all });
		} catch (e: any) {
			toast.error(e?.message || '设置失败');
		}
	}

	async function handleAdjustXP() {
		if (!user) return;
		const delta = Number(adjustDelta);
		if (!delta) {
			toast.error('请输入非 0 的 XP 增量');
			return;
		}
		if (!adjustReason.trim()) {
			toast.error('必须填写调整原因');
			return;
		}
		try {
			await trustService.adjustXP(user.id, { delta, reason: adjustReason.trim() });
			toast.success('已调整 XP');
			setAdjustDelta('');
			setAdjustReason('');
			queryClient.invalidateQueries({ queryKey: qk.admin.trust.userBreakdown(user.id) });
			queryClient.invalidateQueries({ queryKey: qk.admin.trust.userAdjustments(user.id) });
			queryClient.invalidateQueries({ queryKey: qk.admin.users.all });
		} catch (e: any) {
			toast.error(e?.message || '调整失败');
		}
	}

	async function handleDeleteAdjustment(id: string) {
		try {
			await trustService.deleteAdjustment(id);
			toast.success('已撤销');
			queryClient.invalidateQueries({ queryKey: qk.admin.trust.userBreakdown(userId) });
			queryClient.invalidateQueries({ queryKey: qk.admin.trust.userAdjustments(userId) });
		} catch (e: any) {
			toast.error(e?.message || '撤销失败');
		}
	}

	const tabs = [
		{ value: 'info' as TabKey, label: '基本信息' },
		{ value: 'trust' as TabKey, label: '成长' },
		{ value: 'posts' as TabKey, label: `帖子 (${activities?.posts_total ?? 0})` },
		{ value: 'comments' as TabKey, label: `评论 (${activities?.comments_total ?? 0})` },
		{ value: 'reports' as TabKey, label: `被举报 (${activities?.reports_total ?? 0})` },
	];

	return (
		<div>
			<PageHeader
				title={`用户详情${user ? ` — ${user.username}` : ''}`}
				actions={
					<div className="flex flex-wrap items-center gap-2">
						<Button variant="outline" size="sm" onClick={() => navigate({ to: '/admin/users' })}>
							← 返回
						</Button>
						<Button
							variant="outline"
							size="sm"
							onClick={() => {
								if (user) {
									setSelectedRole(user.role);
									setRoleModal(true);
								}
							}}
						>
							修改角色
						</Button>
						{user?.role !== 'admin' ? (
							<>
								<Button variant="destructive" size="sm" onClick={handleBan}>
									禁言
								</Button>
								<Button variant="outline" size="sm" onClick={handleUnban}>
									解禁
								</Button>
							</>
						) : null}
					</div>
				}
			/>

			<div className="mb-4">
				<Segmented label="用户详情分区" value={tab} options={tabs} onValueChange={setTab} />
			</div>

			{tab === 'info' ? (
				<Card className="p-5">
					{loading ? (
						<Loading />
					) : user ? (
						<Descriptions>
							<DescriptionsItem label="用户 ID" span={2}>
								<code className="bg-muted rounded px-1.5 py-0.5 select-all">{user.id}</code>
							</DescriptionsItem>
							<DescriptionsItem label="用户名">{user.username}</DescriptionsItem>
							<DescriptionsItem label="显示名称">{user.display_name}</DescriptionsItem>
							<DescriptionsItem label="邮箱">{user.email}</DescriptionsItem>
							<DescriptionsItem label="角色">
								<Badge variant={user.role === 'admin' ? 'destructive' : 'primary'}>
									{roleDisplay[user.role] || user.role}
								</Badge>
							</DescriptionsItem>
							<DescriptionsItem label="头像">
								{user.avatar ? (
									<img src={user.avatar} alt={`${user.username} 的头像`} className="size-12 rounded-full" />
								) : (
									'-'
								)}
							</DescriptionsItem>
							<DescriptionsItem label="个人简介">{user.bio || '-'}</DescriptionsItem>
							<DescriptionsItem label="注册时间">{formatTime(user.created_at)}</DescriptionsItem>
							<DescriptionsItem label="更新时间">{formatTime(user.updated_at)}</DescriptionsItem>
						</Descriptions>
					) : null}
				</Card>
			) : tab === 'trust' ? (
				<div className="flex flex-col gap-4">
					<Card className="p-5">
						{xpLoading ? (
							<Loading />
						) : xp ? (
							<>
								<Descriptions>
									<DescriptionsItem label="总 XP">
										<span className="text-lg font-semibold">{xp.total_xp}</span>
									</DescriptionsItem>
									<DescriptionsItem label="当前等级">
										<Badge variant="primary">
											TL{xp.trust_level} · {xp.level_name || xp.level_key}
										</Badge>
										{xp.level_manual ? (
											<Badge variant="warning" className="ml-2">
												手工锁定
											</Badge>
										) : null}
									</DescriptionsItem>
									<DescriptionsItem label="上次评估">
										{xp.level_evaluated_at ? formatTime(xp.level_evaluated_at) : '未评估'}
									</DescriptionsItem>
									<DescriptionsItem label="保级指标" span={2}>
										近窗口活跃 {xp.metrics.active_days} 天 · 发帖 {xp.metrics.posts} · 获赞 {xp.metrics.likes} ·
										成立举报 {xp.metrics.upheld_reports}
									</DescriptionsItem>
								</Descriptions>
								<div className="mt-4 flex flex-wrap items-end gap-2 border-t border-border pt-4">
									<div className="flex w-48 flex-col gap-1.5">
										<Label htmlFor="detail-level">手工设级</Label>
										<Select value={selectedLevel} onValueChange={setSelectedLevel}>
											<SelectTrigger id="detail-level" className="w-full">
												<SelectValue placeholder="选择等级" />
											</SelectTrigger>
											<SelectContent>
												{levels
													.filter((l) => l.enabled)
													.map((l) => (
														<SelectItem key={l.level} value={String(l.level)}>
															TL{l.level} · {l.name}
														</SelectItem>
													))}
											</SelectContent>
										</Select>
									</div>
									<Button size="sm" disabled={!selectedLevel} onClick={() => handleSetLevel(true)}>
										锁定为该等级
									</Button>
									<Button variant="outline" size="sm" onClick={() => handleSetLevel(false)}>
										解除手工锁定
									</Button>
								</div>
							</>
						) : null}
					</Card>

					<Card className="p-5">
						<h2 className="mb-2 text-sm font-medium text-foreground">XP 指标分解</h2>
						<p className="mb-3 text-[13px] text-muted-foreground">
							各项之和（含人工调整）等于总 XP。修改 XP 规则后下一次求值即按新分值重算。
						</p>
						<Table>
							<TableHeader>
								<TableRow>
									<TableHead>指标</TableHead>
									<TableHead className="w-24 text-right">数量</TableHead>
									<TableHead className="w-24 text-right">单次 XP</TableHead>
									<TableHead className="w-24 text-right">小计</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{(xp?.xp_metrics ?? []).map((row) => (
									<TableRow key={row.metric}>
										<TableCell>{row.label || row.metric}</TableCell>
										<TableCell className="text-right">{row.count}</TableCell>
										<TableCell className="text-right">{row.delta}</TableCell>
										<TableCell className="text-right">{row.xp}</TableCell>
									</TableRow>
								))}
								{(xp?.xp_streaks ?? []).map((row) => (
									<TableRow key={`streak-${row.days}`}>
										<TableCell>
											连续签到 {row.days} 天（{row.hits} 次）
										</TableCell>
										<TableCell className="text-right">{row.hits}</TableCell>
										<TableCell className="text-right">{row.bonus}</TableCell>
										<TableCell className="text-right">{row.xp}</TableCell>
									</TableRow>
								))}
								<TableRow>
									<TableCell>人工 / 活动调整</TableCell>
									<TableCell className="text-right">-</TableCell>
									<TableCell className="text-right">-</TableCell>
									<TableCell className="text-right">{xp?.xp_adjustments ?? 0}</TableCell>
								</TableRow>
								<TableRow>
									<TableCell className="font-medium">合计</TableCell>
									<TableCell className="text-right">-</TableCell>
									<TableCell className="text-right">-</TableCell>
									<TableCell className="text-right font-medium">{xp?.total_xp ?? 0}</TableCell>
								</TableRow>
							</TableBody>
						</Table>
					</Card>

					<Card className="p-5">
						<h2 className="mb-2 text-sm font-medium text-foreground">人工调整 XP</h2>
						<div className="flex flex-wrap items-end gap-2">
							<div className="flex w-28 flex-col gap-1.5">
								<Label htmlFor="xp-delta">增量（可负）</Label>
								<Input
									id="xp-delta"
									type="number"
									value={adjustDelta}
									onChange={(e) => setAdjustDelta(e.target.value)}
								/>
							</div>
							<div className="flex min-w-[240px] flex-1 flex-col gap-1.5">
								<Label htmlFor="xp-reason">原因（必填）</Label>
								<Input
									id="xp-reason"
									value={adjustReason}
									onChange={(e) => setAdjustReason(e.target.value)}
									placeholder="例如：活动奖励 / 违规扣分"
								/>
							</div>
							<Button size="sm" onClick={handleAdjustXP}>
								提交调整
							</Button>
						</div>
						{(adjustments?.data ?? []).length > 0 ? (
							<Table className="mt-4">
								<TableHeader>
									<TableRow>
										<TableHead className="w-24 text-right">增量</TableHead>
										<TableHead>原因</TableHead>
										<TableHead className="w-40">时间</TableHead>
										<TableHead className="w-20">操作</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{(adjustments?.data ?? []).map((adj: XPAdjustment) => (
										<TableRow key={adj.id}>
											<TableCell className="text-right">{adj.delta > 0 ? `+${adj.delta}` : adj.delta}</TableCell>
											<TableCell>{adj.reason}</TableCell>
											<TableCell>{formatTime(adj.created_at)}</TableCell>
											<TableCell>
												<Button variant="link" size="xs" onClick={() => handleDeleteAdjustment(adj.id)}>
													撤销
												</Button>
											</TableCell>
										</TableRow>
									))}
								</TableBody>
							</Table>
						) : (
							<p className="mt-3 text-[13px] text-muted-foreground">暂无人工调整记录。</p>
						)}
					</Card>
				</div>
			) : tab === 'posts' ? (
				<DataTable
					columns={postColumns}
					data={activities?.posts || []}
					loading={activitiesLoading}
					getRowId={(row: any) => String(row.id)}
					emptyText="暂无帖子"
				/>
			) : tab === 'comments' ? (
				<DataTable
					columns={commentColumns}
					data={activities?.comments || []}
					loading={activitiesLoading}
					getRowId={(row: any) => String(row.id)}
					emptyText="暂无评论"
				/>
			) : (
				<DataTable
					columns={reportColumns}
					data={activities?.reports_about || []}
					loading={activitiesLoading}
					getRowId={(row: any) => String(row.id)}
					emptyText="无被举报记录"
				/>
			)}

			<Dialog open={roleModal} onOpenChange={(v) => !v && setRoleModal(false)}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>修改角色</DialogTitle>
					</DialogHeader>
					<div className="flex flex-col gap-3">
						<p className="text-[13px] text-muted-foreground">
							用户：<strong className="font-medium text-foreground">{user?.username}</strong>
						</p>
						<div className="flex flex-col gap-1.5">
							<Label htmlFor="detail-role">角色</Label>
							<Select value={selectedRole} onValueChange={setSelectedRole}>
								<SelectTrigger id="detail-role" className="w-full">
									<SelectValue placeholder="选择角色" />
								</SelectTrigger>
								<SelectContent>
									{roles.map((r) => (
										<SelectItem key={r.name} value={r.name}>
											{r.display_name || r.name}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
						</div>
					</div>
					<DialogFooter className="mt-2 gap-2">
						<Button variant="outline" size="sm" onClick={() => setRoleModal(false)}>
							取消
						</Button>
						<Button size="sm" onClick={handleChangeRole}>
							确认
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			{confirmDialog}
		</div>
	);
}
