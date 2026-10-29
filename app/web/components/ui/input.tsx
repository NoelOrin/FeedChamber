import type * as React from 'react';

import { cn } from '~/lib/utils';

export function Input({ className, type, ...props }: React.ComponentProps<'input'>) {
	return (
		<input
			type={type}
			data-slot="input"
			className={cn(
				'flex h-9 w-full min-w-0 rounded-md border border-input bg-transparent px-3 py-1 text-sm text-foreground shadow-xs transition-[color,box-shadow] outline-none',
				'placeholder:text-muted-foreground',
				'focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/80',
				'disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50',
				'file:mr-2 file:border-0 file:bg-transparent file:text-sm file:font-medium',
				'aria-[invalid=true]:border-destructive aria-[invalid=true]:ring-destructive/20',
				className,
			)}
			{...props}
		/>
	);
}
