'use client';

import type * as React from 'react';
import { useState } from 'react';

import { Button } from '~/components/ui/button';
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from '~/components/ui/dialog';
import { cn } from '~/lib/utils';

/*
 * 二次确认（替代 antd Popconfirm）。
 *
 * 为什么用 Dialog 而不是 Popover：后台的确认绝大多数是「删除 / 封禁 / 恢复」
 * 这类不可逆操作，Popover 的锚点定位在表格最后一行会被视口裁掉；而且
 * antd Popconfirm 的默认文案只有一行，危险操作该有标题 + 具体说明。
 *
 * 用法是「受控」的：调用方持有 open 状态，避免在 17 个页面里各写一遍弹层逻辑。
 */

export interface ConfirmDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	title: React.ReactNode;
	description?: React.ReactNode;
	confirmText?: string;
	cancelText?: string;
	/** 危险操作（删除 / 封禁）用 destructive，默认用主色 */
	danger?: boolean;
	/** 返回 Promise 时按钮显示 loading，reject 则保持弹层打开 */
	onConfirm: () => void | Promise<void>;
	className?: string;
}

export function ConfirmDialog({
	open,
	onOpenChange,
	title,
	description,
	confirmText = '确认',
	cancelText = '取消',
	danger = false,
	onConfirm,
	className,
}: ConfirmDialogProps) {
	const [pending, setPending] = useState(false);

	async function handleConfirm() {
		setPending(true);
		try {
			await onConfirm();
			onOpenChange(false);
		} catch {
			// 失败时保持打开，让用户看到页面上的错误提示后重试
		} finally {
			setPending(false);
		}
	}

	return (
		<Dialog open={open} onOpenChange={(next) => !pending && onOpenChange(next)}>
			<DialogContent className={cn('max-w-md', className)}>
				<DialogHeader>
					<DialogTitle>{title}</DialogTitle>
					{description ? <DialogDescription>{description}</DialogDescription> : null}
				</DialogHeader>
				<DialogFooter className="gap-2">
					<Button variant="outline" size="sm" disabled={pending} onClick={() => onOpenChange(false)}>
						{cancelText}
					</Button>
					<Button variant={danger ? 'destructive' : 'default'} size="sm" loading={pending} onClick={handleConfirm}>
						{confirmText}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}

/**
 * 命令式确认：把「一行按钮 + 一个弹层」的常见用法包成 hook。
 *
 * const [confirm, dialog] = useConfirm();
 * <Button onClick={() => confirm({ title: '删除？', onConfirm: doDelete })} />
 * {dialog}
 */
export interface ConfirmOptions {
	title: React.ReactNode;
	description?: React.ReactNode;
	confirmText?: string;
	danger?: boolean;
	onConfirm: () => void | Promise<void>;
}

export function useConfirm() {
	const [state, setState] = useState<(ConfirmOptions & { open: boolean }) | null>(null);

	const confirm = (options: ConfirmOptions) => setState({ ...options, open: true });

	const dialog = state ? (
		<ConfirmDialog
			{...state}
			open={state.open}
			onOpenChange={(open) => setState((prev) => (prev ? { ...prev, open } : prev))}
		/>
	) : null;

	return [confirm, dialog] as const;
}
