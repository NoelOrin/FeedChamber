import type * as React from 'react';

import { cn } from '~/lib/utils';

/*
 * 页头：标题 + 可选计数 + 右侧操作。
 *
 * 刻意不提供 description 槽位。
 * 之前 11 个页面都用同一句「h1 text-2xl + mt-1 text-[13px] 说明性副标题」，
 * 副标题又几乎都在复述标题（「时间线 / 4 条主题」「版面 / 4 个版面 · 4 个大区」），
 * 除了把版面撑高一行之外没有信息量——这是模板感最重的一处。
 * 真正需要补充说明的页面（搜索命中数、关注人数）把计数放进 `meta`，用数字而不是整句话。
 */
export function PageHeader({
	title,
	meta,
	actions,
	className,
}: {
	title: React.ReactNode;
	/** 计数或状态，渲染在标题右侧同一行；没有就不占位 */
	meta?: React.ReactNode;
	actions?: React.ReactNode;
	className?: string;
}) {
	return (
		<header className={cn('mb-5 flex flex-wrap items-center justify-between gap-3', className)}>
			<div className="flex min-w-0 items-baseline gap-2.5">
				<h1 className="truncate text-2xl leading-tight font-semibold tracking-tight text-foreground">{title}</h1>
				{meta ? <span className="shrink-0 text-[13px] text-muted-foreground tabular-nums">{meta}</span> : null}
			</div>
			{actions ? <div className="flex shrink-0 items-center gap-2">{actions}</div> : null}
		</header>
	);
}
