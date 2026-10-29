'use client';

import * as CheckboxPrimitive from '@radix-ui/react-checkbox';
import type * as React from 'react';

import { Check, Minus } from '~/lib/icons';
import { cn } from '~/lib/utils';

/* 复选框：16px 方框，选中填主色；支持 indeterminate（表格全选半选态）。 */
export function Checkbox({ className, ...props }: React.ComponentProps<typeof CheckboxPrimitive.Root>) {
	return (
		<CheckboxPrimitive.Root
			data-slot="checkbox"
			className={cn(
				'size-4 shrink-0 cursor-pointer rounded-sm border border-input bg-background transition-colors outline-none',
				'focus-visible:ring-2 focus-visible:ring-ring/80',
				'data-[state=checked]:border-primary data-[state=checked]:bg-primary data-[state=checked]:text-primary-foreground',
				'data-[state=indeterminate]:border-primary data-[state=indeterminate]:bg-primary data-[state=indeterminate]:text-primary-foreground',
				'disabled:cursor-not-allowed disabled:opacity-50',
				className,
			)}
			{...props}
		>
			<CheckboxPrimitive.Indicator className="flex items-center justify-center text-current">
				{props.checked === 'indeterminate' ? <Minus className="size-3" /> : <Check className="size-3" />}
			</CheckboxPrimitive.Indicator>
		</CheckboxPrimitive.Root>
	);
}
