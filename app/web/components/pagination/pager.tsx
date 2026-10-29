'use client';

import { ChevronLeft, ChevronRight } from '~/lib/icons';
import { cn } from '~/lib/utils';

interface PagerProps {
	currentPage: number;
	totalPages: number;
	onPageChange: (page: number) => void;
	/** 追加到外层 <nav> 的类名，供宿主控制间距（默认仍是旧版的 mt-6） */
	className?: string;
}

const CELL_BASE =
	'inline-flex h-8 min-w-8 shrink-0 cursor-pointer items-center justify-center rounded-md border border-transparent px-2 text-xs font-medium text-muted-foreground tabular-nums transition-colors hover:bg-muted hover:text-foreground disabled:pointer-events-none disabled:opacity-40';

const CELL_ACTIVE = 'border-border bg-muted text-foreground';

/** 页码单元格的共享样式：Pager / Paginator 共用同一份，避免第二套分页视觉。 */
function pagerCellClass(active?: boolean, className?: string) {
	return cn(CELL_BASE, active && CELL_ACTIVE, className);
}

/** 省略位（旧版用 -1 / -2 两个负数占位，这里保留同一编码，key 因此天然稳定唯一）。 */
const ELLIPSIS_LEFT = -1;
const ELLIPSIS_RIGHT = -2;

function buildPages(currentPage: number, totalPages: number): number[] {
	const maxButtons = 7;

	if (totalPages <= maxButtons) {
		return Array.from({ length: totalPages }, (_, i) => i + 1);
	}

	const pages: number[] = [1];
	const range = 2;
	let start = Math.max(2, currentPage - range);
	let end = Math.min(totalPages - 1, currentPage + range);

	// Slide window to always show 5 middle buttons
	if (currentPage <= range + 1) {
		end = Math.min(totalPages - 1, maxButtons - 2);
	} else if (currentPage >= totalPages - range) {
		start = Math.max(2, totalPages - maxButtons + 3);
	}

	if (start > 2) pages.push(ELLIPSIS_LEFT);
	for (let i = start; i <= end; i++) pages.push(i);
	if (end < totalPages - 1) pages.push(ELLIPSIS_RIGHT);
	pages.push(totalPages);

	return pages;
}

export function Pager({ currentPage, totalPages, onPageChange, className }: PagerProps) {
	const goto = (page: number) => {
		if (page >= 1 && page <= totalPages && page !== currentPage) {
			onPageChange(page);
		}
	};

	if (totalPages <= 1) return null;

	return (
		<nav aria-label="分页" className={cn('mt-6 flex justify-center', className)}>
			<div className="flex flex-wrap items-center justify-center gap-1">
				<button
					type="button"
					className={pagerCellClass()}
					disabled={currentPage <= 1}
					onClick={() => goto(currentPage - 1)}
					title="上一页"
					aria-label="上一页"
				>
					<ChevronLeft className="size-3.5" aria-hidden="true" />
				</button>
				{buildPages(currentPage, totalPages).map((p) =>
					p > 0 ? (
						<button
							key={`page-${p}`}
							type="button"
							className={pagerCellClass(p === currentPage)}
							onClick={() => goto(p)}
							aria-current={p === currentPage ? 'page' : undefined}
							aria-label={`第 ${p} 页`}
						>
							{p}
						</button>
					) : (
						<span
							key={`ellipsis-${p}`}
							aria-hidden="true"
							className="inline-flex h-8 min-w-8 shrink-0 items-center justify-center text-xs text-muted-foreground select-none"
						>
							…
						</span>
					),
				)}
				<button
					type="button"
					className={pagerCellClass()}
					disabled={currentPage >= totalPages}
					onClick={() => goto(currentPage + 1)}
					title="下一页"
					aria-label="下一页"
				>
					<ChevronRight className="size-3.5" aria-hidden="true" />
				</button>
			</div>
		</nav>
	);
}
