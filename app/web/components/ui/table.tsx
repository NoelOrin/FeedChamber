import type * as React from 'react';

import { cn } from '~/lib/utils';

/*
 * 表格结构壳（shadcn 基准）。
 *
 * 只提供「语义 + 几何」：<table> 负责布局，不负责交互。排序、行选择、分页
 * 由 DataTable 组合 TanStack Table 完成；这里刻意不内置任何状态。
 *
 * 视觉沿用主站语言：表头用 --muted 灰底、行之间 1px --border 分隔、
 * 悬停用 --muted/60，无斑马纹（斑马纹是 antd 的观感，主站不用）。
 */

export function Table({ className, ...props }: React.ComponentProps<'table'>) {
	return (
		<div className="relative w-full overflow-x-auto">
			<table data-slot="table" className={cn('w-full caption-bottom text-sm', className)} {...props} />
		</div>
	);
}

export function TableHeader({ className, ...props }: React.ComponentProps<'thead'>) {
	return <thead data-slot="table-header" className={cn('[&_tr]:border-b', className)} {...props} />;
}

export function TableBody({ className, ...props }: React.ComponentProps<'tbody'>) {
	return <tbody data-slot="table-body" className={cn('[&_tr:last-child]:border-0', className)} {...props} />;
}

export function TableFooter({ className, ...props }: React.ComponentProps<'tfoot'>) {
	return (
		<tfoot
			data-slot="table-footer"
			className={cn('border-t bg-muted/50 font-medium [&>tr]:last:border-b-0', className)}
			{...props}
		/>
	);
}

export function TableRow({ className, ...props }: React.ComponentProps<'tr'>) {
	return (
		<tr
			data-slot="table-row"
			className={cn(
				'border-b border-border transition-colors hover:bg-muted/60 data-[state=selected]:bg-muted',
				className,
			)}
			{...props}
		/>
	);
}

export function TableHead({ className, ...props }: React.ComponentProps<'th'>) {
	return (
		<th
			data-slot="table-head"
			className={cn(
				'h-10 px-3 text-left align-middle text-[13px] font-medium whitespace-nowrap text-muted-foreground',
				'[&:has([role=checkbox])]:pr-0',
				className,
			)}
			{...props}
		/>
	);
}

export function TableCell({ className, ...props }: React.ComponentProps<'td'>) {
	return (
		<td
			data-slot="table-cell"
			className={cn('px-3 py-2.5 align-middle text-[13px] text-foreground [&:has([role=checkbox])]:pr-0', className)}
			{...props}
		/>
	);
}

export function TableCaption({ className, ...props }: React.ComponentProps<'caption'>) {
	return (
		<caption data-slot="table-caption" className={cn('mt-4 text-[13px] text-muted-foreground', className)} {...props} />
	);
}
