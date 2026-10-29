'use client';

import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { useMemo } from 'react';

import { ScrollPaginator } from '~/components/pagination/scroll-paginator';
import { PostCard } from '~/components/post/post-card';
import {
	type ListView,
	type PostFeedPage,
	type PostFeedParams,
	postListInfiniteOptions,
	postsOf,
	publicSettingsOptions,
	type SortBy,
	stableFeedKey,
	totalOf,
} from '~/components/post/queries';
import { PostFeedMoreSkeleton, PostListSkeleton } from '~/components/post/skeletons';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { EmptyState } from '~/components/ui/empty-state';
import { PageHeader } from '~/components/ui/page-header';
import { Segmented } from '~/components/ui/segmented';
import { Plus } from '~/lib/icons';
import { getArticlePageSize } from '~/lib/pagination';
import { cn } from '~/lib/utils';

/*
 * 时间线 / 单版 / 标签复用的列表容器。
 * 取数交给 useInfiniteQuery；排序 / 视图是受控展示件，URL 读写留在宿主（nuqs）；
 * 首屏数据由 RSC 预取后经 initialPage/initialPageKey 传入。
 */

const LATEST = 'latest';

const VIEW_TABS: { value: string; label: string }[] = [
	{ value: LATEST, label: '最新' },
	{ value: 'new', label: '新帖' },
	{ value: 'unread', label: '未读' },
	{ value: 'read', label: '已读' },
	{ value: 'posted', label: '我参与' },
];

const SORT_TABS: { value: string; label: string }[] = [
	{ value: LATEST, label: '最新' },
	{ value: 'hot', label: '最热' },
	{ value: 'most_commented', label: '评论最多' },
	{ value: 'active', label: '活跃' },
];

interface PostListPageProps {
	/** 列表筛选参数（不含 page/page_size）；宿主需保持引用稳定，否则每次渲染都会换 key */
	params: PostFeedParams;
	/** 当前排序语义值，null = 最新（URL 上不写字段） */
	sortBy: SortBy | null;
	onSortByChange: (value: SortBy | null) => void;
	/** 当前视图语义值；只有给了 onListViewChange 才渲染视图行（旧版仅在登录态出现） */
	listView?: ListView | null;
	onListViewChange?: (value: ListView | null) => void;
	/** 发帖入口：时间线 /timeline/create，单版 /spaces/{slug}/post */
	createLink: string;
	/** RSC 预取的第一页 */
	initialPage?: PostFeedPage | null;
	/** 与 RSC 那组参数对应的指纹，见 stableFeedKey；不匹配则不使用 initialPage */
	initialPageKey?: string | null;
	/** 页头已由宿主提供时置 true：只渲染筛选行与列表 */
	hideHeading?: boolean;
	className?: string;
}

export function PostListPage({
	params,
	sortBy,
	onSortByChange,
	listView,
	onListViewChange,
	createLink,
	initialPage,
	initialPageKey,
	hideHeading,
	className,
}: PostListPageProps) {
	const { data: settings } = useQuery(publicSettingsOptions());
	const pageSize = getArticlePageSize(settings);

	const options = useMemo(() => postListInfiniteOptions(params, pageSize), [params, pageSize]);
	/* 只有参数指纹与 RSC 预取那组一致时，才把首屏那页当 initialData 用（key 变了就该重新取）。 */
	const seedKey = stableFeedKey(params, pageSize);
	const initialData = useMemo(
		() =>
			initialPage && initialPageKey && initialPageKey === seedKey
				? { pages: [initialPage], pageParams: [1] }
				: undefined,
		[initialPage, initialPageKey, seedKey],
	);

	const { data, isPending, hasNextPage, isFetchingNextPage, fetchNextPage } = useInfiniteQuery({
		...options,
		...(initialData ? { initialData } : {}),
	});

	const posts = postsOf(data);
	const total = totalOf(data);
	const isEmpty = posts.length === 0;
	/* 尾部两件事同源：ScrollPaginator 的「没有更多了」与 END OF LIST 分隔条 */
	const showEndOfIndex = !isEmpty && hasNextPage === false;

	return (
		<div className={cn('flex h-full w-full flex-col pb-6', className)}>
			{hideHeading ? null : (
				<PageHeader
					title="时间线"
					meta={`${total} 条`}
					actions={
						<Button asChild size="sm" className="gap-1.5">
							<Link href={createLink}>
								<Plus size={14} aria-hidden="true" />
								发布主题
							</Link>
						</Button>
					}
				/>
			)}

			<div className="mb-4 flex flex-wrap items-center gap-x-4 gap-y-2">
				{onListViewChange ? (
					<div className="flex items-center gap-2">
						<span className="hidden text-xs text-muted-foreground sm:inline">视图</span>
						<Segmented
							label="视图筛选"
							value={listView ?? LATEST}
							options={VIEW_TABS}
							onValueChange={(next) => onListViewChange(next === LATEST ? null : (next as ListView))}
						/>
					</div>
				) : null}

				<div className="flex items-center gap-2">
					<span className="hidden text-xs text-muted-foreground sm:inline">排序</span>
					<Segmented
						label="排序方式"
						value={sortBy ?? LATEST}
						options={SORT_TABS}
						onValueChange={(next) => onSortByChange(next === LATEST ? null : (next as SortBy))}
					/>
				</div>
			</div>

			{posts.length > 0 ? (
				<Card variant="default" hover="static" className="gap-0 divide-y divide-border/60 overflow-hidden p-0">
					{posts.map((post) => (
						<PostCard key={post.id} post={post} />
					))}
				</Card>
			) : null}

			{isPending ? <PostListSkeleton /> : null}

			{!isPending && isEmpty ? (
				<EmptyState className="py-16" title="此视图暂无主题" description="换个筛选或发布第一篇讨论。">
					<Button asChild size="sm" className="gap-1.5">
						<Link href={createLink}>
							<Plus size={14} aria-hidden="true" /> 去发布
						</Link>
					</Button>
				</EmptyState>
			) : null}

			<ScrollPaginator
				className="mt-4"
				hasMore={hasNextPage === true}
				isLoading={isFetchingNextPage}
				onLoadMore={() => void fetchNextPage()}
				loadingComponent={<PostFeedMoreSkeleton />}
				/* 首屏 loading 交给骨架屏表达，这里不再叠一层「加载中...」；空结果也不尾部提示 */
				showHasMore={showEndOfIndex}
			/>
		</div>
	);
}
