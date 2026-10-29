'use client';

import { type ReactNode, useEffect, useRef } from 'react';

import { LoaderCircle } from '~/lib/icons';
import { cn } from '~/lib/utils';

interface ScrollPaginatorProps {
	className?: string;
	/** 旧 Solid 版的 `class` 属性名，列表消费方平移期仍可照旧调用 */
	class?: string;
	hasMore: boolean;
	isLoading: boolean;
	onLoadMore: () => void;
	loadingComponent?: ReactNode;
	threshold?: number;
	showHasMore?: boolean;
}

/** 旧 daisyUI `.loading.loading-spinner` → lucide LoaderCircle（Loading 原语同一套） */
function DefaultLoading() {
	return (
		<div className="flex min-h-8 items-center justify-center gap-2 py-4">
			<LoaderCircle className="size-4 animate-spin text-muted-foreground" aria-hidden="true" />
			<span className="text-xs text-muted-foreground">加载中...</span>
		</div>
	);
}

/** 无限滚动哨兵：纯展示 + 回调，不发请求（请求由宿主列表负责）。 */
export function ScrollPaginator({
	class: legacyClass,
	className,
	hasMore,
	isLoading,
	onLoadMore,
	loadingComponent,
	threshold,
	showHasMore = true,
}: ScrollPaginatorProps) {
	const sentinelRef = useRef<HTMLDivElement | null>(null);
	const observerRef = useRef<IntersectionObserver | null>(null);
	// 回调只在挂载时建一次，读最新 props 走 ref，避免捕获旧闭包
	const latestRef = useRef({ hasMore, isLoading, onLoadMore, threshold });

	useEffect(() => {
		latestRef.current = { hasMore, isLoading, onLoadMore, threshold };
	}, [hasMore, isLoading, onLoadMore, threshold]);

	useEffect(() => {
		if (typeof IntersectionObserver === 'undefined') return;

		const observer = new IntersectionObserver(
			(entries) => {
				const entry = entries[0];
				if (!entry?.isIntersecting) return;
				const latest = latestRef.current;
				if (!latest.hasMore || latest.isLoading) return;
				// unobserve to prevent duplicate triggers while loading
				observerRef.current?.unobserve(entry.target);
				latest.onLoadMore();
			},
			{ threshold: 1, rootMargin: `-${latestRef.current.threshold ?? 110}px` },
		);

		observerRef.current = observer;
		if (sentinelRef.current) observer.observe(sentinelRef.current);

		return () => {
			observer.disconnect();
			observerRef.current = null;
		};
	}, []);

	// Re-observe after loading finishes or hasMore changes,
	// so the next batch can be triggered once all items are rendered.
	useEffect(() => {
		const observer = observerRef.current;
		const sentinel = sentinelRef.current;
		if (!observer || !sentinel) return;
		observer.unobserve(sentinel);
		if (isLoading || !hasMore) return;
		// give the browser a frame to lay out new items before re-observing
		const frame = requestAnimationFrame(() => {
			observerRef.current?.observe(sentinel);
		});
		return () => cancelAnimationFrame(frame);
	}, [isLoading, hasMore]);

	return (
		<div className={cn('w-full', className, legacyClass)}>
			<div ref={sentinelRef}>
				{isLoading ? (loadingComponent ?? <DefaultLoading />) : null}
				{!isLoading && !hasMore && showHasMore ? (
					<div className="py-4 text-center text-xs text-muted-foreground">没有更多了</div>
				) : null}
			</div>
		</div>
	);
}
