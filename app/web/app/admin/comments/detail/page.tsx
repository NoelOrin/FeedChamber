'use client';

import { contentService } from '@orinforum/api-client';
import { formatTime } from '@orinforum/shared-utils';
import { useQuery } from '@tanstack/react-query';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { Descriptions, DescriptionsItem } from '~/components/ui/descriptions';
import { EmptyState } from '~/components/ui/empty-state';
import { Loading } from '~/components/ui/loading';
import { PageHeader } from '~/components/ui/page-header';
import { useNavigate, useSearch } from '~/lib/admin/navigation';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

export default function CommentDetail() {
	const navigate = useNavigate();
	const search = useSearch({ strict: false });
	const commentId = (search as any).id || '';

	const [confirm, confirmDialog] = useConfirm();

	/* 手写的 AbortController 守卫由 React Query 接管：卸载时自动中止请求，
	   不需要再判断 signal.aborted 才更新状态。 */
	const { data: comment = null, isPending: loading } = useQuery({
		queryKey: qk.admin.comments.detail(commentId),
		queryFn: () => contentService.adminGetById(commentId),
		enabled: Boolean(commentId),
	});

	if (!commentId) {
		return (
			<div className="flex min-h-[50vh] flex-col items-center justify-center gap-3">
				<EmptyState title="缺少评论 ID" />
				<Button size="sm" onClick={() => navigate({ to: '/admin/comments' })}>
					返回评论列表
				</Button>
			</div>
		);
	}

	async function handleDelete() {
		try {
			await contentService.adminDelete(commentId);
			toast.success('删除成功');
			navigate({ to: '/admin/comments' });
		} catch (e: any) {
			toast.error(e.message || '删除失败');
		}
	}

	return (
		<div>
			<PageHeader
				title="评论详情"
				actions={
					<Button variant="outline" size="sm" onClick={() => navigate({ to: '/admin/comments' })}>
						← 返回
					</Button>
				}
			/>

			{loading ? (
				<Card className="mb-4 py-10">
					<Loading />
				</Card>
			) : comment ? (
				<>
					<Card className="mb-4 p-5">
						<Descriptions>
							<DescriptionsItem label="评论 ID" span={2}>
								<code className="bg-muted rounded px-1.5 py-0.5 select-all">{comment.id}</code>
							</DescriptionsItem>
							<DescriptionsItem label="作者">
								<span className="inline-flex items-center gap-1.5">
									{comment.author_name}
									<Button
										variant="link"
										size="xs"
										onClick={() => navigate({ to: '/admin/users/detail', search: { id: comment.author_id } })}
									>
										查看用户
									</Button>
								</span>
							</DescriptionsItem>
							<DescriptionsItem label="作者 ID">
								<code className="bg-muted rounded px-1.5 py-0.5 select-all">{comment.author_id}</code>
							</DescriptionsItem>
							<DescriptionsItem label="所属帖子">
								<span className="inline-flex items-center gap-1.5">
									{comment.post_title || comment.post_id}
									<Button
										variant="link"
										size="xs"
										onClick={() => navigate({ to: '/admin/posts/detail', search: { id: comment.post_id } })}
									>
										查看帖子
									</Button>
								</span>
							</DescriptionsItem>
							<DescriptionsItem label="帖子 ID">
								<code className="bg-muted rounded px-1.5 py-0.5 select-all">{comment.post_id}</code>
							</DescriptionsItem>
							<DescriptionsItem label="父评论 ID">
								{comment.parent_id ? (
									<code className="bg-muted rounded px-1.5 py-0.5 select-all">{comment.parent_id}</code>
								) : (
									'-'
								)}
							</DescriptionsItem>
							<DescriptionsItem label="类型">
								{comment.parent_id ? <Badge variant="outline">回复</Badge> : <Badge variant="primary">顶级评论</Badge>}
							</DescriptionsItem>
							<DescriptionsItem label="点赞数">{comment.like_count}</DescriptionsItem>
							<DescriptionsItem label="创建时间">{formatTime(comment.created_at)}</DescriptionsItem>
							<DescriptionsItem label="更新时间">{formatTime(comment.updated_at)}</DescriptionsItem>
						</Descriptions>

						<div className="border-border mt-5 border-t pt-4">
							<h2 className="mb-2 text-[13px] font-medium text-foreground">评论内容</h2>
							<p className="text-[13px] leading-relaxed whitespace-pre-wrap break-words text-foreground">
								{comment.body}
							</p>
						</div>
					</Card>

					<div>
						<Button
							variant="destructive"
							size="sm"
							onClick={() =>
								confirm({
									title: '删除该评论？',
									description: '评论会从帖子下移除，且不可恢复。',
									danger: true,
									confirmText: '删除评论',
									onConfirm: handleDelete,
								})
							}
						>
							删除评论
						</Button>
					</div>
				</>
			) : null}

			{confirmDialog}
		</div>
	);
}
