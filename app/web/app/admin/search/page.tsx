'use client';

import { adminService, type SearchResults } from '@feedchamber/api-client';
import type { ColumnDef } from '@tanstack/react-table';
import { useState } from 'react';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { DataTable } from '~/components/ui/data-table';
import { Input } from '~/components/ui/input';
import { PageHeader } from '~/components/ui/page-header';
import { useNavigate } from '~/lib/admin/navigation';
import { Search } from '~/lib/icons';
import { toast } from '~/lib/toast';

/* 三类结果各自一张表。antd 的 Card + Table 嵌套换成「分组标题 + DataTable」，
   避免卡片里套卡片的多层描边。 */
export default function GlobalSearch() {
	const navigate = useNavigate();
	const [query, setQuery] = useState('');
	const [results, setResults] = useState<SearchResults | null>(null);
	const [loading, setLoading] = useState(false);
	const [searched, setSearched] = useState(false);

	async function handleSearch() {
		if (!query.trim()) {
			toast.error('请输入搜索关键词');
			return;
		}
		setLoading(true);
		setSearched(true);
		try {
			const res = await adminService.search({ q: query.trim(), page: 1, page_size: 10 });
			if (res) setResults(res);
		} catch (error: any) {
			toast.error(error.message || '搜索失败');
		} finally {
			setLoading(false);
		}
	}

	const userColumns: ColumnDef<any>[] = [
		{ accessorKey: 'username', header: '用户名' },
		{ accessorKey: 'email', header: '邮箱' },
		{
			accessorKey: 'role',
			header: '角色',
			cell: ({ getValue }) => {
				const role = getValue<string>();
				return <Badge variant={role === 'admin' ? 'destructive' : 'primary'}>{role}</Badge>;
			},
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
	];

	const postColumns: ColumnDef<any>[] = [
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
	];

	const commentColumns: ColumnDef<any>[] = [
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
	];

	return (
		<div>
			<PageHeader title="全局搜索" />

			<div className="mb-6 flex max-w-[600px] items-center gap-2">
				<Input
					placeholder="搜索用户、帖子、评论..."
					value={query}
					onChange={(e) => setQuery(e.target.value)}
					onKeyDown={(e) => {
						if (e.key === 'Enter') handleSearch();
					}}
					className="h-10"
				/>
				<Button size="lg" loading={loading} onClick={handleSearch}>
					<Search className="size-4" />
					搜索
				</Button>
			</div>

			{searched && !loading && !results ? <p className="text-muted-foreground text-[13px]">未找到结果</p> : null}

			{results ? (
				<div className="flex flex-col gap-6">
					<section>
						<h2 className="mb-2 text-[13px] font-medium text-foreground">
							用户 <span className="text-muted-foreground tabular-nums">({results.users.total})</span>
						</h2>
						<DataTable
							columns={userColumns}
							data={results.users.items}
							getRowId={(row: any) => String(row.id)}
							emptyText="无匹配用户"
						/>
					</section>
					<section>
						<h2 className="mb-2 text-[13px] font-medium text-foreground">
							帖子 <span className="text-muted-foreground tabular-nums">({results.posts.total})</span>
						</h2>
						<DataTable
							columns={postColumns}
							data={results.posts.items}
							getRowId={(row: any) => String(row.id)}
							emptyText="无匹配帖子"
						/>
					</section>
					<section>
						<h2 className="mb-2 text-[13px] font-medium text-foreground">
							评论 <span className="text-muted-foreground tabular-nums">({results.comments.total})</span>
						</h2>
						<DataTable
							columns={commentColumns}
							data={results.comments.items}
							getRowId={(row: any) => String(row.id)}
							emptyText="无匹配评论"
						/>
					</section>
				</div>
			) : null}
		</div>
	);
}
