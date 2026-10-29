'use client';

import type { Editor } from '@tiptap/core';
import { useEffect, useRef, useState } from 'react';

import { Button } from '~/components/ui/button';
import { PopoverContent } from '~/components/ui/popover';
import { isSafeHref } from './safe-url';

interface LinkPopoverProps {
	editor: Editor | null;
	onClose: () => void;
}

/**
 * Solid 版 link-popover.tsx 平移为 Radix Popover 版（Trigger = 工具栏链接按钮，Root 由 index 控制）。
 * 校验文案与「移除链接 / 取消 / 确定」保持原样；Content 仅在打开时挂载 —— 等价旧版 <Show when={props.show}>。
 */
export function LinkPopover({ editor, onClose }: LinkPopoverProps) {
	const [url, setUrl] = useState('');
	const [error, setError] = useState('');
	const inputRef = useRef<HTMLInputElement>(null);

	useEffect(() => {
		if (editor) setUrl(editor.getAttributes('link').href || '');
		setError('');
		// 自动聚焦（旧版 50ms setTimeout 的等价时机）
		const timer = setTimeout(() => inputRef.current?.focus(), 50);
		return () => clearTimeout(timer);
	}, [editor]);

	const closeAndFocusEditor = () => {
		onClose();
		editor?.chain().focus().run();
	};

	const setLink = () => {
		const href = url.trim();
		if (href && !isSafeHref(href)) {
			setError('URL 需使用 http、https、mailto 或相对路径');
			return;
		}
		if (!editor) return;
		if (href) {
			editor.chain().focus().extendMarkRange('link').setLink({ href }).run();
		} else {
			editor.chain().focus().extendMarkRange('link').unsetLink().run();
		}
		closeAndFocusEditor();
	};

	const unsetLink = () => {
		editor?.chain().focus().unsetLink().run();
		closeAndFocusEditor();
	};

	const onKeyDown = (e: React.KeyboardEvent) => {
		if (e.key === 'Enter') {
			e.preventDefault();
			setLink();
		}
		if (e.key === 'Escape') {
			closeAndFocusEditor();
		}
	};

	return (
		<PopoverContent
			align="start"
			sideOffset={6}
			className="min-w-[280px]"
			onCloseAutoFocus={(e) => {
				// 焦点回编辑器而不是回 Trigger 按钮
				e.preventDefault();
			}}
		>
			<div className="space-y-2">
				<input
					ref={inputRef}
					type="url"
					value={url}
					onChange={(e) => {
						setUrl(e.currentTarget.value);
						setError('');
					}}
					onKeyDown={onKeyDown}
					placeholder="输入链接 URL..."
					className="w-full border border-border bg-transparent px-2 py-1.5 text-sm outline-none focus:border-foreground"
				/>
				{error && <p className="text-xs text-destructive">{error}</p>}
				<div className="flex items-center justify-between">
					<button type="button" onClick={unsetLink} className="cursor-pointer text-xs text-destructive hover:underline">
						移除链接
					</button>
					<div className="flex items-center gap-2">
						<Button type="button" variant="ghost" size="xs" onClick={onClose}>
							取消
						</Button>
						<Button type="button" variant="primary" size="xs" onClick={setLink}>
							确定
						</Button>
					</div>
				</div>
			</div>
		</PopoverContent>
	);
}
