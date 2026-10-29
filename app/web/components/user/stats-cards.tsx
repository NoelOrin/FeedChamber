import { StatsCardsSkeleton } from '~/components/user/skeletons';
import { cn } from '~/lib/utils';

interface StatsCardsProps {
	postCount: number;
	commentCount: number;
	likeCount: number;
	daysSinceRegister: number;
	points?: number;
	loading?: boolean;
	className?: string;
}

export function StatsCards({
	postCount,
	commentCount,
	likeCount,
	daysSinceRegister,
	points,
	loading,
	className,
}: StatsCardsProps) {
	if (loading) return <StatsCardsSkeleton />;

	const cells = [
		{ key: 'posts', label: '帖子', value: postCount },
		{ key: 'comments', label: '评论', value: commentCount },
		{ key: 'likes', label: '获赞', value: likeCount },
		{ key: 'points', label: '积分', value: points ?? 0 },
		{ key: 'days', label: '驻留天数', value: daysSinceRegister },
	];

	return (
		<dl
			className={cn(
				'grid grid-cols-3 gap-1 rounded-lg border border-border p-1 sm:grid-cols-5 sm:gap-0 sm:divide-x sm:divide-border sm:p-0',
				className,
			)}
		>
			{cells.map((cell) => (
				<div key={cell.key} className="flex flex-col gap-0.5 rounded-md px-3 py-2.5 sm:rounded-none sm:px-4 sm:py-3">
					<dd className="text-lg leading-tight font-semibold text-foreground tabular-nums">{cell.value}</dd>
					<dt className="text-xs text-muted-foreground">{cell.label}</dt>
				</div>
			))}
		</dl>
	);
}
