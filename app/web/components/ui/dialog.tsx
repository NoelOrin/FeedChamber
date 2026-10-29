'use client';

import * as DialogPrimitive from '@radix-ui/react-dialog';
import type * as React from 'react';

import { X } from '~/lib/icons';
import { cn } from '~/lib/utils';

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogClose = DialogPrimitive.Close;
const DialogPortal = DialogPrimitive.Portal;

function DialogOverlay({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Overlay>) {
	return (
		<DialogPrimitive.Overlay
			data-slot="dialog-overlay"
			className={cn(
				'fixed inset-0 z-[var(--z-modal-backdrop)] bg-black/45',
				'data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0',
				className,
			)}
			{...props}
		/>
	);
}

export function DialogContent({
	className,
	children,
	showClose = true,
	...props
}: React.ComponentProps<typeof DialogPrimitive.Content> & { showClose?: boolean }) {
	return (
		<DialogPortal>
			<DialogOverlay />
			<DialogPrimitive.Content
				data-slot="dialog-content"
				className={cn(
					'fixed top-1/2 left-1/2 z-[var(--z-modal)] grid max-h-[calc(100vh-4rem)] w-[calc(100vw-2rem)] max-w-lg -translate-x-1/2 -translate-y-1/2',
					'gap-4 overflow-y-auto rounded-lg border border-border bg-background p-5 text-foreground shadow-lg',
					'data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95',
					'data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95',
					'duration-150',
					className,
				)}
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
		</DialogPortal>
	);
}

export function DialogHeader({ className, ...props }: React.ComponentProps<'div'>) {
	return <div data-slot="dialog-header" className={cn('flex flex-col gap-1.5 pr-8', className)} {...props} />;
}

export function DialogFooter({ className, ...props }: React.ComponentProps<'div'>) {
	return (
		<div
			data-slot="dialog-footer"
			className={cn('flex flex-col-reverse gap-2 sm:flex-row sm:justify-end', className)}
			{...props}
		/>
	);
}

export function DialogTitle({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Title>) {
	return (
		<DialogPrimitive.Title
			data-slot="dialog-title"
			className={cn('text-[15px] leading-snug font-semibold tracking-tight', className)}
			{...props}
		/>
	);
}

export function DialogDescription({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Description>) {
	return (
		<DialogPrimitive.Description
			data-slot="dialog-description"
			className={cn('text-[13px] leading-relaxed text-muted-foreground', className)}
			{...props}
		/>
	);
}
