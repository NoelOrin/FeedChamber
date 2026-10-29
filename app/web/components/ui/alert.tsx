import { cva, type VariantProps } from 'class-variance-authority';
import type * as React from 'react';

import { cn } from '~/lib/utils';

/* 提示条：中性描边面 + 语义色只出现在图标上；不铺底色。 */
const alertVariants = cva(
	'relative flex items-start gap-2.5 rounded-lg border border-border bg-card px-3.5 py-3 text-[13px] [&_svg]:size-4 [&_svg]:shrink-0',
	{
		variants: {
			variant: {
				neutral: 'text-foreground',
				info: 'text-foreground [&_svg]:text-info',
				success: 'text-foreground [&_svg]:text-success',
				warning: 'text-foreground [&_svg]:text-warning',
				destructive: 'text-foreground [&_svg]:text-destructive',
			},
		},
		defaultVariants: { variant: 'info' },
	},
);

export function Alert({
	className,
	variant,
	...props
}: React.ComponentProps<'div'> & VariantProps<typeof alertVariants>) {
	return <div data-slot="alert" role="alert" className={cn(alertVariants({ variant }), className)} {...props} />;
}

export function AlertTitle({ className, ...props }: React.ComponentProps<'div'>) {
	return <div data-slot="alert-title" className={cn('font-medium', className)} {...props} />;
}

export function AlertDescription({ className, ...props }: React.ComponentProps<'div'>) {
	return (
		<div data-slot="alert-description" className={cn('leading-relaxed text-muted-foreground', className)} {...props} />
	);
}
