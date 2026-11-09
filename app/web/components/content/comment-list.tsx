'use client';

import type { ContentListItem } from '@feedchamber/api-client';
import { postService } from '@feedchamber/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type * as React from 'react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { ContentItem } from '~/components/content/content-item';
import { ScrollPaginator } from '~/components/pagination/scroll-paginator';
import { postContentsOptions } from '~/components/post/detail-queries';
import { EmptyState } from '~/components/ui/empty-state';
import { Skeleton } from '~/components/ui/skeleton';

interface CommentListProps {
	postId: string;
	pageSize: number;
	/** 服务端回传的续读游标（上次读到的评论 id）：用于进入时定位 + 画「上次读到这里」。 */
	resumeFrom?: string;
	/**
	 * 最近一次观测到的阅读位置。用 ref 而不是 state：滚动会高频改它，
	 * 而唯一需要读它的时刻是用户点「标记已读」——取一次当前值就够，
	 * 不该为此把整篇详情重渲染一遍。
	 */
	cursorRef?: { current: string };
}

/** 楼层最多铺到第 4 层，再深的回复并入第 4 层，避免窄列里无限嵌套。 */
const MAX_DEPTH = 4;

/** 上报阅读位置的防抖：滚完一轮讨论不该打几十次接口。 */
const CURSOR_REPORT_DEBOUNCE_MS = 1200;

/** ContentItem 锚点 id 的前缀（`id={'comment-'+content.id}`），两处必须同步改。 */
const FLOOR_ID_PREFIX = 'comment-';

function orderAsc(a?: ContentListItem, b?: ContentListItem) {
	if (!a || !b) return 0;
	return a.created_at < b.created_at ? -1 : a.created_at > b.created_at ? 1 : 0;
}

function dedupeById(items: ContentListItem[]): ContentListItem[] {
	const seen = new Set<string>();
	const out: ContentListItem[] = [];
	for (const item of items) {
		if (seen.has(item.id)) continue;
		seen.add(item.id);
		out.push(item);
	}
	return out;
}

/**
 * 平铺列表 → 楼层树。
 * 后端 `/posts/{id}/contents/list` 返回任意深度的平铺集合（created_at 升序），层级靠
 * parent_id 在前端还原；父层不在已载入集合里时（分页切断）该条先顶到一层，下一页把它
 * 带回来时自然归位。
 */
function buildTree(items: ContentListItem[]) {
	const byId = new Map(items.map((item) => [item.id, item]));
	const known = new Set(byId.keys());
	const childIds = new Map<string, string[]>();
	const roots: string[] = [];

	for (const item of items) {
		const parent = item.parent_id && item.parent_id !== item.id && known.has(item.parent_id) ? item.parent_id : '';
		if (parent) {
			const siblings = childIds.get(parent) ?? [];
			siblings.push(item.id);
			childIds.set(parent, siblings);
		} else {
			roots.push(item.id);
		}
	}

	for (const ids of childIds.values()) ids.sort((a, b) => orderAsc(byId.get(a), byId.get(b)));
	roots.sort((a, b) => orderAsc(byId.get(a), byId.get(b)));

	/** 某个楼层的全部后代（广度优先，用于超出 MAX_DEPTH 时把剩余楼层摊平展示，绝不丢内容）。 */
	function descendantsOf(id: string): string[] {
		const queue = [...(childIds.get(id) ?? [])];
		const out: string[] = [];
		while (queue.length > 0) {
			const current = queue.shift() as string;
			out.push(current);
			queue.push(...(childIds.get(current) ?? []));
		}
		return out;
	}

	return { roots, childIds, byId, descendantsOf };
}

function FloorSkeleton() {
	return (
		<div className="flex gap-3.5 py-4" aria-hidden="true">
			<div className="flex w-9 shrink-0 flex-col items-center gap-2">
				<Skeleton className="size-9" />
				<Skeleton className="h-8 w-px" />
			</div>
			<div className="min-w-0 flex-1 space-y-2">
				<Skeleton className="h-3.5 w-24" />
				<Skeleton className="h-3 w-full" />
				<Skeleton className="h-3 w-3/4" />
				<Skeleton className="h-3 w-1/2" />
			</div>
		</div>
	);
}

export function CommentList({ postId, pageSize, resumeFrom = '', cursorRef }: CommentListProps) {
	const queryClient = useQueryClient();
	const firstPage = useQuery(postContentsOptions(postId, 1, pageSize));
	/* 第 2 页起由 ScrollPaginator 触发追加（fetchQuery 直接进同一份缓存）。 */
	const [loadingMore, setLoadingMore] = useState(false);

	const [loaded, setLoaded] = useState<{ pageCount: number; tail: ContentListItem[]; ended: boolean }>({
		pageCount: 1,
		tail: [],
		ended: false,
	});
	/* 首屏那一页重新取回时（发布 / 删除 / 改版本后失效）把追加页清掉，等同旧 refetch()
	 回到第一页。用 React 官方的「渲染期复位派生 state」而不是 useEffect，少一帧闪烁。 */
	const resetToken = `${postId}:${firstPage.dataUpdatedAt}`;
	const [token, setToken] = useState(resetToken);
	if (token !== resetToken) {
		setToken(resetToken);
		setLoaded({ pageCount: 1, tail: [], ended: false });
	}

	const head = firstPage.data?.data ?? [];
	const total = firstPage.data?.total ?? 0;
	const items = useMemo(() => dedupeById([...head, ...loaded.tail]), [head, loaded.tail]);
	const hasMore = !loaded.ended && items.length < total;

	const loadMore = useCallback(async () => {
		if (loadingMore || !hasMore) return;
		setLoadingMore(true);
		const next = loaded.pageCount + 1;
		try {
			const result = await queryClient.fetchQuery(postContentsOptions(postId, next, pageSize));
			setLoaded((prev) => ({
				pageCount: next,
				tail: dedupeById([...prev.tail, ...(result.data ?? [])]),
				ended: next * pageSize >= (result.total ?? 0),
			}));
		} catch {
			// 旧版失败即 hasMore=false，避免滚动到底反复重试
			setLoaded((prev) => ({ ...prev, ended: true }));
		} finally {
			setLoadingMore(false);
		}
	}, [hasMore, loaded.pageCount, loadingMore, postId, queryClient, pageSize]);

	const tree = useMemo(() => buildTree(items), [items]);

	/* ── 阅读位置（续读游标）───────────────────────────────────────────────
	 * 「读到哪一条」用几何位置判定，不用渲染顺序表：IntersectionObserver 的 root
	 * 被 rootMargin 削成视口上部 60%（阅读区），落在阅读区里、位置最靠下的那一层
	 * 就是正在读的那条。这样既不依赖楼层树的展开顺序，也不受分页追加影响。
	 */
	const containerRef = useRef<HTMLDivElement | null>(null);
	const lastReported = useRef<string>('');
	const reportTimer = useRef<number | undefined>(undefined);
	/* 只有真正由用户发起的滚动/键盘翻页才允许上报。否则观察器挂载时的首帧回调
	   会把「页面顶部那条」当成新位置，在续读锚点（尤其位于后续分页）还没渲染出来
	   之前就把服务端保存的游标悄悄回退掉。程序触发的 scrollIntoView 不置位。 */
	const userEngaged = useRef(false);

	useEffect(() => {
		const engage = () => {
			userEngaged.current = true;
		};
		window.addEventListener('wheel', engage, { passive: true });
		window.addEventListener('touchmove', engage, { passive: true });
		window.addEventListener('keydown', engage);
		return () => {
			window.removeEventListener('wheel', engage);
			window.removeEventListener('touchmove', engage);
			window.removeEventListener('keydown', engage);
		};
	}, []);

	/* items.length 是刻意的多余依赖：effect 体不读它，但它代表「DOM 里多了一批楼层」，
	   新追加页的评论必须重新登记进观察器，否则阅读位置会冻结在第 1 页最后一条。
	   useExhaustiveDependencies 只看代码引用，看不见 DOM 成员变化。 */
	// biome-ignore lint/correctness/useExhaustiveDependencies: 依赖 DOM 集合变化，不是变量引用
	useEffect(() => {
		const root = containerRef.current;
		if (!root || typeof IntersectionObserver === 'undefined') return;

		const reading = new Set<Element>();
		const observer = new IntersectionObserver(
			(entries) => {
				for (const entry of entries) {
					if (entry.isIntersecting) reading.add(entry.target);
					else reading.delete(entry.target);
				}
				if (!userEngaged.current) return;
				let current: Element | null = null;
				for (const el of reading) {
					if (!current || el.getBoundingClientRect().top > current.getBoundingClientRect().top) current = el;
				}
				if (!current) return;
				const id = (current as HTMLElement).id.slice(FLOOR_ID_PREFIX.length);
				if (!id || id === lastReported.current) return;
				lastReported.current = id;

				window.clearTimeout(reportTimer.current);
				reportTimer.current = window.setTimeout(() => {
					// 先更新 ref，「标记已读」按钮点下去时带的就是同一个位置
					if (cursorRef) cursorRef.current = id;
					// 后台遥测：失败一律静默（下一次滚动会再报），不打扰阅读
					void postService.recordView(postId, id).catch(() => {});
				}, CURSOR_REPORT_DEBOUNCE_MS);
			},
			{ rootMargin: '0px 0px -40% 0px', threshold: 0 },
		);

		root.querySelectorAll<HTMLElement>(`[id^="${FLOOR_ID_PREFIX}"]`).forEach((el) => {
			observer.observe(el);
		});
		return () => {
			window.clearTimeout(reportTimer.current);
			observer.disconnect();
		};
	}, [postId, cursorRef, items.length]);

	/* 进入时回到上次的位置。只滚一次，且必须满足两个条件才滚：
	   1) 锚点已经渲染出来（分页追加后才有）；
	   2) 用户自己还没滚动过——否则会把正在翻页的用户硬拽回旧位置。 */
	const didResume = useRef(false);
	const userScrolled = useRef(false);
	useEffect(() => {
		const onScroll = () => {
			if (window.scrollY > 24) userScrolled.current = true;
		};
		window.addEventListener('scroll', onScroll, { passive: true });
		return () => window.removeEventListener('scroll', onScroll);
	}, []);
	// biome-ignore lint/correctness/useExhaustiveDependencies: 楼层是分页追加进来的，载入后才有锚点可滚
	useEffect(() => {
		if (didResume.current || !resumeFrom || userScrolled.current) return;
		const el = document.getElementById(`${FLOOR_ID_PREFIX}${resumeFrom}`);
		if (!el) return;
		didResume.current = true;
		el.scrollIntoView({ block: 'center' });
	}, [resumeFrom, items.length]);

	/* Solid 的递归 <For> 展开 → 递归渲染；key 一律用 content.id（嵌套楼层尤其不能用下标）。
	 超过 MAX_DEPTH 的后代不再继续套竖轨，而是摊平挂在最深层楼层下面，内容一条都不会少。 */
	const renderFloor = useCallback(
		(content: ContentListItem, depth: number): React.ReactNode => {
			const capped = depth >= MAX_DEPTH;
			const kids = capped ? tree.descendantsOf(content.id) : (tree.childIds.get(content.id) ?? []);
			return (
				<ContentItem
					key={content.id}
					content={content}
					postId={postId}
					className={depth === 0 ? undefined : 'py-3'}
					resumeHere={resumeFrom !== '' && content.id === resumeFrom}
				>
					{kids.map((id) => {
						const child = tree.byId.get(id);
						return child ? renderFloor(child, capped ? depth : depth + 1) : null;
					})}
				</ContentItem>
			);
		},
		[postId, resumeFrom, tree],
	);

	if (firstPage.isPending) {
		return (
			<div>
				<FloorSkeleton />
				<FloorSkeleton />
			</div>
		);
	}

	if (items.length === 0) {
		return <EmptyState title="暂无回复" description="成为第一个参与讨论的人。" />;
	}

	return (
		<div className="flex flex-col">
			<div ref={containerRef} className="divide-y divide-border/60">
				{tree.roots.map((id) => {
					const root = tree.byId.get(id);
					return root ? renderFloor(root, 0) : null;
				})}
			</div>

			{loadingMore ? (
				<div className="pt-4">
					<FloorSkeleton />
				</div>
			) : null}

			<ScrollPaginator
				className="mt-4"
				hasMore={hasMore}
				isLoading={loadingMore}
				onLoadMore={loadMore}
				showHasMore={false}
			/>
		</div>
	);
}

export default CommentList;
