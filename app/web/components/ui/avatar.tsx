'use client';

import * as AvatarPrimitive from '@radix-ui/react-avatar';
import type * as React from 'react';

import { cn } from '~/lib/utils';

export function Avatar({ className, ...props }: React.ComponentProps<typeof AvatarPrimitive.Root>) {
	return (
		<AvatarPrimitive.Root
			data-slot="avatar"
			className={cn('relative inline-grid size-8 shrink-0 place-items-center overflow-hidden rounded-full', className)}
			{...props}
		/>
	);
}

export function AvatarImage({ className, ...props }: React.ComponentProps<typeof AvatarPrimitive.Image>) {
	return (
		<AvatarPrimitive.Image
			data-slot="avatar-image"
			className={cn('aspect-square size-full object-cover', className)}
			{...props}
		/>
	);
}

export function AvatarFallback({ className, ...props }: React.ComponentProps<typeof AvatarPrimitive.Fallback>) {
	return (
		<AvatarPrimitive.Fallback
			data-slot="avatar-fallback"
			className={cn(
				'grid size-full place-items-center rounded-full bg-muted text-[0.72em] font-medium text-muted-foreground uppercase select-none',
				className,
			)}
			{...props}
		/>
	);
}
