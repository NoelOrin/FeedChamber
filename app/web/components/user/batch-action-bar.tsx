'use client';

import { Button } from '~/components/ui/button';
import { cn } from '~/lib/utils';

interface BatchActionBarProps {
	selectedCount: number;
	totalCount: number;
	onSelectAll: () => void;
	onDeselectAll: () => void;
	onBatchFollow: () => void;
	onBatchUnfollow: () => void;
	className?: string;
}

/**
 * 批量操作条（旧 -modules/BatchActionBar.tsx）。
 * 按钮文案按旧版原样保留（通知页复用时同样是「批量关注 / 批量取关」，旧版即如此，未在迁移期改文案）。
 */
export function BatchActionBar({
	selectedCount,
	totalCount,
	onSelectAll,
	onDeselectAll,
	onBatchFollow,
	onBatchUnfollow,
	className,
}: BatchActionBarProps) {
	return (
		<div
			className={cn(
				'flex flex-wrap items-center gap-2 rounded-md border border-border bg-muted/50 px-3 py-2',
				className,
			)}
		>
			<span className="text-sm text-muted-foreground">
				已选 <span className="font-bold text-foreground">{selectedCount}</span> 项
			</span>
			{selectedCount < totalCount ? (
				<Button size="xs" variant="ghost" onClick={onSelectAll}>
					全选
				</Button>
			) : null}
			{selectedCount > 0 ? (
				<Button size="xs" variant="ghost" onClick={onDeselectAll}>
					取消全选
				</Button>
			) : null}
			<div className="flex-1" />
			{selectedCount > 0 ? (
				<>
					<Button size="xs" onClick={onBatchFollow}>
						批量关注
					</Button>
					<Button size="xs" variant="outline" className="text-destructive" onClick={onBatchUnfollow}>
						批量取关
					</Button>
				</>
			) : null}
		</div>
	);
}
