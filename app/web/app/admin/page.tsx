'use client';

import { adminService, type SearchResults } from '@feedchamber/api-client';
import { formatTime } from '@feedchamber/shared-utils';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useMemo, useState } from 'react';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { DataTable } from '~/components/ui/data-table';
import { Input } from '~/components/ui/input';
import { PageHeader } from '~/components/ui/page-header';
import { useNavigate } from '~/lib/admin/navigation';
import { MessageSquare as Comment, FileText, RefreshCw, Search, CircleAlert as TriangleAlert, User } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

/*
 * 运营概览。
 *
 * antd 的 Row/Col + Statistic + Card 三层嵌套换成「CSS grid + 统计卡」：
 * 原来 6 张卡片各包一层 Col/Card，加上 gutter 后视觉层级比内容还重。
 * 这里统计数字直接进卡，网格用 grid-cols 表达断点（sm:2 / lg:4）。
 */

function StatCard({
	label,
	value,
	icon,
	tone = 'default',
	suffix,
}: {
	label: string;
	value: number;
	icon: React.ReactNode;
	tone?: 'default' | 'info' | 'success' | 'primary' | 'destructive';
	suffix?: string;
}) {
	const toneClass = {
		default: 'text-foreground',
		info: 'text-info',
		success: 'text-success',
		primary: 'text-brand-text',
		destructive: 'text-destructive',
	}[tone];

	return (
		<Card className="p-4">
			<div className="flex items-start justify-between gap-3">
				<div className="flex flex-col gap-1">
					<span className="text-muted-foreground text-[13px]">{label}</span>
					<span className={`text-2xl font-semibold tabular-nums ${toneClass}`}>
						{value}
						{suffix ? <span className="text-muted-foreground ml-1 text-xs font-normal">{suffix}</span> : null}
					</span>
				</div>
				<span className="text-muted-foreground shrink-0">{icon}</span>
			</div>
		</Card>
	);
}

export default function OperationsOverview() {
	const navigate = useNavigate();
	const queryClient = useQueryClient();

	const [query, setQuery] = useState('');
	const [results, setResults] = useState<SearchResults | null>(null);
	const [searchLoading, setSearchLoading] = useState(false);
	const [searched, setSearched] = useState(false);

	/* 概览数据交给 React Query：它自带请求去重、缓存与卸载中止，
	   原先手写的 ignoreRef 竞态守卫与 loading 状态都不再需要。 */
	const { data = null, isPending: loading } = useQuery({
		queryKey: qk.admin.dashboard,
		queryFn: async () => (await adminService.getDashboard()) ?? null,
	});

	const invalidate = () => queryClient.invalidateQueries({ queryKey: qk.admin.dashboard });

	async function handleSearch() {
		if (!query.trim()) {
			toast.error('请输入搜索关键词');
			return;
		}
		setSearchLoading(true);
		setSearched(true);
		try {
			const res = await adminService.search({ q: query.trim(), page: 1, page_size: 10 });
			if (res) setResults(res);
		} catch (error: any) {
			toast.error(error.message || '搜索失败');
		} finally {
			setSearchLoading(false);
		}
	}

	const dashStats = data?.stats;

	const recentUserColumns = useMemo<ColumnDef<any>[]>(
		() => [
			{ accessorKey: 'username', header: '用户名' },
			{ accessorKey: 'email', header: '邮箱' },
			{
				accessorKey: 'role',
				header: '角色',
				cell: ({ getValue }) => (
					<Badge variant={getValue<string>() === 'admin' ? 'destructive' : 'primary'}>
						{getValue<string>() === 'admin' ? '管理员' : '用户'}
					</Badge>
				),
			},
			{
				accessorKey: 'created_at',
				header: '注册时间',
				cell: ({ getValue }) => formatTime(getValue<string>()),
			},
		],
		[],
	);

	const recentPostColumns = useMemo<ColumnDef<any>[]>(
		() => [
			{ accessorKey: 'title', header: '标题' },
			{ accessorKey: 'author_id', header: '作者 ID' },
			{
				accessorKey: 'status',
				header: '状态',
				cell: ({ getValue }) => <Badge variant="outline">{getValue<string>()}</Badge>,
			},
			{
				accessorKey: 'created_at',
				header: '创建时间',
				cell: ({ getValue }) => formatTime(getValue<string>()),
			},
		],
		[],
	);

	const userColumns = useMemo<ColumnDef<any>[]>(
		() => [
			{ accessorKey: 'username', header: '用户名' },
			{ accessorKey: 'email', header: '邮箱' },
			{
				accessorKey: 'role',
				header: '角色',
				cell: ({ getValue }) => (
					<Badge variant={getValue<string>() === 'admin' ? 'destructive' : 'primary'}>{getValue<string>()}</Badge>
				),
			},
			{
				id: 'action',
				header: '操作',
				cell: ({ row }) => (
					<Button
						variant="link"
						size="xs"
						onClick={() => navigate({ to: '/admin/users/detail', search: { id: row.original.id } })}
					>
						查看
					</Button>
				),
			},
		],
		[navigate],
	);

	const postColumns = useMemo<ColumnDef<any>[]>(
		() => [
			{ accessorKey: 'title', header: '标题' },
			{ accessorKey: 'author_id', header: '作者 ID' },
			{
				id: 'action',
				header: '操作',
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
			{ accessorKey: 'body', header: '内容', size: 400 },
			{ accessorKey: 'author_id', header: '作者 ID' },
			{
				id: 'action',
				header: '操作',
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

	return (
		<div>
			<PageHeader
				title="运营概览"
				actions={
					<Button variant="outline" size="sm" loading={loading} onClick={() => void invalidate()}>
						<RefreshCw className="size-3.5" />
						刷新
					</Button>
				}
			/>

			{/* 统计卡片：4 列（lg）/ 2 列（sm） */}
			<div className="mb-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
				<StatCard label="总用户数" value={dashStats?.total_users || 0} icon={<User className="size-4" />} tone="info" />
				<StatCard
					label="总帖子数"
					value={dashStats?.total_posts || 0}
					icon={<FileText className="size-4" />}
					tone="success"
				/>
				<StatCard
					label="总评论数"
					value={dashStats?.total_comments || 0}
					icon={<Comment className="size-4" />}
					tone="primary"
				/>
				<StatCard
					label="待处理举报"
					value={data?.pending_reports || 0}
					icon={<TriangleAlert className="size-4" />}
					tone={data?.pending_reports ? 'destructive' : 'success'}
				/>
			</div>

			{/* 今日新增 */}
			<Card className="mb-4 p-4">
				<h2 className="mb-3 text-[13px] font-medium text-foreground">今日新增</h2>
				<div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
					<div className="flex items-baseline gap-2">
						<span className="text-muted-foreground text-[13px]">新用户</span>
						<span className="text-info text-xl font-semibold tabular-nums">{dashStats?.today_new_users || 0}</span>
						<span className="text-muted-foreground text-xs">人</span>
					</div>
					<div className="flex items-baseline gap-2">
						<span className="text-muted-foreground text-[13px]">新帖子</span>
						<span className="text-success text-xl font-semibold tabular-nums">{dashStats?.today_new_posts || 0}</span>
						<span className="text-muted-foreground text-xs">篇</span>
					</div>
					<div className="flex items-baseline gap-2">
						<span className="text-muted-foreground text-[13px]">新评论</span>
						<span className="text-brand-text text-xl font-semibold tabular-nums">
							{dashStats?.today_new_comments || 0}
						</span>
						<span className="text-muted-foreground text-xs">条</span>
					</div>
				</div>
			</Card>

			{/* 快捷操作 */}
			<Card className="mb-4 p-4">
				<h2 className="mb-3 text-[13px] font-medium text-foreground">快捷操作</h2>
				<div className="flex flex-wrap gap-2">
					<Button size="sm" onClick={() => navigate({ to: '/admin/users' })}>
						用户管理
					</Button>
					<Button variant="outline" size="sm" onClick={() => navigate({ to: '/admin/posts' })}>
						帖子管理
					</Button>
					<Button variant="outline" size="sm" onClick={() => navigate({ to: '/admin/comments' })}>
						评论管理
					</Button>
					<Button variant="outline" size="sm" onClick={() => navigate({ to: '/admin/reports' })}>
						举报管理 {data?.pending_reports ? `(${data.pending_reports} 条待处理)` : ''}
					</Button>
				</div>
			</Card>

			{/* 全局搜索 */}
			<Card className="mb-4 p-4">
				<h2 className="mb-3 text-[13px] font-medium text-foreground">全局搜索</h2>
				<div className="mb-4 flex max-w-[600px] items-center gap-2">
					<Input
						placeholder="搜索用户、帖子、评论..."
						value={query}
						onChange={(e) => setQuery(e.target.value)}
						onKeyDown={(e) => {
							if (e.key === 'Enter') handleSearch();
						}}
						className="h-10"
					/>
					<Button size="lg" loading={searchLoading} onClick={handleSearch}>
						<Search className="size-4" />
						搜索
					</Button>
				</div>

				{searched && !searchLoading && !results ? (
					<p className="text-muted-foreground text-[13px]">未找到结果</p>
				) : null}

				{results ? (
					<div className="flex flex-col gap-5">
						<section>
							<h3 className="mb-2 text-[13px] font-medium text-foreground">
								用户 <span className="text-muted-foreground tabular-nums">({results.users.total})</span>
							</h3>
							<DataTable
								columns={userColumns}
								data={results.users.items}
								getRowId={(row: any) => String(row.id)}
								emptyText="无匹配用户"
							/>
						</section>
						<section>
							<h3 className="mb-2 text-[13px] font-medium text-foreground">
								帖子 <span className="text-muted-foreground tabular-nums">({results.posts.total})</span>
							</h3>
							<DataTable
								columns={postColumns}
								data={results.posts.items}
								getRowId={(row: any) => String(row.id)}
								emptyText="无匹配帖子"
							/>
						</section>
						<section>
							<h3 className="mb-2 text-[13px] font-medium text-foreground">
								评论 <span className="text-muted-foreground tabular-nums">({results.comments.total})</span>
							</h3>
							<DataTable
								columns={commentColumns}
								data={results.comments.items}
								getRowId={(row: any) => String(row.id)}
								emptyText="无匹配评论"
							/>
						</section>
					</div>
				) : null}
			</Card>

			{/* 近期数据：两列 */}
			<div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
				<section>
					<h2 className="mb-2 text-[13px] font-medium text-foreground">近期注册</h2>
					<DataTable
						columns={recentUserColumns}
						data={data?.recent_users || []}
						getRowId={(row: any) => String(row.id)}
						emptyText="暂无数据"
					/>
				</section>
				<section>
					<h2 className="mb-2 text-[13px] font-medium text-foreground">近期帖子</h2>
					<DataTable
						columns={recentPostColumns}
						data={data?.recent_posts || []}
						getRowId={(row: any) => String(row.id)}
						emptyText="暂无数据"
					/>
				</section>
			</div>
		</div>
	);
}
