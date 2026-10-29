'use client';

import type { ContentListItem } from '@feedchamber/api-client';
import { contentService, likeService, revisionService } from '@feedchamber/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';
import type { ReactNode } from 'react';
import { useEffect, useRef, useState } from 'react';

import { MarkdownRenderer } from '~/components/content/markdown-renderer';
import { Reply } from '~/components/content/reply';
import { Avatar, AvatarFallback, AvatarImage } from '~/components/ui/avatar';
import { Button } from '~/components/ui/button';
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from '~/components/ui/dialog';
import { Textarea } from '~/components/ui/textarea';
import { useAuthStore } from '~/lib/auth';
import { History, MessageCircle, PenLine, ThumbsUp, Trash2 } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';
import { cn } from '~/lib/utils';

interface ContentItemProps {
	content: ContentListItem;
	postId?: string;
	/** 嵌套楼层：由 CommentList 组树后传入，渲染在本楼层下方（左侧竖线缩进） */
	children?: ReactNode;
	className?: string;
	/** root = 帖子正文：不显示作者行与楼层动作，只保留点赞 */
	mode?: 'floor' | 'root';
}

/** 旧 revision 列表的行形状（本切片只用 body / created_at）。 */
interface RevisionRow {
	id: string;
	body: string;
	created_at: string;
}

function errText(error: unknown, fallback: string) {
	return error instanceof Error && error.message ? error.message : fallback;
}

export function ContentItem({ content, postId, children, className, mode = 'floor' }: ContentItemProps) {
	const queryClient = useQueryClient();
	const user = useAuthStore((state) => state.user);
	const canModerate = user?.role === 'admin' || user?.role === 'moderator';
	const isRoot = mode === 'root';

	const [body, setBody] = useState(content.body);
	const [editing, setEditing] = useState(false);
	const [editBody, setEditBody] = useState(content.body);
	const [replying, setReplying] = useState(false);
	const [liked, setLiked] = useState(false);
	const [likeCount, setLikeCount] = useState(content.like_count ?? 0);
	const [historyOpen, setHistoryOpen] = useState(false);
	const [revisions, setRevisions] = useState<RevisionRow[]>([]);
	const [confirmDelete, setConfirmDelete] = useState(false);
	// 乐观更新的回滚快照
	const snapshot = useRef({ liked: false, count: 0 });

	const canEdit = Boolean(postId && user?.id === content.author_id) || canModerate;

	/* 登录前不发：后端对匿名请求一律拒绝，N 个楼层就是 N 个必然失败的往返。 */
	const loggedIn = user !== null;
	useEffect(() => {
		if (!loggedIn) return;
		let alive = true;
		likeService
			.status(content.id)
			.then((res) => {
				if (!alive || !res) return;
				setLiked(Boolean(res.liked));
				if (typeof res.count === 'number') setLikeCount(res.count);
			})
			.catch(() => {
				// 保留列表里的静态计数
			});
		return () => {
			alive = false;
		};
	}, [content.id, loggedIn]);

	// 分页/刷新带回新计数时以服务端为准，清掉乐观值
	useEffect(() => setLikeCount(content.like_count ?? 0), [content.like_count]);

	const like = useMutation({
		mutationFn: () => likeService.toggle(content.id),
		onSuccess: (res) => {
			if (res && typeof res.liked === 'boolean') setLiked(res.liked);
		},
		onError: (error) => {
			setLiked(snapshot.current.liked);
			setLikeCount(snapshot.current.count);
			toast.error(errText(error, '操作失败'));
		},
	});

	/** 乐观更新 + 失败回滚。 */
	const toggleLike = () => {
		snapshot.current = { liked, count: likeCount };
		const next = !liked;
		setLiked(next);
		setLikeCount((count) => count + (next ? 1 : -1));
		like.mutate();
	};

	const update = useMutation({
		mutationFn: (text: string) => contentService.update(postId as string, content.id, { body: text }),
		onSuccess: (updated) => {
			setBody(updated.body);
			setEditing(false);
			toast.success('评论已更新');
			queryClient.invalidateQueries({ queryKey: qk.contents.all });
		},
		onError: (error) => toast.error(errText(error, '更新失败')),
	});

	const remove = useMutation({
		mutationFn: () => contentService.delete(postId as string, content.id),
		onSuccess: () => {
			setConfirmDelete(false);
			toast.success('评论已删除');
			queryClient.invalidateQueries({ queryKey: qk.contents.all });
			if (postId) queryClient.invalidateQueries({ queryKey: qk.posts.detail(postId) });
		},
		onError: (error) => toast.error(errText(error, '删除失败')),
	});

	const restore = useMutation({
		mutationFn: (revisionId: string) => revisionService.restoreComment(content.id, revisionId),
		onSuccess: () => {
			toast.success('评论版本已恢复');
			queryClient.invalidateQueries({ queryKey: qk.contents.all });
			if (postId) queryClient.invalidateQueries({ queryKey: qk.posts.detail(postId) });
		},
		onError: (error) => toast.error(errText(error, '版本恢复失败')),
	});

	const loadHistory = async () => {
		if (historyOpen) {
			setHistoryOpen(false);
			return;
		}
		setHistoryOpen(true);
		try {
			const res = await revisionService.listComment(content.id, { page: 1, page_size: 20 });
			setRevisions(res.data ?? []);
		} catch {
			setRevisions([]);
		}
	};

	return (
		<div id={`comment-${content.id}`} className={cn('group flex gap-3', !isRoot && 'py-4', className)}>
			{!isRoot ? (
				<Avatar className="mt-0.5 size-8 shrink-0">
					{content.author_avatar ? <AvatarImage src={content.author_avatar} alt="" /> : null}
					<AvatarFallback className="text-[11px]">
						{content.author_name?.charAt(0)?.toUpperCase() || '?'}
					</AvatarFallback>
				</Avatar>
			) : null}

			<div className="min-w-0 flex-1">
				{!isRoot ? (
					<div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
						<span className="text-[13px] font-medium text-foreground">{content.author_name}</span>
						<span aria-hidden="true" className="text-border">
							·
						</span>
						<span className="tabular-nums">{dayjs(content.created_at).format('YYYY-MM-DD HH:mm')}</span>
					</div>
				) : null}

				<div className={cn(isRoot ? '' : 'mt-1.5')}>
					{editing ? (
						<div className="space-y-3 rounded-md border border-border bg-muted/40 p-3">
							<Textarea
								value={editBody}
								rows={5}
								placeholder="正文 Markdown…"
								className="w-full bg-background font-mono text-sm"
								onChange={(event) => setEditBody(event.target.value)}
							/>
							<div className="flex justify-end gap-2">
								<Button
									size="sm"
									variant="outline"
									onClick={() => {
										setEditing(false);
										setEditBody(body);
									}}
								>
									取消
								</Button>
								<Button
									size="sm"
									loading={update.isPending}
									disabled={!editBody.trim()}
									onClick={() => postId && update.mutate(editBody.trim())}
								>
									{update.isPending ? '保存中…' : '保存'}
								</Button>
							</div>
						</div>
					) : (
						<MarkdownRenderer mdText={body} />
					)}
				</div>

				<div className={cn('flex flex-wrap items-center gap-0.5', isRoot ? 'mt-4' : 'mt-1')}>
					<Button
						variant="ghost"
						size="xs"
						className="gap-1 text-muted-foreground"
						disabled={like.isPending}
						onClick={toggleLike}
					>
						<ThumbsUp className={cn('size-3.5', liked && 'fill-current text-brand-text')} aria-hidden="true" />
						{liked ? '已赞' : '点赞'}
						{likeCount > 0 ? <span className="tabular-nums">{likeCount}</span> : null}
					</Button>
					{!isRoot && postId ? (
						<Button
							variant="ghost"
							size="xs"
							className="gap-1 text-muted-foreground"
							onClick={() => setReplying((v) => !v)}
						>
							<MessageCircle className="size-3.5" aria-hidden="true" />
							回复
						</Button>
					) : null}
					{!isRoot && canEdit ? (
						<Button
							variant="ghost"
							size="xs"
							className="gap-1 text-muted-foreground"
							onClick={() => {
								setEditing((v) => !v);
								setEditBody(body);
							}}
						>
							<PenLine className="size-3.5" aria-hidden="true" />
							编辑
						</Button>
					) : null}
					{!isRoot ? (
						<Button variant="ghost" size="xs" className="gap-1 text-muted-foreground" onClick={loadHistory}>
							<History className="size-3.5" aria-hidden="true" />
							版本
						</Button>
					) : null}
					{!isRoot && canEdit && postId ? (
						<Button
							variant="ghost"
							size="xs"
							className="gap-1 text-muted-foreground hover:text-destructive"
							onClick={() => setConfirmDelete(true)}
						>
							<Trash2 className="size-3.5" aria-hidden="true" />
							删除
						</Button>
					) : null}
				</div>

				{replying && postId ? (
					<div className="mt-3">
						<Reply
							postId={postId}
							rootContentId={content.root_id || content.id}
							parentId={content.id}
							compact
							onDone={() => setReplying(false)}
							onSuccess={() => queryClient.invalidateQueries({ queryKey: qk.contents.list(postId) })}
						/>
					</div>
				) : null}

				{historyOpen ? (
					<div className="mt-3 space-y-2 rounded-md border border-border bg-muted/40 p-3">
						{revisions.length === 0 ? (
							<div className="py-2 text-center text-xs text-muted-foreground">暂无历史版本</div>
						) : (
							revisions.map((rev) => (
								<div
									key={rev.id}
									className="flex items-start justify-between gap-3 rounded-md border border-border bg-background p-3"
								>
									<div className="min-w-0 flex-1">
										<div className="line-clamp-2 text-xs leading-relaxed text-muted-foreground">{rev.body}</div>
										<div className="mt-1 text-[11px] text-muted-foreground tabular-nums">
											{new Date(rev.created_at).toLocaleString('zh-CN')}
										</div>
									</div>
									<Button
										size="xs"
										variant="outline"
										className="shrink-0"
										loading={restore.isPending}
										disabled={restore.isPending}
										onClick={() => restore.mutate(rev.id)}
									>
										恢复
									</Button>
								</div>
							))
						)}
					</div>
				) : null}

				{children ? <div className="mt-2 space-y-1 border-l border-border pl-3 sm:pl-4">{children}</div> : null}
			</div>

			<Dialog open={confirmDelete} onOpenChange={setConfirmDelete}>
				<DialogContent className="max-w-md">
					<DialogHeader>
						<DialogTitle>删除这条回复？</DialogTitle>
						<DialogDescription>删除后该楼层及其子回复将被移除，此操作不可撤销。</DialogDescription>
					</DialogHeader>
					<DialogFooter>
						<Button variant="outline" onClick={() => setConfirmDelete(false)}>
							取消
						</Button>
						<Button variant="destructive" loading={remove.isPending} onClick={() => remove.mutate()}>
							删除
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</div>
	);
}

export default ContentItem;
