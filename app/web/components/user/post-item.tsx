import type { PostListItem } from '@feedchamber/api-client';
import dayjs from 'dayjs';
import Link from 'next/link';

import { Badge } from '~/components/ui/badge';

interface PostItemProps {
	post: PostListItem;
}

const STATUS_LABELS: Record<string, string> = {
	published: '已发布',
	approved: '已发布',
	draft: '草稿',
	pending: '待审',
	locked: '已锁定',
	closed: '已关闭',
	deleted: '已删除',
	rejected: '已驳回',
};

/** 列表行（由 PostList 的 Card + divide-y 承载外壳）。 */
export function PostItem({ post }: PostItemProps) {
	const seoSlug = post.slug || 'post';
	const status = post.status;
	const published = status === 'published' || status === 'approved';

	return (
		<li className="list-none">
			<div className="flex items-start gap-3 px-4 py-3.5 transition-colors hover:bg-muted/50">
				<div className="min-w-0 flex-1">
					<div className="flex items-start gap-2">
						<Link
							href={`/t/${encodeURIComponent(seoSlug)}/${encodeURIComponent(post.id)}`}
							className="line-clamp-2 min-w-0 flex-1 text-[15px] leading-snug font-medium text-foreground hover:underline"
						>
							{post.title}
						</Link>
						{post.has_new ? (
							<span
								role="img"
								title="有新回复"
								aria-label="有新回复"
								className="mt-1.5 size-1.5 shrink-0 rounded-full bg-primary"
							/>
						) : null}
						{!published ? (
							<Badge variant={status === 'deleted' || status === 'rejected' ? 'destructive' : 'outline'}>
								{STATUS_LABELS[status] || status}
							</Badge>
						) : null}
					</div>

					<div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
						<span className="text-foreground">{post.space_name || '未分类'}</span>
						<span aria-hidden="true" className="text-border">
							·
						</span>
						<span className="tabular-nums">{dayjs(post.created_at).format('YYYY-MM-DD')}</span>
						<span aria-hidden="true" className="text-border">
							·
						</span>
						<span className="tabular-nums">{post.comment_count} 回复</span>
						<span aria-hidden="true" className="text-border">
							·
						</span>
						<span className="tabular-nums">{post.view_count} 浏览</span>
					</div>
				</div>
			</div>
		</li>
	);
}
