'use client';

import * as SwitchPrimitive from '@radix-ui/react-switch';
import type * as React from 'react';

import { cn } from '~/lib/utils';

/* 开关：32×18 轨道，选中填主色；与主站控件同高体系。 */
export function Switch({ className, ...props }: React.ComponentProps<typeof SwitchPrimitive.Root>) {
	return (
		<SwitchPrimitive.Root
			data-slot="switch"
			className={cn(
				'peer inline-flex h-[18px] w-8 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors outline-none',
				'focus-visible:ring-2 focus-visible:ring-ring/80',
				'data-[state=checked]:bg-primary data-[state=unchecked]:bg-input',
				'disabled:cursor-not-allowed disabled:opacity-50',
				className,
			)}
			{...props}
		>
			<SwitchPrimitive.Thumb
				data-slot="switch-thumb"
				className={cn(
					'pointer-events-none block size-3.5 rounded-full bg-background shadow-xs ring-0 transition-transform',
					'data-[state=checked]:translate-x-[14px] data-[state=unchecked]:translate-x-0',
				)}
			/>
		</SwitchPrimitive.Root>
	);
}
