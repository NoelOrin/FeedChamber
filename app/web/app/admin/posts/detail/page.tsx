'use client';

import { type ContentListItem, contentService, postService, userService } from '@orinforum/api-client';
import { formatTime } from '@orinforum/shared-utils';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable } from '~/components/ui/data-table';
import { Descriptions, DescriptionsItem } from '~/components/ui/descriptions';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { EmptyState } from '~/components/ui/empty-state';
import { Label } from '~/components/ui/label';
import { Loading } from '~/components/ui/loading';
import { PageHeader } from '~/components/ui/page-header';
import { Textarea } from '~/components/ui/textarea';
import { useNavigate, useSearch } from '~/lib/admin/navigation';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

const STATUS: Record<string, { text: string; variant: 'warning' | 'primary' | 'destructive' }> = {
	pending: { text: '待审', variant: 'warning' },
	approved: { text: '已通过', variant: 'primary' },
	rejected: { text: '已拒绝', variant: 'destructive' },
};

export default function PostDetailPage() {
	const navigate = useNavigate();
	const search = useSearch({ strict: false });
	const postId = (search as any).id || '';

	const [commentModal, setCommentModal] = useState<{ open: boolean; mode: 'create' | 'edit'; editId?: string }>({
		open: false,
		mode: 'create',
	});
	const [commentBody, setCommentBody] = useState('');
	const [commentLoading, setCommentLoading] = useState(false);
	const [commentPage, setCommentPage] = useState(1);
	const [commentPageSize] = useState(20);
	const [confirm, confirmDialog] = useConfirm();

	const queryClient = useQueryClient();

	const { data: post = null, isPending: loading } = useQuery({
		queryKey: qk.admin.posts.detail(postId),
		queryFn: () => postService.getDetail(postId),
		enabled: Boolean(postId),
	});

	const commentsQuery = useQuery({
		queryKey: qk.admin.comments.list({ postId, page: commentPage, pageSize: commentPageSize }),
		queryFn: () => contentService.list(postId, { page: commentPage, page_size: commentPageSize }),
		enabled: Boolean(postId),
	});

	const comments = commentsQuery.data?.data ?? [];
	const commentTotal = commentsQuery.data?.total ?? 0;
	const commentsLoading = commentsQuery.isPending;

	/** 写操作后让帖子详情与评论列表失效。 */
	const invalidate = useCallback(async () => {
		await Promise.all([
			queryClient.invalidateQueries({ queryKey: qk.admin.comments.all }),
			queryClient.invalidateQueries({ queryKey: qk.admin.posts.all }),
		]);
	}, [queryClient]);

	const openEditComment = useCallback((item: ContentListItem) => {
		setCommentBody(item.body);
		setCommentModal({ open: true, mode: 'edit', editId: item.id });
	}, []);

	const handleDeleteComment = useCallback(
		async (id: string) => {
			try {
				await contentService.delete(postId, id);
				toast.success('删除成功');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '删除失败');
			}
		},
		[postId, invalidate],
	);

	const handleBanUser = useCallback(
		async (userId: string, username: string) => {
			try {
				await userService.ban(userId, {});
				toast.success(`已禁言 ${username}`);
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '禁言失败');
			}
		},
		[invalidate],
	);

	const commentColumns = useMemo<ColumnDef<ContentListItem>[]>(
		() => [
			{
				accessorKey: 'author_name',
				header: '作者',
				size: 100,
				cell: ({ row }) => (
					<Button
						variant="link"
						size="xs"
						className="p-0"
						onClick={() => navigate({ to: '/admin/users/detail', search: { id: row.original.author_id } })}
					>
						{row.original.author_name}
					</Button>
				),
			},
			{
				accessorKey: 'body',
				header: '内容',
				cell: ({ getValue }) => (
					<p className="line-clamp-2 max-w-[400px] text-[13px] text-foreground" title={getValue<string>()}>
						{getValue<string>()}
					</p>
				),
			},
			{ accessorKey: 'like_count', header: '点赞', size: 60 },
			{
				accessorKey: 'parent_id',
				header: '类型',
				size: 80,
				cell: ({ row }) =>
					row.original.parent_id ? <Badge variant="outline">回复</Badge> : <Badge variant="primary">顶级评论</Badge>,
			},
			{
				accessorKey: 'created_at',
				header: '创建时间',
				size: 160,
				cell: ({ getValue }) => formatTime(getValue<string>()),
			},
			{
				id: 'actions',
				header: '操作',
				size: 200,
				cell: ({ row }) => {
					const record = row.original;
					return (
						<div className="flex items-center gap-1.5">
							<Button variant="outline" size="xs" onClick={() => openEditComment(record)}>
								编辑
							</Button>
							<Button
								variant="destructive"
								size="xs"
								onClick={() =>
									confirm({
										title: '删除该评论？',
										description: '评论会从帖子下移除，且不可恢复。',
										danger: true,
										confirmText: '删除',
										onConfirm: () => handleDeleteComment(record.id),
									})
								}
							>
								删除
							</Button>
							<Button
								variant="destructive"
								size="xs"
								onClick={() =>
									confirm({
										title: `禁言 ${record.author_name}？`,
										description: '该用户将无法发帖与评论，直到解禁。',
										danger: true,
										confirmText: '禁言',
										onConfirm: () => handleBanUser(record.author_id, record.author_name),
									})
								}
							>
								禁言
							</Button>
						</div>
					);
				},
			},
		],
		[confirm, navigate, openEditComment, handleDeleteComment, handleBanUser],
	);

	if (!postId) {
		return (
			<div className="flex min-h-[50vh] flex-col items-center justify-center gap-3">
				<EmptyState title="缺少帖子 ID" />
				<Button size="sm" onClick={() => navigate({ to: '/admin/posts' })}>
					返回帖子列表
				</Button>
			</div>
		);
	}

	function openCreateComment() {
		setCommentBody('');
		setCommentModal({ open: true, mode: 'create' });
	}

	async function handleCommentSubmit() {
		if (!commentBody.trim()) {
			toast.error('请输入评论内容');
			return;
		}
		setCommentLoading(true);
		try {
			if (commentModal.mode === 'create') {
				await contentService.create(postId, { body: commentBody.trim() });
				toast.success('评论添加成功');
			} else if (commentModal.mode === 'edit' && commentModal.editId) {
				await contentService.update(postId, commentModal.editId, { body: commentBody.trim() });
				toast.success('评论更新成功');
			}
			setCommentModal({ open: false, mode: 'create' });
			invalidate();
		} catch (e: any) {
			toast.error(e.message || '操作失败');
		} finally {
			setCommentLoading(false);
		}
	}

	const statusCfg = post?.status ? STATUS[post.status] : undefined;

	return (
		<div>
			<PageHeader
				title={`帖子详情${post ? ` — ${post.title}` : ''}`}
				actions={
					<Button variant="outline" size="sm" onClick={() => navigate({ to: '/admin/posts' })}>
						← 返回
					</Button>
				}
			/>

			{loading ? (
				<Card className="mb-4 py-10">
					<Loading />
				</Card>
			) : post ? (
				<Card className="mb-4 p-5">
					<Descriptions>
						<DescriptionsItem label="标题" span={2}>
							<strong className="text-[15px] font-semibold">{post.title}</strong>
						</DescriptionsItem>
						<DescriptionsItem label="作者">
							<Button
								variant="link"
								size="xs"
								className="p-0"
								onClick={() => navigate({ to: '/admin/users/detail', search: { id: post.author_id } })}
							>
								{post.author_name}
							</Button>
						</DescriptionsItem>
						<DescriptionsItem label="状态">
							{statusCfg ? <Badge variant={statusCfg.variant}>{statusCfg.text}</Badge> : post.status}
						</DescriptionsItem>
						<DescriptionsItem label="置顶">{post.pinned ? <Badge variant="primary">是</Badge> : '否'}</DescriptionsItem>
						<DescriptionsItem label="锁定">
							{post.locked ? <Badge variant="destructive">是</Badge> : '否'}
						</DescriptionsItem>
						<DescriptionsItem label="浏览数">{post.view_count}</DescriptionsItem>
						<DescriptionsItem label="评论数">{post.comment_count}</DescriptionsItem>
						<DescriptionsItem label="点赞数">{post.like_count}</DescriptionsItem>
						<DescriptionsItem label="收藏数">{post.favorite_count}</DescriptionsItem>
						<DescriptionsItem label="创建时间">{formatTime(post.created_at)}</DescriptionsItem>
						<DescriptionsItem label="更新时间">{formatTime(post.updated_at)}</DescriptionsItem>
						<DescriptionsItem label="内容摘要" span={2}>
							<span className="text-muted-foreground">{post.content_digest || '（无摘要）'}</span>
						</DescriptionsItem>
						<DescriptionsItem label="正文" span={2}>
							<p className="m-0 whitespace-pre-wrap">{post.body || '（无正文）'}</p>
						</DescriptionsItem>
					</Descriptions>
				</Card>
			) : null}

			<div className="mb-2 flex items-center justify-between">
				<h2 className="text-[13px] font-medium text-foreground">
					评论列表 <span className="text-muted-foreground tabular-nums">({post?.comment_count ?? 0})</span>
				</h2>
				<Button size="sm" onClick={openCreateComment}>
					添加评论
				</Button>
			</div>
			<DataTable
				columns={commentColumns}
				data={comments}
				loading={commentsLoading}
				getRowId={(row) => row.id}
				pagination={{
					page: commentPage,
					pageSize: commentPageSize,
					total: commentTotal,
					onChange: (p) => {
						setCommentPage(p);
						invalidate();
					},
				}}
			/>

			<Dialog
				open={commentModal.open}
				onOpenChange={(v) => !commentLoading && setCommentModal((s) => ({ ...s, open: v }))}
			>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>{commentModal.mode === 'create' ? '添加评论' : '编辑评论'}</DialogTitle>
					</DialogHeader>
					<div className="flex flex-col gap-1.5">
						<Label htmlFor="comment-body">评论内容</Label>
						<Textarea
							id="comment-body"
							rows={4}
							value={commentBody}
							onChange={(e) => setCommentBody(e.target.value)}
							placeholder="请输入评论内容"
						/>
					</div>
					<DialogFooter className="mt-2 gap-2">
						<Button variant="outline" size="sm" onClick={() => setCommentModal({ open: false, mode: 'create' })}>
							取消
						</Button>
						<Button size="sm" loading={commentLoading} onClick={handleCommentSubmit}>
							保存
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			{confirmDialog}
		</div>
	);
}
