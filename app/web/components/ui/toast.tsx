'use client';

import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';

import { Button } from '~/components/ui/button';
import { CircleAlert, CircleCheck, CircleX, Info, type LucideIcon, X } from '~/lib/icons';
import { type ToastItem, type ToastPosition, toast, useToasts } from '~/lib/toast';
import { cn } from '~/lib/utils';

/* 浮层表面（与 popover 同几何）：描边 + 轻阴影；语义色只出现在图标上。 */
const iconTone = {
	info: 'text-info',
	success: 'text-success',
	warning: 'text-warning',
	error: 'text-destructive',
} as const;

const iconOf: Record<ToastItem['type'], LucideIcon> = {
	info: Info,
	success: CircleCheck,
	warning: CircleAlert,
	error: CircleX,
};

const positionOf: Record<ToastPosition, string> = {
	'top-center': 'top-4 left-1/2 -translate-x-1/2',
	'top-right': 'top-4 right-4',
	'bottom-center': 'bottom-4 left-1/2 -translate-x-1/2',
	'bottom-right': 'bottom-4 right-4',
};

function ToastRow({ item }: { item: ToastItem }) {
	const Icon = iconOf[item.type];
	return (
		<div
			/* 错误条目升级为 alert：外层容器是 polite 的 status 区，失败提示不该等用户空闲才播报 */
			role={item.type === 'error' ? 'alert' : undefined}
			className={cn(
				'flex w-[min(92vw,22rem)] items-center gap-2.5 rounded-lg border border-border bg-popover px-3 py-2.5 shadow-md',
				'data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:slide-in-from-top-2 duration-150',
			)}
		>
			<Icon className={cn('size-4 shrink-0', iconTone[item.type])} aria-hidden="true" />
			<span className="flex-1 text-[13px] leading-snug text-foreground">{item.message}</span>
			<Button
				variant="ghost"
				size="xs"
				onClick={() => toast.remove(item.id)}
				aria-label="关闭"
				className="shrink-0 text-muted-foreground"
			>
				<X className="size-3.5" aria-hidden="true" />
			</Button>
		</div>
	);
}

export function ToastViewport() {
	const { toasts, position } = useToasts();
	const [mounted, setMounted] = useState(false);

	useEffect(() => setMounted(true), []);

	if (!mounted) return null;

	return createPortal(
		<div
			/* toast 是运行时插入的纯 DOM：live region 必须比内容先存在于文档里，
			 所以挂在常驻容器上而不是逐条 toast（空数组时容器仍然渲染）。 */
			role="status"
			aria-live="polite"
			aria-atomic="false"
			className={cn('pointer-events-none fixed z-[var(--z-popover)] flex flex-col gap-2', positionOf[position])}
		>
			{toasts.map((item) => (
				<div key={item.id} className="pointer-events-auto">
					<ToastRow item={item} />
				</div>
			))}
		</div>,
		document.body,
	);
}
