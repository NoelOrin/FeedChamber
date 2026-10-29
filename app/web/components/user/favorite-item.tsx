'use client';

import type { PostListItem } from '@orinforum/api-client';
import dayjs from 'dayjs';
import Link from 'next/link';

import { Button } from '~/components/ui/button';

interface FavoriteItemProps {
	post: PostListItem;
	onUnfavorite: () => void;
}

/** 收藏条目（旧 -modules/FavoriteItem.tsx）：`li.card` → `<li>` + Card 原语。 */
export function FavoriteItem({ post, onUnfavorite }: FavoriteItemProps) {
	const seoSlug = post.slug || 'post';

	return (
		<li className="group flex list-none items-start gap-3 px-4 py-3 transition-colors hover:bg-muted/50">
			<div className="flex flex-1 items-start justify-between gap-3">
				<div className="min-w-0 flex-1">
					<Link
						href={`/t/${encodeURIComponent(seoSlug)}/${encodeURIComponent(post.id)}`}
						className="block truncate text-[15px] font-medium text-foreground hover:underline"
					>
						{post.title}
					</Link>
					<div className="mt-1.5 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
						<span>{dayjs(post.created_at).format('YYYY-MM-DD')}</span>
						<span className="font-medium text-foreground/70">{post.author_name}</span>
						<span>{post.space_name}</span>
						<span>{post.comment_count} 评论</span>
					</div>
				</div>
				<Button
					size="xs"
					variant="ghost"
					onClick={onUnfavorite}
					className="shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100 hover:text-destructive focus-visible:opacity-100"
				>
					取消收藏
				</Button>
			</div>
		</li>
	);
}
