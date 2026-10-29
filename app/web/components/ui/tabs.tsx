'use client';

import * as TabsPrimitive from '@radix-ui/react-tabs';
import { cva, type VariantProps } from 'class-variance-authority';
import type * as React from 'react';

import { cn } from '~/lib/utils';

export const Tabs = TabsPrimitive.Root;

/* 分段控件：灰槽 + 白底游标（shadcn 默认形态）。 */
export function TabsList({ className, ...props }: React.ComponentProps<typeof TabsPrimitive.List>) {
	return (
		<TabsPrimitive.List
			data-slot="tabs-list"
			className={cn('inline-flex h-8 items-center gap-0.5 rounded-lg bg-muted p-0.5 text-muted-foreground', className)}
			{...props}
		/>
	);
}

const tabsTriggerVariants = cva(
	'inline-flex h-7 cursor-pointer items-center justify-center gap-1.5 rounded-md px-2.5 text-xs font-medium whitespace-nowrap transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/80 disabled:pointer-events-none disabled:opacity-50 [&_svg]:size-3.5 [&_svg]:shrink-0',
	{
		variants: {
			variant: {
				capsule:
					'text-muted-foreground hover:text-foreground data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-xs',
				line: 'rounded-none border-b-2 border-transparent px-2 text-muted-foreground hover:text-foreground data-[state=active]:border-primary data-[state=active]:text-foreground',
				plate:
					'text-muted-foreground hover:text-foreground data-[state=active]:bg-foreground data-[state=active]:text-background',
			},
		},
		defaultVariants: { variant: 'capsule' },
	},
);

export function TabsTrigger({
	className,
	variant,
	...props
}: React.ComponentProps<typeof TabsPrimitive.Trigger> & VariantProps<typeof tabsTriggerVariants>) {
	return (
		<TabsPrimitive.Trigger
			data-slot="tabs-trigger"
			className={cn(tabsTriggerVariants({ variant }), className)}
			{...props}
		/>
	);
}

export function TabsContent({ className, ...props }: React.ComponentProps<typeof TabsPrimitive.Content>) {
	return <TabsPrimitive.Content data-slot="tabs-content" className={cn('outline-none', className)} {...props} />;
}
