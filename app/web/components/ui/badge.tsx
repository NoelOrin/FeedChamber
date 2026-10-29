import { cva, type VariantProps } from 'class-variance-authority';
import type * as React from 'react';

import { cn } from '~/lib/utils';

/* 徽标：h-5 / 6px 圆角 / 11px，默认无描边；只有 outline 用描边。 */
const badgeVariants = cva(
	'inline-flex shrink-0 items-center justify-center gap-1 rounded-md border border-transparent px-1.5 py-0.5 text-[11px] leading-4 font-medium whitespace-nowrap [&_svg]:size-3 [&_svg]:shrink-0',
	{
		variants: {
			variant: {
				default: 'bg-secondary text-secondary-foreground',
				secondary: 'bg-secondary text-secondary-foreground',
				primary: 'bg-primary/10 text-brand-text',
				brand: 'bg-primary/10 text-brand-text',
				state: 'bg-foreground text-background',
				outline: 'border-border text-muted-foreground',
				id: 'bg-muted text-muted-foreground',
				success: 'bg-success/10 text-success',
				warning: 'bg-warning/10 text-warning',
				info: 'bg-info/10 text-info',
				destructive: 'bg-destructive/10 text-destructive',
			},
		},
		defaultVariants: { variant: 'default' },
	},
);

export function Badge({
	className,
	variant,
	...props
}: React.ComponentProps<'span'> & VariantProps<typeof badgeVariants>) {
	return <span data-slot="badge" className={cn(badgeVariants({ variant }), className)} {...props} />;
}

export { badgeVariants };
