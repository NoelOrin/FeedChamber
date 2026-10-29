import type * as React from 'react';

import { cn } from '~/lib/utils';

export function Label({ className, ...props }: React.ComponentProps<'label'>) {
	return (
		// biome-ignore lint/a11y/noLabelWithoutControl: htmlFor 经 ...props 透传，静态检查看不到
		<label
			data-slot="label"
			className={cn('text-[13px] leading-none font-medium text-foreground peer-disabled:opacity-50', className)}
			{...props}
		/>
	);
}
