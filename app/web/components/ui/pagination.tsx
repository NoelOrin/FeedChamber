'use client';

import { Button } from '~/components/ui/button';
import { ChevronLeft, ChevronRight } from '~/lib/icons';
import { cn } from '~/lib/utils';

/*
 * 分页条：页码窗口 + 上/下页。
 *
 * 与后端契约一致：page 从 1 开始，pageSize 固定由调用方给。只受控，不内置状态。
 * 页码窗口算法：总页数 ≤ 7 全展开；否则首末页常驻，当前页左右各 1 页，
 * 空档折叠成省略号（避免 antd 那种 10 个数字挤成一行）。
 */

export interface PaginationProps {
	page: number;
	pageSize: number;
	total: number;
	onChange: (page: number) => void;
	disabled?: boolean;
	className?: string;
	/** 是否显示「共 N 条」 */
	showTotal?: boolean;
}

function buildPages(current: number, totalPages: number): (number | 'ellipsis')[] {
	if (totalPages <= 7) return Array.from({ length: totalPages }, (_, i) => i + 1);
	const pages: (number | 'ellipsis')[] = [1];
	const left = Math.max(2, current - 1);
	const right = Math.min(totalPages - 1, current + 1);
	if (left > 2) pages.push('ellipsis');
	for (let i = left; i <= right; i++) pages.push(i);
	if (right < totalPages - 1) pages.push('ellipsis');
	pages.push(totalPages);
	return pages;
}

export function Pagination({
	page,
	pageSize,
	total,
	onChange,
	disabled,
	className,
	showTotal = true,
}: PaginationProps) {
	const totalPages = Math.max(1, Math.ceil(total / pageSize));
	const pages = buildPages(page, totalPages);

	return (
		<nav
			data-slot="pagination"
			aria-label="分页"
			className={cn('flex flex-wrap items-center justify-between gap-3', className)}
		>
			{showTotal ? (
				<span className="text-[13px] text-muted-foreground tabular-nums">
					共 {total} 条 · 第 {page}/{totalPages} 页
				</span>
			) : (
				<span />
			)}
			<div className="flex items-center gap-1">
				<Button
					variant="outline"
					size="sm"
					disabled={disabled || page <= 1}
					onClick={() => onChange(page - 1)}
					aria-label="上一页"
				>
					<ChevronLeft className="size-3.5" />
				</Button>
				{pages.map((p, idx) =>
					p === 'ellipsis' ? (
						<span
							// biome-ignore lint/suspicious/noArrayIndexKey: 省略号位置由算法决定，无稳定 id
							key={`e-${idx}`}
							className="px-1.5 text-[13px] text-muted-foreground"
						>
							…
						</span>
					) : (
						<Button
							key={p}
							variant={p === page ? 'default' : 'outline'}
							size="sm"
							className="min-w-8 tabular-nums"
							disabled={disabled}
							onClick={() => onChange(p)}
							aria-current={p === page ? 'page' : undefined}
						>
							{p}
						</Button>
					),
				)}
				<Button
					variant="outline"
					size="sm"
					disabled={disabled || page >= totalPages}
					onClick={() => onChange(page + 1)}
					aria-label="下一页"
				>
					<ChevronRight className="size-3.5" />
				</Button>
			</div>
		</nav>
	);
}
