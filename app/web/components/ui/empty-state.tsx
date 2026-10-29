import type * as React from 'react';

import { Inbox, Layers, type LucideIcon, Search } from '~/lib/icons';
import { cn } from '~/lib/utils';

const iconOf: Record<'default' | 'search' | 'spaces', LucideIcon> = {
	default: Inbox,
	search: Search,
	spaces: Layers,
};

export function EmptyState({
	variant = 'default',
	title = '暂无数据',
	description,
	icon,
	className,
	children,
	...props
}: React.ComponentProps<'div'> & {
	variant?: 'default' | 'search' | 'spaces';
	title?: string;
	description?: string;
	icon?: React.ReactNode;
}) {
	const Icon = iconOf[variant];
	return (
		<div
			className={cn(
				'flex w-full flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-border px-6 py-12 text-center',
				className,
			)}
			{...props}
		>
			<span className="mb-1 text-muted-foreground">{icon ?? <Icon className="size-6" aria-hidden="true" />}</span>
			<span className="text-sm font-medium text-foreground">{title}</span>
			{description ? (
				<span className="max-w-sm text-xs leading-relaxed text-muted-foreground">{description}</span>
			) : null}
			{children ? <div className="mt-2 flex items-center gap-2">{children}</div> : null}
		</div>
	);
}
