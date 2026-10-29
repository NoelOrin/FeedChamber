'use client';

import * as DialogPrimitive from '@radix-ui/react-dialog';
import { cva, type VariantProps } from 'class-variance-authority';
import type * as React from 'react';

import { X } from '~/lib/icons';
import { cn } from '~/lib/utils';

export const Sheet = DialogPrimitive.Root;
export const SheetTrigger = DialogPrimitive.Trigger;

const sheetVariants = cva('fixed z-[var(--z-modal)] flex flex-col bg-background text-foreground duration-200', {
	variants: {
		side: {
			left: 'inset-y-0 left-0 h-full w-72 max-w-[85vw] border-r border-border data-[state=closed]:animate-out data-[state=closed]:slide-out-to-left data-[state=open]:animate-in data-[state=open]:slide-in-from-left',
			right:
				'inset-y-0 right-0 h-full w-80 max-w-[85vw] border-l border-border data-[state=closed]:animate-out data-[state=closed]:slide-out-to-right data-[state=open]:animate-in data-[state=open]:slide-in-from-right',
			bottom:
				'inset-x-0 bottom-0 max-h-[85vh] rounded-t-lg border-t border-border data-[state=closed]:animate-out data-[state=closed]:slide-out-to-bottom data-[state=open]:animate-in data-[state=open]:slide-in-from-bottom',
		},
	},
	defaultVariants: { side: 'right' },
});

export function SheetContent({
	className,
	children,
	side,
	showClose = true,
	...props
}: React.ComponentProps<typeof DialogPrimitive.Content> &
	VariantProps<typeof sheetVariants> & { showClose?: boolean }) {
	return (
		<DialogPrimitive.Portal>
			<DialogPrimitive.Overlay className="fixed inset-0 z-[var(--z-modal-backdrop)] bg-black/45 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0" />
			<DialogPrimitive.Content
				data-slot="sheet-content"
				className={cn(sheetVariants({ side }), 'gap-4 overflow-y-auto p-4', className)}
				{...props}
			>
				{children}
				{showClose ? (
					<DialogPrimitive.Close
						className="absolute top-3.5 right-3.5 tap-target inline-grid size-7 cursor-pointer place-items-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/80 focus-visible:outline-none"
						aria-label="关闭"
					>
						<X className="size-4" aria-hidden="true" />
					</DialogPrimitive.Close>
				) : null}
			</DialogPrimitive.Content>
		</DialogPrimitive.Portal>
	);
}

export function SheetHeader({ className, ...props }: React.ComponentProps<'div'>) {
	return <div data-slot="sheet-header" className={cn('flex flex-col gap-1 pr-8', className)} {...props} />;
}

export function SheetTitle({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Title>) {
	return (
		<DialogPrimitive.Title
			data-slot="sheet-title"
			className={cn('text-[15px] font-semibold tracking-tight', className)}
			{...props}
		/>
	);
}

export function SheetDescription({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Description>) {
	return (
		<DialogPrimitive.Description
			data-slot="sheet-description"
			className={cn('text-[13px] text-muted-foreground', className)}
			{...props}
		/>
	);
}
