'use client';

import { favoriteService } from '@feedchamber/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { parseAsInteger, useQueryState } from 'nuqs';
import { useState } from 'react';
import { PageHeader } from '~/components/ui/page-header';

import { FavoriteList } from '~/components/user/favorite-list';
import { favoriteIdsOptions, favoritePostsOptions, totalPages } from '~/components/user/queries';
import { SkeletonList } from '~/components/user/skeletons';
import { DEFAULT_PAGE_SIZE } from '~/lib/pagination';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

/** 我的收藏（旧 pages/(app)/user/favorites.tsx）：id 列表 → 批量换帖子 的两跳照旧。 */
export function FavoritesPage() {
	const queryClient = useQueryClient();
	const [page, setPage] = useQueryState('page', parseAsInteger.withDefault(1));
	/* 旧版是本地 signal 乐观移除；缓存驱动后用一个 pendingRemoval 表达同一件事 */
	const [pendingRemoval, setPendingRemoval] = useState<string | null>(null);

	const idsQuery = useQuery(favoriteIdsOptions(page));
	const ids = idsQuery.data?.data ?? [];
	const total = idsQuery.data?.total ?? 0;
	const postsQuery = useQuery(favoritePostsOptions(ids));

	const posts = (postsQuery.data ?? []).filter((post) => post.id !== pendingRemoval);

	const toggle = useMutation({
		mutationFn: (postId: string) => favoriteService.toggle(postId),
		onMutate: (postId) => setPendingRemoval(postId),
		onSuccess: (res) => {
			setPendingRemoval(null);
			if (res.favorited) {
				toast.info('该帖子已重新收藏');
			} else {
				toast.success('已取消收藏');
				void queryClient.invalidateQueries({ queryKey: qk.favorite.all });
				void queryClient.invalidateQueries({ queryKey: qk.posts.all });
			}
		},
		onError: () => {
			setPendingRemoval(null);
			toast.error('操作失败');
		},
	});

	const isPending = idsQuery.isPending || (ids.length > 0 && postsQuery.isPending);

	return (
		<div className="flex w-full flex-col gap-3 pb-10">
			<PageHeader title="我的收藏" meta={`${total} 条`} />

			{isPending ? (
				<ul className="m-0 flex list-none flex-col gap-2 p-0">
					<SkeletonList count={5} type="favorite-item" />
				</ul>
			) : (
				<FavoriteList
					posts={posts}
					total={total}
					currentPage={page}
					totalPages={totalPages(total, DEFAULT_PAGE_SIZE)}
					onPageChange={(next) => void setPage(next)}
					onUnfavorite={(postId) => toggle.mutate(postId)}
				/>
			)}
		</div>
	);
}
