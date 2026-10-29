import type { PostListItem } from '@orinforum/api-client';

import { PostList } from '~/components/user/post-list';
import { cn } from '~/lib/utils';

interface MyPostsProps {
	posts: PostListItem[];
	currentPage: number;
	totalPages: number;
	loading: boolean;
	onPageChange: (page: number) => void;
	className?: string;
}

export function MyPosts({ posts, currentPage, totalPages, loading, onPageChange, className }: MyPostsProps) {
	return (
		<section className={cn('flex flex-col gap-2', className)}>
			<h2 className="text-xs font-medium text-muted-foreground">发布的主题</h2>
			<PostList
				posts={posts}
				currentPage={currentPage}
				totalPages={totalPages}
				onPageChange={onPageChange}
				loading={loading}
			/>
		</section>
	);
}
