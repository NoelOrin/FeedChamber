import type * as React from 'react';

import { cn } from '~/lib/utils';

export function Textarea({ className, ...props }: React.ComponentProps<'textarea'>) {
	return (
		<textarea
			data-slot="textarea"
			className={cn(
				'field-sizing-content flex min-h-20 w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm leading-relaxed text-foreground shadow-xs transition-[color,box-shadow] outline-none',
				'placeholder:text-muted-foreground',
				'focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/80',
				'disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50',
				'aria-[invalid=true]:border-destructive',
				className,
			)}
			{...props}
		/>
	);
}
