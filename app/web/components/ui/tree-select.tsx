'use client';

import type * as React from 'react';
import { useMemo, useState } from 'react';

import { Button } from '~/components/ui/button';
import { ChevronDown } from '~/lib/icons';
import { cn } from '~/lib/utils';

/*
 * 树形选择（替代 antd TreeSelect）。
 *
 * 只在「版面管理」的父级选择里用到：需要展示无限嵌套层级并排除自身子树。
 * 不引 radix 的下拉 + 递归菜单：这里的交互只是「点开、看到缩进层级、选一个」，
 * 用 button + 绝对定位面板 + 递归渲染就够，也避免再引一个组件依赖。
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
		<div className={cn('relative', className)}>
			<Button
				type="button"
				variant="outline"
				size="md"
				disabled={disabled}
				onClick={() => setOpen((v) => !v)}
				className="w-full justify-between font-normal"
			>
				<span className={cn('truncate', !selectedLabel && 'text-muted-foreground')}>
					{selectedLabel || placeholder}
				</span>
				<ChevronDown className="text-muted-foreground size-4 shrink-0" />
			</Button>
			{open ? (
				<>
					<div aria-hidden="true" className="fixed inset-0 z-40" onClick={() => setOpen(false)} />
					<div className="border-border bg-popover absolute z-50 mt-1 max-h-72 w-full overflow-y-auto rounded-md border p-1 shadow-md">
						{renderNodes(treeData)}
					</div>
				</>
			) : null}
		</div>
	);
}
