'use client';

import type { PostDetail } from '@orinforum/api-client';
import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';

import { postSuggestionsOptions } from '~/components/post/detail-queries';
import { Eye } from '~/lib/icons';
import { cn } from '~/lib/utils';

interface RelatedTopicsProps {
	post: PostDetail;
	className?: string;
}

/** 相关主题：轻量文字列表，右侧浏览量。 */
export function RelatedTopics({ post, className }: RelatedTopicsProps) {
	const suggestions = useQuery(postSuggestionsOptions(post.id));
	const items = (suggestions.data ?? []).filter((item) => item.id !== post.id).slice(0, 5);

	if (items.length === 0) return null;

	return (
		<section className={cn('mt-2', className)}>
			<h2 className="mb-2 text-xs font-medium text-muted-foreground">相关主题</h2>
			<ul className="divide-y divide-border/60 overflow-hidden rounded-lg border border-border bg-card">
				{items.map((item) => (
					<li key={item.id} className="min-w-0">
						<Link
							href={`/timeline/post?id=${item.id}`}
							className="flex items-center justify-between gap-3 px-4 py-2.5 text-[13px] transition-colors hover:bg-muted/50"
						>
							<span className="min-w-0 truncate text-foreground">{item.title}</span>
							<span className="inline-flex shrink-0 items-center gap-1 text-xs text-muted-foreground tabular-nums">
								<Eye className="size-3.5" aria-hidden="true" />
								{item.view_count}
							</span>
						</Link>
					</li>
				))}
			</ul>
		</section>
	);
}

export default RelatedTopics;
