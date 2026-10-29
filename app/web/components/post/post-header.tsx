'use client';

import { favoriteService, watchService } from '@feedchamber/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';

import { Button } from '~/components/ui/button';
import { useAuthStore } from '~/lib/auth';
import { Bell, Bookmark } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

interface PostHeaderProps {
	postId: string;
	className?: string;
}

function errText(error: unknown, fallback: string) {
	return error instanceof Error && error.message ? error.message : fallback;
}

/**
 * 动作行右侧的关注 / 收藏开关（旧 -modules/PostHeader.tsx）。
 * 旧版用 onMount + 两个 signal 拉状态；这里收藏走 qk.favorite.status 缓存，
 * 订阅状态后端没有对应 query key（qk 里不存在 watch），保持旧写法用本地 state 兜住。
 */
export function PostHeader({ postId, className }: PostHeaderProps) {
	const queryClient = useQueryClient();
	const loggedIn = useAuthStore((state) => state.status === 'authenticated' && state.user !== null);
	const [watching, setWatching] = useState(false);

	const favoriteStatus = useQuery({
		queryKey: qk.favorite.status(postId),
		queryFn: () => favoriteService.status(postId),
		enabled: loggedIn && Boolean(postId),
	});

	useEffect(() => {
		if (!loggedIn || !postId) return;
		let alive = true;
		watchService
			.status({ target_type: 'post', target_id: postId })
			.then((res) => {
				if (alive) setWatching(Boolean(res?.watching));
			})
			.catch(() => {
				// 未登录或后端拒绝时保持未订阅外观，与旧版一致
			});
		return () => {
			alive = false;
		};
	}, [loggedIn, postId]);

	const toggleFavorite = useMutation({
		mutationFn: () => favoriteService.toggle(postId),
		onSuccess: (res) => {
			queryClient.setQueryData(qk.favorite.status(postId), { favorited: Boolean(res?.favorited) });
			queryClient.invalidateQueries({ queryKey: qk.favorite.all });
			queryClient.invalidateQueries({ queryKey: qk.posts.detail(postId) });
			toast.success(res?.favorited ? '已加入收藏' : '已取消收藏');
		},
		onError: (error) => toast.error(errText(error, '操作失败')),
	});

	const toggleWatch = useMutation({
		mutationFn: () =>
			watching
				? watchService.delete({ target_type: 'post', target_id: postId })
				: watchService.set({ target_type: 'post', target_id: postId, level: 'watching' }),
		onSuccess: () => {
			setWatching((v) => !v);
			queryClient.invalidateQueries({ queryKey: qk.posts.list({}) });
			queryClient.invalidateQueries({ queryKey: qk.posts.detail(postId) });
			toast.success(watching ? '已取消关注话题' : '已关注话题');
		},
		onError: (error) => toast.error(errText(error, '操作失败')),
	});

	const favorited = Boolean(favoriteStatus.data?.favorited);

	return (
		<div className={className}>
			<div className="flex flex-wrap items-center justify-end gap-2">
				<Button
					variant="ghost"
					size="sm"
					className="gap-1.5"
					disabled={toggleWatch.isPending}
					onClick={() => toggleWatch.mutate()}
				>
					<Bell className="size-3.5" aria-hidden="true" />
					{watching ? '已关注话题' : '关注话题'}
				</Button>
				<Button
					variant="ghost"
					size="sm"
					className="gap-1.5"
					disabled={toggleFavorite.isPending}
					onClick={() => toggleFavorite.mutate()}
				>
					<Bookmark className={favorited ? 'size-3.5 fill-current' : 'size-3.5'} aria-hidden="true" />
					{favorited ? '已收藏' : '收藏'}
				</Button>
			</div>
		</div>
	);
}

export default PostHeader;
