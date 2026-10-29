'use client';

import type { PostDetail as PostDetailData } from '@orinforum/api-client';
import { PostStatus, postService, revisionService } from '@orinforum/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import Link from 'next/link';
import { useState } from 'react';

import { CommentList } from '~/components/content/comment-list';
import { Reply } from '~/components/content/reply';
import { ErrorFallback } from '~/components/error/error-fallback';
import { StartMotionButton } from '~/components/motion/motion-hall';
import { postDetailOptions } from '~/components/post/detail-queries';
import { LevelGateNotice } from '~/components/post/level-gate-notice';
import { PostContent } from '~/components/post/post-content';
import { PostHeader } from '~/components/post/post-header';
import { RelatedTopics } from '~/components/post/related-topics';
import { TopicHeader } from '~/components/post/topic-header';
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { Input } from '~/components/ui/input';
import { Skeleton } from '~/components/ui/skeleton';
import { Textarea } from '~/components/ui/textarea';
import { useAuthStore } from '~/lib/auth';
import { Bookmark, Clock, History, Lock, PenLine, Pin } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';
import { cn } from '~/lib/utils';

interface PostDetailProps {
	postId: string;
	pageSize: number;
}

function errText(error: unknown, fallback: string) {
	return error instanceof Error && error.message ? error.message : fallback;
}

/** 卡片动作按钮：默认 ghost，激活态用主色描边。 */
function ActionPill({ active, className, ...props }: React.ComponentProps<typeof Button> & { active?: boolean }) {
	return (
		<Button
			size="sm"
			variant={active ? 'outline' : 'ghost'}
			className={cn('gap-1.5 text-xs', active && 'border-brand-text/40 text-brand-text', className)}
			{...props}
		/>
	);
}

function DetailSkeleton() {
	return (
		<div className="flex w-full flex-col gap-4" aria-hidden="true">
			<Card variant="archive" hover="static" className="p-5 sm:p-6">
				<Skeleton className="h-3 w-40" />
				<Skeleton className="mt-3 h-8 w-3/4" />
				<div className="mt-4 flex items-center gap-3">
					<Skeleton className="size-6" />
					<Skeleton className="h-3 w-24" />
					<Skeleton className="h-3 w-16" />
					<Skeleton className="ml-auto h-3 w-20" />
				</div>
			</Card>
			<Card hover="static" className="p-5 sm:p-6">
				<Skeleton className="h-3 w-28" />
				<div className="mt-5 space-y-2">
					<Skeleton className="h-3 w-full" />
					<Skeleton className="h-3 w-5/6" />
					<Skeleton className="h-3 w-2/3" />
				</div>
			</Card>
			<Card hover="static" className="p-5 sm:p-6">
				<Skeleton className="h-3 w-32" />
				<div className="mt-5 space-y-4">
					{['floor-1', 'floor-2', 'floor-3'].map((id) => (
						<div key={id} className="flex gap-3.5">
							<Skeleton className="size-9 shrink-0" />
							<div className="min-w-0 flex-1 space-y-2">
								<Skeleton className="h-3 w-24" />
								<Skeleton className="h-3 w-full" />
								<Skeleton className="h-3 w-3/4" />
							</div>
						</div>
					))}
				</div>
			</Card>
		</div>
	);
}

/**
 * 帖子详情主体（旧 components/post/PostDetailView.tsx）。
 * 解剖逐条对齐设计文档 §11.4：面包眉标 → 大标题 → 元信息条 → 动作行 → 正文卡 → 楼层
 * → 回复区 → 回复编辑器 → 相关主题；卡片全部 hover="static"。
 */
export function PostDetail({ postId, pageSize }: PostDetailProps) {
	const queryClient = useQueryClient();
	const detail = useQuery(postDetailOptions(postId));
	const user = useAuthStore((state) => state.user);
	const loggedIn = useAuthStore((state) => state.status === 'authenticated' && state.user !== null);
	const canModerate = user?.role === 'admin' || user?.role === 'moderator';

	const [editing, setEditing] = useState(false);
	const [editTitle, setEditTitle] = useState('');
	const [editBody, setEditBody] = useState('');
	const [historyOpen, setHistoryOpen] = useState(false);
	const [revisions, setRevisions] = useState<Array<{ id: string; title: string; body: string; created_at: string }>>(
		[],
	);
	const [historyLoading, setHistoryLoading] = useState(false);

	const post = detail.data;
	const isClosed = post?.status === 'closed';
	const isLocked = Boolean(post?.locked);
	const canReply = Boolean(post) && !isClosed && !isLocked;
	const canEdit = canModerate || Boolean(post && user?.id === post.author_id);
	const unreadCount = post?.unread_content_count ?? 0;
	const canMarkRead = loggedIn && unreadCount > 0;

	const markRead = useMutation({
		mutationFn: () => postService.markRead(postId),
		onSuccess: () => {
			toast.success('已标记为已读');
			queryClient.invalidateQueries({ queryKey: qk.posts.detail(postId) });
			queryClient.invalidateQueries({ queryKey: qk.posts.all });
		},
		onError: (error) => toast.error(errText(error, '标记失败')),
	});

	const saveEdit = useMutation({
		mutationFn: () => postService.update(postId, { title: editTitle, content: editBody }),
		onSuccess: () => {
			setEditing(false);
			toast.success('帖子已更新');
			queryClient.invalidateQueries({ queryKey: qk.posts.detail(postId) });
			/* 列表卡片的标题/摘要同样是这份数据的副本 */
			queryClient.invalidateQueries({ queryKey: qk.posts.all });
		},
		onError: (error) => toast.error(errText(error, '更新失败')),
	});

	const restoreRevision = useMutation({
		mutationFn: (revisionId: string) => revisionService.restorePost(postId, revisionId),
		onSuccess: () => {
			toast.success('版本已恢复');
			queryClient.invalidateQueries({ queryKey: qk.posts.detail(postId) });
			queryClient.invalidateQueries({ queryKey: qk.contents.all });
		},
		onError: (error) => toast.error(errText(error, '版本恢复失败')),
	});

	const togglePin = useMutation({
		mutationFn: () => postService.togglePin(postId),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: qk.posts.all });
			toast.success('已更新置顶状态');
		},
		onError: (error) => toast.error(errText(error, '操作失败')),
	});

	const toggleLock = useMutation({
		mutationFn: () => postService.toggleLock(postId),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: qk.posts.all });
			toast.success('已更新锁定状态');
		},
		onError: (error) => toast.error(errText(error, '操作失败')),
	});

	const loadHistory = async () => {
		if (historyOpen) {
			setHistoryOpen(false);
			return;
		}
		setHistoryOpen(true);
		setHistoryLoading(true);
		try {
			const res = await revisionService.listPost(postId, { page: 1, page_size: 20 });
			setRevisions(res.data ?? []);
		} catch {
			setRevisions([]);
		} finally {
			setHistoryLoading(false);
		}
	};

	const startEdit = (current: PostDetailData) => {
		setEditTitle(current.title);
		setEditBody(current.body ?? '');
		setEditing(true);
	};

	if (detail.isPending) return <DetailSkeleton />;
	if (detail.isError || !post) {
		const message = detail.error instanceof Error ? detail.error.message : '';
		if (message.includes('需要等级') || message.includes('需要登录后访问')) {
			return <LevelGateNotice message={message} />;
		}
		return (
			<Card className="p-0">
				<ErrorFallback
					error={detail.error ?? new Error('帖子不存在')}
					variant="section"
					onRetry={() => detail.refetch()}
					hideHome
				/>
			</Card>
		);
	}

	return (
		<div className="flex w-full flex-col gap-3 pb-16">
			{/* 待审帖按契约 3 只对作者本人（与审核者）可见，所以这里不必再判身份：
			   能看到这条记录就说明它还没公开，给出状态说明即可。 */}
			{post.status === PostStatus.PENDING ? (
				<Alert variant="warning">
					<Clock aria-hidden="true" />
					<div>
						<AlertTitle>审核中</AlertTitle>
						<AlertDescription>该主题正在等待审核，通过后才会出现在列表、搜索与版面里。</AlertDescription>
					</div>
				</Alert>
			) : null}

			{/* ── 主题：标题 + 元信息 + 动作 + 正文 ── */}
			<Card className="gap-0 overflow-hidden p-0">
				<div className="p-5">
					<TopicHeader post={post} />

					<div className="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-border pt-3">
						<div className="flex flex-wrap items-center gap-1">
							{canMarkRead ? (
								<ActionPill loading={markRead.isPending} onClick={() => markRead.mutate()}>
									<Bookmark className="size-3.5" aria-hidden="true" /> 标记已读 ({unreadCount})
								</ActionPill>
							) : null}
							{canEdit ? (
								<ActionPill
									active={editing}
									onClick={() => {
										if (editing) setEditing(false);
										else startEdit(post);
									}}
								>
									<PenLine className="size-3.5" aria-hidden="true" /> 编辑
								</ActionPill>
							) : null}
							<ActionPill active={historyOpen} onClick={loadHistory}>
								<History className="size-3.5" aria-hidden="true" /> 历史版本
							</ActionPill>
							{loggedIn ? (
								<span className="inline-flex items-center px-2">
									<StartMotionButton targetType="post" targetId={postId} />
								</span>
							) : null}
							{canModerate ? (
								<>
									<ActionPill active={post.pinned} loading={togglePin.isPending} onClick={() => togglePin.mutate()}>
										<Pin className="size-3.5" aria-hidden="true" /> {post.pinned ? '取消置顶' : '置顶'}
									</ActionPill>
									<ActionPill active={isLocked} loading={toggleLock.isPending} onClick={() => toggleLock.mutate()}>
										<Lock className="size-3.5" aria-hidden="true" /> {isLocked ? '解除锁定' : '锁定'}
									</ActionPill>
								</>
							) : null}
						</div>
						<PostHeader postId={postId} />
					</div>

					{editing ? (
						<div className="mt-4 space-y-3 rounded-md border border-border bg-muted/40 p-3">
							<Input
								value={editTitle}
								placeholder="标题"
								className="bg-background font-medium"
								onChange={(event) => setEditTitle(event.target.value)}
							/>
							<Textarea
								value={editBody}
								rows={8}
								placeholder="正文 Markdown…"
								className="bg-background font-mono text-sm"
								onChange={(event) => setEditBody(event.target.value)}
							/>
							<div className="flex justify-end gap-2">
								<Button size="sm" variant="outline" onClick={() => setEditing(false)}>
									取消
								</Button>
								<Button
									size="sm"
									loading={saveEdit.isPending}
									disabled={!editTitle.trim() || !editBody.trim()}
									onClick={() => saveEdit.mutate()}
								>
									{saveEdit.isPending ? '保存中…' : '保存'}
								</Button>
							</div>
						</div>
					) : null}

					{historyOpen ? (
						<div className="mt-4 rounded-md border border-border bg-muted/40 p-3">
							{historyLoading ? (
								<div className="py-3 text-center text-xs text-muted-foreground">加载中…</div>
							) : revisions.length === 0 ? (
								<div className="py-3 text-center text-xs text-muted-foreground">暂无历史</div>
							) : (
								<div className="max-h-[280px] space-y-2 overflow-y-auto pr-1">
									{revisions.map((rev) => (
										<div
											key={rev.id}
											className="flex items-start justify-between gap-2 rounded-md border border-border bg-background p-3"
										>
											<div className="min-w-0">
												<div className="truncate text-[13px] font-medium">{rev.title}</div>
												<div className="text-[11px] text-muted-foreground tabular-nums">
													{new Date(rev.created_at).toLocaleString('zh-CN')}
												</div>
											</div>
											<Button
												size="xs"
												variant="outline"
												className="shrink-0"
												loading={restoreRevision.isPending}
												disabled={restoreRevision.isPending}
												onClick={() => restoreRevision.mutate(rev.id)}
											>
												恢复
											</Button>
										</div>
									))}
								</div>
							)}
						</div>
					) : null}
				</div>

				<div className="border-t border-border px-5 py-5">
					<PostContent post={post} />
				</div>
			</Card>

			{/* ── 回复 ── */}
			<Card className="gap-0 overflow-hidden p-0">
				<div className="flex items-center justify-between border-b border-border px-5 py-2.5">
					<h2 className="text-[15px] font-semibold text-foreground">
						回复
						<span className="ml-1.5 text-xs font-normal text-muted-foreground tabular-nums">{post.comment_count}</span>
					</h2>
					<span className="text-xs text-muted-foreground">按时间排序</span>
				</div>

				<div className="px-5 py-1">
					<CommentList postId={postId} pageSize={pageSize} />
				</div>

				{loggedIn ? (
					canReply ? (
						<div className="border-t border-border p-4">
							<Reply bare postId={postId} rootContentId={post.content_id ?? ''} />
						</div>
					) : (
						<div className="border-t border-border px-5 py-4 text-center text-xs text-muted-foreground">
							{isLocked ? '主题已锁定，无法回复' : '主题已关闭'}
						</div>
					)
				) : (
					<div className="flex flex-wrap items-center justify-between gap-3 border-t border-border px-5 py-4">
						<span className="text-[13px] text-muted-foreground">登录后即可参与讨论</span>
						<Button asChild size="sm">
							<Link href={`/login?redirect=${encodeURIComponent(`/timeline/post?id=${postId}`)}`}>去登录</Link>
						</Button>
					</div>
				)}
			</Card>

			<RelatedTopics post={post} />
		</div>
	);
}

export default PostDetail;
