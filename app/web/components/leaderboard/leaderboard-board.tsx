'use client';

import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { type LeaderboardRow, type LeaderboardType, leaderboardOptions } from '~/components/leaderboard/queries';
import { Avatar, AvatarFallback, AvatarImage } from '~/components/ui/avatar';
import { Card } from '~/components/ui/card';
import { EmptyState } from '~/components/ui/empty-state';
import { PageHeader } from '~/components/ui/page-header';
import { Segmented } from '~/components/ui/segmented';
import { Skeleton } from '~/components/ui/skeleton';
import { cn } from '~/lib/utils';

const TABS: Array<{ value: LeaderboardType; label: string }> = [
	{ value: 'points', label: '经验' },
	{ value: 'checkin', label: '签到' },
	{ value: 'posts', label: '发帖' },
];

export function LeaderboardBoard() {
	const [type, setType] = useState<LeaderboardType>('points');
	const { data, isPending } = useQuery(leaderboardOptions(type));
	const rows = data?.data ?? [];

	/* 三种榜各读自己的指标：积分榜 total_xp、签到榜打卡天数、发帖榜 post_count。
	   此前发帖榜误读 total_points，导致榜单按积分排序却显示积分。 */
	const score = (row: LeaderboardRow) =>
		type === 'checkin' ? row.total_checkins : type === 'posts' ? (row.post_count ?? 0) : row.total_xp;
	const unit = type === 'checkin' ? '天' : type === 'posts' ? '帖' : 'XP';

	return (
		<div className="flex w-full flex-col gap-3 pb-10">
			<PageHeader
				title="荣誉榜"
				actions={
					<Segmented
						label="榜单类型"
						value={type}
						options={TABS}
						onValueChange={(value) => setType(value as LeaderboardType)}
					/>
				}
			/>

			{isPending ? (
				<Card className="gap-0 divide-y divide-border/60 p-0">
					{(['one', 'two', 'three', 'four'] as const).map((row) => (
						<div key={`rank-skeleton-${row}`} className="flex items-center gap-3 px-4 py-3">
							<Skeleton className="size-6" />
							<Skeleton className="size-8 rounded-full" />
							<div className="flex-1 space-y-1.5">
								<Skeleton className="h-3.5 w-24" />
								<Skeleton className="h-3 w-20" />
							</div>
							<Skeleton className="h-4 w-10" />
						</div>
					))}
				</Card>
			) : rows.length === 0 ? (
				<EmptyState title="暂无排行" description="还没有上榜数据。" />
			) : (
				<Card className="gap-0 divide-y divide-border/60 overflow-hidden p-0">
					{rows.map((row, index) => (
						<div key={row.id} className="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-muted/50">
							<span
								className={cn(
									'inline-grid size-6 shrink-0 place-items-center rounded-md text-xs tabular-nums',
									index < 3 ? 'bg-muted font-medium text-foreground' : 'text-muted-foreground',
								)}
							>
								{index + 1}
							</span>
							<Avatar className="size-8">
								{row.avatar ? <AvatarImage src={row.avatar} alt="" /> : null}
								<AvatarFallback className="text-[11px]">
									{(row.display_name || row.username || '?').slice(0, 1).toUpperCase()}
								</AvatarFallback>
							</Avatar>
							<div className="min-w-0 flex-1">
								<div className="truncate text-[13px] font-medium text-foreground">
									{row.display_name || row.username}
								</div>
								<div className="truncate text-[11px] text-muted-foreground">@{row.username}</div>
							</div>
							<div className="shrink-0 text-right">
								<div className="text-[13px] font-semibold text-foreground tabular-nums">{score(row)}</div>
								<div className="text-[11px] text-muted-foreground">{unit}</div>
							</div>
						</div>
					))}
				</Card>
			)}
		</div>
	);
}
