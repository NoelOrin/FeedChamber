'use client';

import type * as React from 'react';
import { createContext, useContext, useState } from 'react';

import { ChevronDown } from '~/lib/icons';
import { cn } from '~/lib/utils';

/*
 * 折叠面板（原生实现，不引 radix accordion）。
 *
 * 需要的能力只有「分组可折叠」，radix Accordion 会额外带来键盘漫游与
 * roving tabindex 的复杂度，而设置页的分组不需要那套交互。
 * 这里用 button + aria-expanded + aria-controls 保证可访问性。
 */

interface RootCtx {
	open: string[];
	toggle: (value: string) => void;
}
const RootCtx = createContext<RootCtx | null>(null);

interface ItemCtx {
	value: string;
	contentId: string;
	triggerId: string;
}
const ItemCtx = createContext<ItemCtx | null>(null);

function useRoot(): RootCtx {
	const ctx = useContext(RootCtx);
	if (!ctx) throw new Error('Collapse 子组件必须在 <Collapse> 内使用');
	return ctx;
}

function useItem(): ItemCtx {
	const ctx = useContext(ItemCtx);
	if (!ctx) throw new Error('CollapseTrigger / CollapseContent 必须在 <CollapseItem> 内使用');
	return ctx;
}

export function Collapse({
	children,
	defaultOpen = [],
	className,
}: {
	children: React.ReactNode;
	defaultOpen?: string[] | boolean;
	className?: string;
}) {
	const [open, setOpen] = useState<string[]>(Array.isArray(defaultOpen) ? defaultOpen : []);
	const toggle = (v: string) => setOpen((prev) => (prev.includes(v) ? prev.filter((x) => x !== v) : [...prev, v]));

	return (
		<RootCtx.Provider value={{ open, toggle }}>
			<div data-slot="collapse" className={cn('flex flex-col gap-2', className)}>
				{children}
			</div>
		</RootCtx.Provider>
	);
}

export function CollapseItem({
	value,
	children,
	className,
}: {
	value: string;
	children: React.ReactNode;
	className?: string;
}) {
	const triggerId = `collapse-trigger-${value}`;
	const contentId = `collapse-content-${value}`;

	return (
		<ItemCtx.Provider value={{ value, triggerId, contentId }}>
			<div data-slot="collapse-item" className={cn('border-border bg-card rounded-lg border', className)}>
				{children}
			</div>
		</ItemCtx.Provider>
	);
}

export function CollapseTrigger({ children, className }: { children: React.ReactNode; className?: string }) {
	const root = useRoot();
	const item = useItem();
	const expanded = root.open.includes(item.value);

	return (
		<button
			type="button"
			id={item.triggerId}
			aria-expanded={expanded}
			aria-controls={item.contentId}
			onClick={() => root.toggle(item.value)}
			className={cn(
				'flex w-full cursor-pointer items-center justify-between gap-2 rounded-lg px-4 py-2.5 text-[13px] font-medium text-foreground',
				className,
			)}
		>
			{children}
			<ChevronDown
				className={cn('text-muted-foreground size-3.5 shrink-0 transition-transform', expanded && 'rotate-180')}
			/>
		</button>
	);
}

export function CollapseContent({ children, className }: { children: React.ReactNode; className?: string }) {
	const root = useRoot();
	const item = useItem();
	if (!root.open.includes(item.value)) return null;

	return (
		<div
			id={item.contentId}
			role="region"
			aria-labelledby={item.triggerId}
			data-slot="collapse-content"
			className={cn('border-border border-t px-4 py-3', className)}
		>
			{children}
		</div>
	);
}
