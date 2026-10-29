import type { PostListItem } from '@orinforum/api-client';

import { Pager } from '~/components/pagination/pager';
import { Card } from '~/components/ui/card';
import { EmptyState } from '~/components/ui/empty-state';
import { PostItem } from '~/components/user/post-item';
import { SkeletonList } from '~/components/user/skeletons';

interface PostListProps {
	posts: PostListItem[];
	currentPage: number;
	totalPages: number;
	onPageChange: (page: number) => void;
	loading?: boolean;
	className?: string;
}

export function PostList({ posts, currentPage, totalPages, onPageChange, loading, className }: PostListProps) {
	if (loading) {
		return (
			<Card className="gap-0 overflow-hidden p-0">
				<ul className="m-0 list-none divide-y divide-border/60 p-0">
					<SkeletonList count={5} type="item" />
				</ul>
			</Card>
		);
	}

	if (posts.length === 0) {
		return <EmptyState title="暂无帖子" description="这里会显示该用户发布的主题。" />;
	}

	return (
		<div className={className}>
			<Card className="gap-0 overflow-hidden p-0">
				<ul className="m-0 list-none divide-y divide-border/60 p-0">
					{posts.map((post) => (
						<PostItem key={post.id} post={post} />
					))}
				</ul>
			</Card>
			<Pager currentPage={currentPage} totalPages={totalPages} onPageChange={onPageChange} />
		</div>
	);
}
