'use client';

import type * as React from 'react';
import { useMemo, useState } from 'react';

import { Button } from '~/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover';
import { ChevronDown } from '~/lib/icons';
import { cn } from '~/lib/utils';

/*
 * 树形选择（替代 antd TreeSelect）。
 *
 * 只在「版面管理」的父级选择里用到：需要展示无限嵌套层级并排除自身子树。
 * 面板走 Popover（portal + 统一浮层层级 `--z-popover` + 视口碰撞检测）：
 * 之前是「relative 容器 + absolute 面板 + z-50」，在 modal / drawer 里会被
 * 弹窗的 1050 层级压住、也会被弹窗自身的滚动容器裁掉。
 */

export interface TreeNode {
	value: string;
	title: string;
	children?: TreeNode[];
}

export interface TreeSelectProps {
	value?: string;
	onChange?: (value: string) => void;
	treeData: TreeNode[];
	placeholder?: string;
	className?: string;
	disabled?: boolean;
}

export function TreeSelect({
	value,
	onChange,
	treeData,
	placeholder = '请选择',
	className,
	disabled,
}: TreeSelectProps) {
	const [open, setOpen] = useState(false);

	const selectedLabel = useMemo(() => {
		const find = (nodes: TreeNode[]): string | undefined => {
			for (const n of nodes) {
				if (n.value === value) return n.title;
				if (n.children) {
					const hit = find(n.children);
					if (hit) return hit;
				}
			}
			return undefined;
		};
		return find(treeData);
	}, [treeData, value]);

	function renderNodes(nodes: TreeNode[], depth = 0): React.ReactNode {
		return nodes.map((n) => (
			<div key={n.value}>
				<button
					type="button"
					onClick={() => {
						onChange?.(n.value);
						setOpen(false);
					}}
					className={cn(
						'flex w-full cursor-pointer items-center rounded-sm py-1.5 pr-2 text-left text-[13px] text-foreground hover:bg-muted',
						n.value === value && 'bg-accent text-brand-text',
					)}
					style={{ paddingLeft: 8 + depth * 16 }}
				>
					<span className="truncate">{n.title}</span>
				</button>
				{n.children && n.children.length > 0 ? renderNodes(n.children, depth + 1) : null}
			</div>
		));
	}

	return (
		<Popover open={open} onOpenChange={setOpen}>
			<PopoverTrigger asChild>
				<Button
					type="button"
					variant="outline"
					size="md"
					disabled={disabled}
					className={cn('w-full justify-between font-normal', className)}
				>
					<span className={cn('truncate', !selectedLabel && 'text-muted-foreground')}>
						{selectedLabel || placeholder}
					</span>
					<ChevronDown className="text-muted-foreground size-4 shrink-0" />
				</Button>
			</PopoverTrigger>
			<PopoverContent
				align="start"
				sideOffset={4}
				className="max-h-72 w-[var(--radix-popover-trigger-width)] overflow-y-auto p-1"
			>
				{renderNodes(treeData)}
			</PopoverContent>
		</Popover>
	);
}
