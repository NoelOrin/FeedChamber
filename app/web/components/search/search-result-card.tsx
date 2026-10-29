import type { PostListItem } from '@orinforum/api-client';
import Link from 'next/link';

import { Avatar, AvatarFallback, AvatarImage } from '~/components/ui/avatar';
import { Badge } from '~/components/ui/badge';
import { Card } from '~/components/ui/card';
import { Lock, Pin } from '~/lib/icons';

function metaDate(value: string | undefined) {
	if (!value) return '';
	try {
		return new Date(value).toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' });
	} catch {
		return '';
	}
}

/** 检索结果卡（与列表行同一套信息层级）。 */
export function SearchResultCard({ post }: { post: PostListItem }) {
	const href = `/t/${post.slug || 'post'}/${post.id}`;
	return (
		<Card className="gap-0 p-4">
			<div className="flex items-start gap-3">
				<Link href={`/user/${post.author_id}`} aria-label={`${post.author_name} 的主页`} className="shrink-0">
					<Avatar className="size-8">
						{post.author_avatar ? <AvatarImage src={post.author_avatar} alt="" /> : null}
						<AvatarFallback className="text-[11px]">
							{(post.author_name || '?').slice(0, 1).toUpperCase()}
						</AvatarFallback>
					</Avatar>
				</Link>

				<div className="min-w-0 flex-1">
					<div className="flex min-w-0 items-start gap-1.5">
						<Link
							href={href}
							className="line-clamp-2 min-w-0 flex-1 text-[15px] leading-snug font-medium text-foreground hover:underline"
						>
							{post.title}
						</Link>
						{post.pinned ? (
							<Badge variant="outline" className="shrink-0 gap-1">
								<Pin className="size-2.5" aria-hidden="true" /> 置顶
							</Badge>
						) : null}
						{post.locked ? (
							<Badge variant="outline" className="shrink-0 gap-1">
								<Lock className="size-2.5" aria-hidden="true" /> 锁定
							</Badge>
						) : null}
					</div>

					<div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
						<span className="text-foreground">{post.space_name || '未分类'}</span>
						<span aria-hidden="true">·</span>
						<Link href={`/user/${post.author_id}`} className="hover:text-foreground">
							{post.author_name}
						</Link>
						<span aria-hidden="true">·</span>
						<span className="tabular-nums">{post.comment_count} 回复</span>
						{post.created_at ? (
							<>
								<span aria-hidden="true">·</span>
								<span className="tabular-nums">{metaDate(post.created_at)}</span>
							</>
						) : null}
					</div>
				</div>
			</div>
		</Card>
	);
}
