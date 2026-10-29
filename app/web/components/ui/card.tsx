import { cva, type VariantProps } from 'class-variance-authority';
import type * as React from 'react';

import { cn } from '~/lib/utils';

/*
 * 卡片 = 极淡描边 + 10px 圆角 + 一层极轻阴影。
 *
 * 为什么不再只靠描边：整站所有面都用 1px 灰线切分时，页面读起来像表格而不像内容，
 * 层级全靠线宽区分（而线宽又是恒定的 1px），这是「模板感」的主要来源。
 * 现在改为「面靠底色 + 阴影分层，线只画在真正需要边界的控件上」。
 */
const cardVariants = cva(
	'flex flex-col gap-4 rounded-lg border border-border/60 bg-card text-card-foreground shadow-xs',
	{
		variants: {
			variant: {
				default: 'border-border/60 bg-card',
				archive: 'border-border/60 bg-card',
				inset: 'border-transparent bg-muted shadow-none',
			},
			hover: {
				lift: 'transition-shadow hover:shadow-sm',
				static: '',
			},
		},
		defaultVariants: { variant: 'default', hover: 'static' },
	},
);

export function Card({
	className,
	variant,
	hover,
	...props
}: React.ComponentProps<'div'> & VariantProps<typeof cardVariants>) {
	return <div data-slot="card" className={cn(cardVariants({ variant, hover }), className)} {...props} />;
}

export function CardHeader({ className, ...props }: React.ComponentProps<'div'>) {
	return <div data-slot="card-header" className={cn('flex flex-col gap-1', className)} {...props} />;
}

export function CardTitle({ className, ...props }: React.ComponentProps<'div'>) {
	return (
		<div
			data-slot="card-title"
			className={cn('text-[15px] leading-snug font-semibold tracking-tight text-foreground', className)}
			{...props}
		/>
	);
}

export function CardDescription({ className, ...props }: React.ComponentProps<'div'>) {
	return <div data-slot="card-description" className={cn('text-[13px] text-muted-foreground', className)} {...props} />;
}

export function CardContent({ className, ...props }: React.ComponentProps<'div'>) {
	return <div data-slot="card-content" className={cn('flex-1', className)} {...props} />;
}

export function CardFooter({ className, ...props }: React.ComponentProps<'div'>) {
	return <div data-slot="card-footer" className={cn('flex items-center gap-2', className)} {...props} />;
}
