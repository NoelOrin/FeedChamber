'use client';

import type * as React from 'react';
import { useCallback, useRef } from 'react';

import { cn } from '~/lib/utils';

/*
 * 分段筛选控件（radigroup 语义）。
 *
 * 为什么不用 Radix Tabs：列表页的「视图 / 排序 / 通知类型」只是筛选条件，
 * 切换后同一个列表重新取数，不存在「每项对应一个面板」。用 Tabs 会让 Radix
 * 生成 aria-controls 指向不存在的 TabsContent（axe 报 critical），
 * 语义上也是错的（tab 暗示有 tabpanel）。
 *
 * 这里用 role="radiogroup" + role="radio"，并补齐 WAI-ARIA 要求的
 * 方向键漫游 + roving tabindex。
 */

export interface SegmentedOption<T extends string> {
	value: T;
	label: React.ReactNode;
}

interface SegmentedProps<T extends string> {
	value: T;
	options: readonly SegmentedOption<T>[];
	onValueChange: (next: T) => void;
	/** 必须提供：分组本身需要一个可访问名称，否则读屏只会播报「一组单选按钮」。 */
	label: string;
	className?: string;
}

export function Segmented<T extends string>({ value, options, onValueChange, label, className }: SegmentedProps<T>) {
	const refs = useRef<Array<HTMLButtonElement | null>>([]);
	const activeIndex = Math.max(
		0,
		options.findIndex((option) => option.value === value),
	);

	const move = useCallback(
		(delta: number) => {
			const next = (activeIndex + delta + options.length) % options.length;
			onValueChange(options[next].value);
			refs.current[next]?.focus();
		},
		[activeIndex, onValueChange, options],
	);

	const onKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
		if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
			event.preventDefault();
			move(1);
		} else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
			event.preventDefault();
			move(-1);
		}
	};

	return (
		<div
			role="radiogroup"
			aria-label={label}
			onKeyDown={onKeyDown}
			className={cn('inline-flex h-8 items-center gap-0.5 rounded-lg bg-muted p-0.5', className)}
		>
			{options.map((option, index) => {
				const checked = option.value === value;
				return (
					<button
						key={option.value}
						ref={(node) => {
							refs.current[index] = node;
						}}
						type="button"
						role="radio"
						aria-checked={checked}
						tabIndex={checked ? 0 : -1}
						onClick={() => onValueChange(option.value)}
						className={cn(
							'inline-flex h-7 cursor-pointer items-center justify-center rounded-md px-2.5 text-xs font-medium whitespace-nowrap transition-colors outline-none',
							'focus-visible:ring-2 focus-visible:ring-ring/80',
							checked ? 'bg-background text-foreground shadow-xs' : 'text-muted-foreground hover:text-foreground',
						)}
					>
						{option.label}
					</button>
				);
			})}
		</div>
	);
}
