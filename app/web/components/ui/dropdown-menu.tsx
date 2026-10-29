'use client';

import * as DropdownMenuPrimitive from '@radix-ui/react-dropdown-menu';
import type * as React from 'react';

import { cn } from '~/lib/utils';

export const DropdownMenu = DropdownMenuPrimitive.Root;
export const DropdownMenuTrigger = DropdownMenuPrimitive.Trigger;

const contentClass =
	'z-[var(--z-popover)] min-w-40 overflow-hidden rounded-md border border-border bg-popover p-1 text-popover-foreground shadow-md duration-150 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95 focus:outline-none';

export function DropdownMenuContent({
	className,
	sideOffset = 4,
	...props
}: React.ComponentProps<typeof DropdownMenuPrimitive.Content>) {
	return (
		<DropdownMenuPrimitive.Portal>
			<DropdownMenuPrimitive.Content
				data-slot="dropdown-content"
				sideOffset={sideOffset}
				className={cn(contentClass, className)}
				{...props}
			/>
		</DropdownMenuPrimitive.Portal>
	);
}

const itemClass =
	'flex w-full cursor-pointer items-center gap-2 rounded-sm px-2 py-1.5 text-[13px] text-foreground outline-none select-none data-[disabled]:pointer-events-none data-[disabled]:opacity-50 data-[inset]:pl-7 focus:bg-muted [&_svg]:size-4 [&_svg]:shrink-0';

export function DropdownMenuItem({
	className,
	inset,
	variant,
	...props
}: React.ComponentProps<typeof DropdownMenuPrimitive.Item> & { inset?: boolean; variant?: 'default' | 'destructive' }) {
	return (
		<DropdownMenuPrimitive.Item
			data-slot="dropdown-item"
			data-inset={inset}
			data-variant={variant}
			className={cn(itemClass, 'data-[variant=destructive]:text-destructive', className)}
			{...props}
		/>
	);
}

export function DropdownMenuLabel({
	className,
	inset,
	...props
}: React.ComponentProps<typeof DropdownMenuPrimitive.Label> & { inset?: boolean }) {
	return (
		<DropdownMenuPrimitive.Label
			data-slot="dropdown-label"
			data-inset={inset}
			className={cn('px-2 py-1.5 text-xs font-medium text-muted-foreground', inset && 'pl-7', className)}
			{...props}
		/>
	);
}

export function DropdownMenuSeparator({
	className,
	...props
}: React.ComponentProps<typeof DropdownMenuPrimitive.Separator>) {
	return <DropdownMenuPrimitive.Separator className={cn('-mx-1 my-1 h-px bg-border', className)} {...props} />;
}
