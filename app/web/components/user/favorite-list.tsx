import type { PostListItem } from '@feedchamber/api-client';

import { Pager } from '~/components/pagination/pager';
import { EmptyState } from '~/components/ui/empty-state';
import { FavoriteItem } from '~/components/user/favorite-item';

interface FavoriteListProps {
	posts: PostListItem[];
	/** 收藏总数：仅用于外层页头展示，列表本身不读 */
	total?: number;
	currentPage: number;
	totalPages: number;
	onPageChange: (page: number) => void;
	onUnfavorite: (postId: string) => void;
}

/** 收藏列表（旧 -modules/FavoriteList.tsx）。 */
export function FavoriteList({ posts, currentPage, totalPages, onPageChange, onUnfavorite }: FavoriteListProps) {
	if (posts.length === 0) {
		return <EmptyState title="暂无收藏" description="收藏的帖子会显示在这里" />;
	}

	return (
		<>
			<ul className="m-0 list-none divide-y divide-border/60 overflow-hidden rounded-lg border border-border bg-card p-0">
				{posts.map((post) => (
					<FavoriteItem key={post.id} post={post} onUnfavorite={() => onUnfavorite(post.id)} />
				))}
			</ul>
			<Pager currentPage={currentPage} totalPages={totalPages} onPageChange={onPageChange} />
		</>
	);
}
