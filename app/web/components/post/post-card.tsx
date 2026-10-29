import type { PostListItem } from '@orinforum/api-client';
import Link from 'next/link';

import { Avatar, AvatarFallback, AvatarImage } from '~/components/ui/avatar';
import { Badge } from '~/components/ui/badge';
import { Eye, Lock, MessageCircle, Pin } from '~/lib/icons';
import { cn } from '~/lib/utils';

/*
 * 列表行：头像 + 标题 + 一行元信息 + 右侧计数。
 * 行本身不带卡片外壳——外壳由列表容器统一画（一张 Card + divide-y）。
 * 详情 URL 是 query 参数形态 /timeline/post?id=（e2e 锁死），不是路径参数。
 */

interface PostCardProps {
	post: PostListItem;
	className?: string;
}

function postHref(id: string) {
	return `/timeline/post?id=${encodeURIComponent(id)}`;
}

function metaDate(value: string | undefined) {
	if (!value) return '';
	try {
		return new Date(value).toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' });
	} catch {
		return '';
	}
}

function authorInitial(name: string | undefined) {
	return (name || '?').slice(0, 1).toUpperCase();
}

export function PostCard({ post, className }: PostCardProps) {
	const date = metaDate(post.created_at);

	return (
		<article
			className={cn('group relative flex items-start gap-3 px-4 py-3.5 transition-colors hover:bg-muted/50', className)}
		>
			<Link
				href={`/user/${encodeURIComponent(post.author_id)}`}
				className="mt-0.5 shrink-0 rounded-full focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
				aria-label={`${post.author_name} 的主页`}
				tabIndex={-1}
			>
				<Avatar className="size-8">
					{post.author_avatar ? <AvatarImage src={post.author_avatar} alt="" /> : null}
					<AvatarFallback>{authorInitial(post.author_name)}</AvatarFallback>
				</Avatar>
			</Link>

			<div className="min-w-0 flex-1">
				<div className="flex min-w-0 items-start gap-1.5">
					<Link
						href={postHref(post.id)}
						className="line-clamp-2 min-w-0 flex-1 text-[15px] leading-snug font-medium text-foreground group-hover:underline"
					>
						{post.title}
					</Link>
					{post.pinned ? (
						<Badge variant="outline" className="mt-0.5 hidden shrink-0 gap-1 sm:inline-flex">
							<Pin className="size-2.5" aria-hidden="true" /> 置顶
						</Badge>
					) : null}
					{post.locked ? (
						<Badge variant="outline" title="已锁定" className="mt-0.5 hidden shrink-0 gap-1 sm:inline-flex">
							<Lock className="size-2.5" aria-hidden="true" /> 锁定
						</Badge>
					) : null}
				</div>

				<div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
					<span className="text-foreground">{post.space_name || '未分类'}</span>
					<span aria-hidden="true" className="text-border">
						·
					</span>
					<Link href={`/user/${encodeURIComponent(post.author_id)}`} className="hover:text-foreground">
						{post.author_name}
					</Link>
					{date ? (
						<>
							<span aria-hidden="true" className="text-border">
								·
							</span>
							<span className="tabular-nums">{date}</span>
						</>
					) : null}
					{post.has_new ? (
						<span
							role="img"
							title="有新回复"
							aria-label="有新回复"
							className="ml-0.5 inline-block size-1.5 rounded-full bg-primary align-middle"
						/>
					) : null}
				</div>
			</div>

			<div className="hidden shrink-0 items-center gap-3 pt-1 text-xs text-muted-foreground tabular-nums sm:flex">
				<span className="inline-flex items-center gap-1">
					<Eye className="size-3.5" aria-hidden="true" />
					{post.view_count ?? 0}
				</span>
				<span className="inline-flex items-center gap-1">
					<MessageCircle className="size-3.5" aria-hidden="true" />
					{post.comment_count ?? 0}
				</span>
			</div>
		</article>
	);
}
