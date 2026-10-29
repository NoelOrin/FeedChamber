import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';
import type * as React from 'react';

import { LoaderCircle } from '~/lib/icons';
import { cn } from '~/lib/utils';

/* shadcn 按钮几何：h-9 / rounded-md / 13px。主行动实心主题色，其余靠描边与灰面分层。 */
const buttonVariants = cva(
	'inline-flex shrink-0 cursor-pointer items-center justify-center gap-1.5 whitespace-nowrap rounded-md font-medium transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/80 disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*=size-])]:size-4',
	{
		variants: {
			variant: {
				default: 'bg-primary text-primary-foreground hover:bg-primary/90',
				primary: 'bg-primary text-primary-foreground hover:bg-primary/90',
				secondary: 'bg-secondary text-secondary-foreground hover:bg-secondary/70',
				outline: 'border border-border bg-background text-foreground hover:bg-muted',
				ghost: 'text-foreground hover:bg-muted',
				destructive: 'bg-destructive text-destructive-foreground hover:bg-destructive/90',
				dashed: 'border border-dashed border-border text-foreground hover:bg-muted',
				link: 'text-brand-text underline-offset-4 hover:underline',
			},
			size: {
				xs: 'h-7 px-2 text-xs [&_svg:not([class*=size-])]:size-3',
				sm: 'h-8 px-3 text-xs [&_svg:not([class*=size-])]:size-3.5',
				md: 'h-9 px-4 text-[13px]',
				lg: 'h-10 px-5 text-sm',
				xl: 'h-11 px-6 text-sm font-semibold [&_svg:not([class*=size-])]:size-5',
				icon: 'size-9',
			},
		},
		defaultVariants: { variant: 'default', size: 'md' },
	},
);

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> {
	asChild?: boolean;
	loading?: boolean;
	/** 兼容旧调用的「方形图标键」语义（新语言里不再是正圆） */
	round?: boolean;
}

export function Button({
	className,
	variant,
	size,
	round,
	loading,
	asChild,
	type,
	disabled,
	children,
	...props
}: ButtonProps) {
	const classes = cn(buttonVariants({ variant, size }), round && 'aspect-square px-0', className);
	const isDisabled = disabled || loading;

	if (asChild) {
		return (
			<Slot
				className={classes}
				data-slot="button"
				aria-busy={loading || undefined}
				aria-disabled={isDisabled || undefined}
				{...props}
			>
				{children}
			</Slot>
		);
	}

	return (
		<button
			type={type ?? 'button'}
			aria-busy={loading || undefined}
			disabled={isDisabled}
			className={classes}
			{...props}
		>
			{loading ? <LoaderCircle className="size-3.5 animate-spin" aria-hidden="true" /> : null}
			{children}
		</button>
	);
}
