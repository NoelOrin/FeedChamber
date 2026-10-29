import { Skeleton } from '~/components/ui/skeleton';

function slots(count: number, prefix: string): string[] {
	return Array.from({ length: count }, (_, index) => `${prefix}-${String(index).padStart(3, '0')}`);
}

export function PostCardSkeleton() {
	return (
		<div className="flex items-start gap-3 px-4 py-3.5" aria-hidden={true}>
			<Skeleton className="size-8 shrink-0 rounded-full" />
			<div className="min-w-0 flex-1 space-y-2">
				<Skeleton className="h-4 w-4/5" />
				<div className="flex items-center gap-2">
					<Skeleton className="h-3 w-10" />
					<Skeleton className="h-3 w-8" />
					<Skeleton className="h-3 w-10" />
				</div>
			</div>
			<Skeleton className="hidden h-3 w-16 sm:block" />
		</div>
	);
}

/** 列表首屏骨架：与真实行同解剖。 */
export function PostListSkeleton({ count = 5 }: { count?: number }) {
	return (
		<div className="divide-y divide-border/60 overflow-hidden rounded-lg border border-border bg-card">
			{slots(count, 'post-card-skeleton').map((slot) => (
				<PostCardSkeleton key={slot} />
			))}
		</div>
	);
}

/** 翻页时尾部补位的轻量骨架（ScrollPaginator 的 loadingComponent）。 */
export function PostFeedMoreSkeleton({ count = 2 }: { count?: number }) {
	return (
		<div className="divide-y divide-border/60 overflow-hidden rounded-lg border border-border bg-card">
			{slots(count, 'post-more-skeleton').map((slot) => (
				<PostCardSkeleton key={slot} />
			))}
		</div>
	);
}
