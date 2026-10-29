'use client';

import type { PostDetail } from '@orinforum/api-client';
import dayjs from 'dayjs';
import Link from 'next/link';

import { Avatar, AvatarFallback, AvatarImage } from '~/components/ui/avatar';
import { Badge } from '~/components/ui/badge';
import { Clock, Eye, Lock, MessageCircle, Pin } from '~/lib/icons';

interface TopicHeaderProps {
	post: PostDetail;
	className?: string;
}

/** 详情页头部：版面 → 标题 → 元信息行。 */
export function TopicHeader({ post, className }: TopicHeaderProps) {
	const closed = post.status === 'closed';

	return (
		<div className={className}>
			<div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
				<Link href="/spaces" className="transition-colors hover:text-foreground">
					{post.space_name || '未分类'}
				</Link>
				{post.pinned ? (
					<Badge variant="outline" className="gap-1">
						<Pin className="size-2.5" aria-hidden="true" /> 置顶
					</Badge>
				) : null}
				{post.locked ? (
					<Badge variant="outline" className="gap-1">
						<Lock className="size-2.5" aria-hidden="true" /> 锁定
					</Badge>
				) : null}
				{closed ? <Badge variant="warning">已关闭</Badge> : null}
			</div>

			<h1 className="mt-2 text-[26px] leading-tight font-semibold tracking-tight text-balance text-foreground">
				{post.title}
			</h1>

			<div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1.5 text-xs text-muted-foreground">
				{post.author_id ? (
					<Link
						href={`/user/${post.author_id}`}
						className="inline-flex items-center gap-1.5 font-medium text-foreground transition-colors hover:underline"
					>
						<Avatar className="size-5">
							{post.author_avatar ? <AvatarImage src={post.author_avatar} alt="" /> : null}
							<AvatarFallback className="text-[11px]">
								{(post.author_name || '?').slice(0, 1).toUpperCase()}
							</AvatarFallback>
						</Avatar>
						{post.author_name}
					</Link>
				) : null}
				<span className="inline-flex items-center gap-1 tabular-nums">
					<Clock className="size-3.5" aria-hidden="true" />
					{dayjs(post.created_at).format('YYYY-MM-DD HH:mm')}
				</span>
				<span className="inline-flex items-center gap-1 tabular-nums">
					<Eye className="size-3.5" aria-hidden="true" />
					{post.view_count}
				</span>
				<span className="inline-flex items-center gap-1 tabular-nums">
					<MessageCircle className="size-3.5" aria-hidden="true" />
					{post.comment_count}
				</span>
			</div>
		</div>
	);
}

export default TopicHeader;
