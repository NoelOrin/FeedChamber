import type { ReactElement } from 'react';

import { Card } from '~/components/ui/card';
import { Skeleton } from '~/components/ui/skeleton';
import { cn } from '~/lib/utils';

/**
 * 旧 `components/ui/Skeleton.tsx` 的用户域预设平移到这里，全部基于 `~/components/ui/skeleton`
 * 的 `Skeleton` 组合（旧版的 `skeleton` 类在任何样式表里都没有定义，实际不发光，这里换成真的脉冲块）。
 * key 用「槽位」而不是下标：数量由 props 决定，槽位 id 只跟位置编号相关，重排不会串位。
 */

function skeletonSlots(count: number, prefix: string): string[] {
	return Array.from({ length: count }, (_, index) => `${prefix}-${String(index).padStart(3, '0')}`);
}

const WEEK_SLOTS = skeletonSlots(53, 'heatmap-week');
const DAY_SLOTS = skeletonSlots(7, 'heatmap-day');

/** 帖子条目：标题行 + 状态徽标 + 日期/评论/浏览元信息（与 post-item 的节奏一致）。 */
function PostItemSkeleton() {
	return (
		<li className="list-none px-4 py-3.5" aria-hidden="true">
			<div className="flex items-center gap-2">
				<Skeleton className="h-4 w-2/3" />
				<Skeleton className="h-5 w-12 shrink-0" />
			</div>
			<div className="mt-1.5 flex gap-3">
				<Skeleton className="h-3 w-16 rounded" />
				<Skeleton className="h-3 w-14 rounded" />
				<Skeleton className="h-3 w-12 rounded" />
			</div>
		</li>
	);
}

/** 版面/信息流用的帖子卡（旧 PostCardSkeleton，为用户域列表保留同一套节奏）。 */
export function PostCardSkeleton() {
	return (
		<Card className="gap-0 p-4" aria-hidden={true}>
			<div className="flex items-start gap-3">
				<Skeleton className="size-9 shrink-0" />
				<div className="min-w-0 flex-1 space-y-2">
					<Skeleton className="h-5 w-4/5" />
					<div className="flex items-center gap-3">
						<Skeleton className="h-3 w-14 rounded" />
						<Skeleton className="h-3 w-16 rounded" />
						<Skeleton className="h-3 w-12 rounded" />
					</div>
				</div>
			</div>
		</Card>
	);
}

/** 我的主页整卡（旧 UserProfileSkeleton：封面条 + 头像 + 两行文字 + 台账条）。 */
export function UserProfileSkeleton() {
	return (
		<Card className="gap-0 p-4" aria-hidden={true}>
			<div className="relative h-40 w-full overflow-hidden bg-muted h-40">
				<Skeleton className="h-full w-full" />
			</div>
			<div className="mt-4 flex items-start gap-5">
				<Skeleton className="size-20 shrink-0" />
				<div className="min-w-0 flex-1">
					<Skeleton className="h-6 w-32 rounded" />
					<Skeleton className="mt-2 h-4 w-48 rounded" />
				</div>
				<Skeleton className="h-8 w-20 shrink-0" />
			</div>
			<div className="mt-5 flex gap-6">
				<Skeleton className="h-4 w-16 rounded" />
				<Skeleton className="h-4 w-16 rounded" />
			</div>
			<div aria-hidden="true" className="my-5 h-px w-full border-b opacity-40" />
			<Skeleton className="mb-2 h-4 w-20 rounded" />
			<Skeleton className="h-12 w-full rounded" />
		</Card>
	);
}

/** 封面位（与 cover-image 的 156/188/220px 版心一致）。 */
export function CoverImageSkeleton() {
	return (
		<div aria-hidden="true" className="relative w-full overflow-hidden bg-muted">
			<Skeleton className="h-[156px] w-full sm:h-[188px] lg:h-[220px]" />
		</div>
	);
}

/** 台账条（stats-cards 的五列，旧版按 4 列画，这里对齐真实列数）。 */
export function StatsCardsSkeleton() {
	return (
		<div aria-hidden="true" className="grid grid-cols-2 gap-3 sm:grid-cols-4">
			{skeletonSlots(4, 'stats').map((slot) => (
				<div key={slot} className="rounded-md border border-border p-3 text-center">
					<Skeleton className="mx-auto h-6 w-12 rounded" />
					<Skeleton className="mx-auto mt-1 h-3 w-10 rounded" />
				</div>
			))}
		</div>
	);
}

/** 活跃度热力图（年份选择 + 7×N 周格 + 图例）。 */
export function ActivityHeatmapSkeleton() {
	return (
		<div aria-hidden="true">
			<div className="mb-2 flex items-center justify-between">
				<Skeleton className="h-4 w-16 rounded" />
				<Skeleton className="h-6 w-16 rounded" />
			</div>
			<div className="mb-1 ml-6 flex gap-[2px]">
				{WEEK_SLOTS.map((slot) => (
					<div key={slot} className="w-3 flex-shrink-0" />
				))}
			</div>
			<div className="flex">
				<div className="mr-1 flex flex-col gap-[2px]">
					{DAY_SLOTS.map((slot) => (
						<div key={slot} className="h-3 w-5" />
					))}
				</div>
				<div className="flex gap-[2px]">
					{WEEK_SLOTS.map((week) => (
						<div key={week} className="flex flex-col gap-[2px]">
							{DAY_SLOTS.map((day) => (
								<Skeleton key={`${week}-${day}`} className="size-3" />
							))}
						</div>
					))}
				</div>
			</div>
		</div>
	);
}

/** 关注/粉丝列表的一行（user-card 的节奏）。 */
function UserCardSkeleton() {
	return (
		<li aria-hidden="true" className="flex items-center gap-3 px-4 py-3">
			<Skeleton className="size-10 shrink-0" />
			<div className="min-w-0 flex-1">
				<Skeleton className="h-4 w-24 rounded" />
				<Skeleton className="mt-1 h-3 w-40 rounded" />
				<Skeleton className="mt-1 h-3 w-28 rounded" />
			</div>
			<Skeleton className="h-8 w-16 shrink-0" />
		</li>
	);
}

/** 通知条目。 */
function NotificationItemSkeleton() {
	return (
		<li aria-hidden="true" className="flex items-start gap-3 px-4 py-3">
			<Skeleton className="mt-1.5 size-2 shrink-0" />
			<div className="min-w-0 flex-1">
				<Skeleton className="h-4 w-full rounded" />
				<Skeleton className="mt-1 h-4 w-3/4 rounded" />
				<Skeleton className="mt-1 h-3 w-32 rounded" />
			</div>
		</li>
	);
}

/** 收藏条目。 */
function FavoriteItemSkeleton() {
	return (
		<li aria-hidden="true" className="flex items-start justify-between gap-2 px-4 py-3">
			<div className="min-w-0 flex-1">
				<Skeleton className="h-4 w-2/3 rounded" />
				<Skeleton className="mt-1.5 h-3 w-40 rounded" />
			</div>
		</li>
	);
}

/** 评论条目（旧 CommentItemSkeleton，用户域之外的列表也复用这一套）。 */
function CommentItemSkeleton() {
	return (
		<li aria-hidden="true" className="flex min-h-30 gap-2 rounded">
			<Skeleton className="size-12 shrink-0" />
			<div className="flex flex-1 flex-col justify-between gap-1 text-sm">
				<div>
					<div className="flex items-center gap-2">
						<Skeleton className="h-4 w-20 rounded" />
						<Skeleton className="h-3 w-28 rounded" />
					</div>
					<Skeleton className="mt-1 h-4 w-full rounded" />
					<Skeleton className="mt-1 h-4 w-3/4 rounded" />
				</div>
			</div>
		</li>
	);
}

/** 版面卡。 */
function SpaceCardSkeleton() {
	return (
		<div aria-hidden="true" className="flex items-start gap-4 rounded-lg border border-border bg-card p-4">
			<Skeleton className="size-10 shrink-0 rounded" />
			<div className="min-w-0 flex-1">
				<Skeleton className="h-5 w-24 rounded" />
				<Skeleton className="mt-1 h-3 w-full rounded" />
				<Skeleton className="mt-1 h-3 w-3/4 rounded" />
				<Skeleton className="mt-2 h-3 w-14 rounded" />
			</div>
		</div>
	);
}

/** 帖子详情（页头 + 正文 + 三条回复）。 */
/** Markdown 编辑器（工具栏两排按钮 + 正文行）。 */
export function EditorSkeleton() {
	const groups = [
		{ slot: 'toolbar-main', count: 9 },
		{ slot: 'toolbar-heading', count: 3 },
		{ slot: 'toolbar-block', count: 4 },
	];
	return (
		<div aria-hidden="true" className="w-full space-y-3 rounded-md border border-border px-3 py-3 text-sm">
			<div className="flex flex-wrap items-center gap-1">
				{groups.map((group, groupIndex) => (
					<div key={group.slot} className="flex items-center gap-1">
						{skeletonSlots(group.count, group.slot).map((slot) => (
							<Skeleton key={slot} className="size-7 rounded" />
						))}
						{groupIndex < groups.length - 1 ? <span className="mx-1 h-5 w-px bg-border" aria-hidden="true" /> : null}
					</div>
				))}
			</div>
			<div className="space-y-3">
				{skeletonSlots(3, 'editor-line').map((slot, index) => (
					<Skeleton
						key={slot}
						className={cn('h-3 rounded', index === 1 ? 'w-3/4' : index === 2 ? 'w-1/2' : 'w-full')}
					/>
				))}
			</div>
		</div>
	);
}

type SkeletonListType =
	| 'item'
	| 'card'
	| 'user-card'
	| 'notification'
	| 'favorite-item'
	| 'comment'
	| 'space-card'
	| 'editor';

const SKELETON_BY_TYPE: Record<SkeletonListType, () => ReactElement> = {
	item: PostItemSkeleton,
	card: PostCardSkeleton,
	'user-card': UserCardSkeleton,
	notification: NotificationItemSkeleton,
	'favorite-item': FavoriteItemSkeleton,
	comment: CommentItemSkeleton,
	'space-card': SpaceCardSkeleton,
	editor: EditorSkeleton,
};

/** 旧 `SkeletonList` 的等价件：按类型重复渲染 N 个预设骨架。 */
export function SkeletonList({ count, type }: { count: number; type: SkeletonListType }) {
	const Preset = SKELETON_BY_TYPE[type];
	return (
		<>
			{skeletonSlots(count, `skeleton-list-${type}`).map((slot) => (
				<Preset key={slot} />
			))}
		</>
	);
}
