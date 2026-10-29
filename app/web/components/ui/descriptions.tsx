import type * as React from 'react';

import { cn } from '~/lib/utils';

/*
 * 详情描述列表（替代 antd ProDescriptions）。
 *
 * 只是「标签 / 值」的二维网格，不需要 antd 那套 column/span/bordered/size
 * 的排列组合。这里用 CSS grid：label 列固定宽度、值列自适应，
 * 用 grid-column 表达跨列。
 */

export function Descriptions({ className, children }: { className?: string; children: React.ReactNode }) {
	return (
		<dl
			data-slot="descriptions"
			className={cn('grid grid-cols-[max-content_1fr_max-content_1fr] items-start gap-x-6 gap-y-3', className)}
		>
			{children}
		</dl>
	);
}

export function DescriptionsItem({
	label,
	children,
	span = 1,
	className,
}: {
	label: React.ReactNode;
	children: React.ReactNode;
	/** 值占几列（1 = 半行，2 = 整行） */
	span?: 1 | 2;
	className?: string;
}) {
	return (
		<div
			data-slot="descriptions-item"
			className={cn(
				'grid gap-x-6 gap-y-1',
				span === 2 ? 'col-span-4 grid-cols-[max-content_1fr]' : 'col-span-2 grid-cols-[max-content_1fr]',
				className,
			)}
		>
			<dt className="text-muted-foreground text-[13px] leading-6 whitespace-nowrap">{label}</dt>
			<dd className="min-w-0 text-[13px] leading-6 break-words text-foreground">{children}</dd>
		</div>
	);
}
