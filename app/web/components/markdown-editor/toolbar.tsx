'use client';

import type { Editor } from '@tiptap/core';
import { useEffect, useState } from 'react';

import { PopoverTrigger } from '~/components/ui/popover';
import type { LucideIcon } from '~/lib/icons';
import {
	Bold,
	SquareCheck as CheckSquare,
	Code,
	Eye,
	Heading1,
	Heading2,
	Heading3,
	Image,
	Italic,
	Link,
	List,
	ListOrdered,
	PenLine,
	Quote,
	Strikethrough,
	Underline,
} from '~/lib/icons';
import { cn } from '~/lib/utils';

interface ToolbarProps {
	editor: Editor | null;
	disabled?: boolean;
	onImageClick: () => void;
	preview: boolean;
	onPreviewToggle: () => void;
}

type ToolItem = {
	key: string;
	icon: LucideIcon;
	title: string;
	action: () => void;
	isActive: (editor: Editor) => boolean;
	/** 链接按钮同时充当 Radix Popover 的 Trigger/Anchor（气泡贴工具栏弹出） */
	asPopoverTrigger?: boolean;
};

type ToolGroup = {
	items: ToolItem[];
};

/**
 * Solid 版 toolbar.tsx 平移：三组图标顺序与分组 1:1 保留 ——
 * 分组顺序：行内样式（粗体/斜体/下划线/删除线/行内代码/链接/图片）→ 标题 1-3 → 列表与引用，
 * 右侧追加预览切换按钮（旧版 Reply/create 卡头部的切换钮收进组件）。
 */
export function EditorToolbar({ editor, disabled, onImageClick, preview, onPreviewToggle }: ToolbarProps) {
	// 旧版每个按钮订阅 selectionUpdate；这里整条工具栏订阅一次，渲染时逐按钮重算 isActive
	const [, setTick] = useState(0);
	useEffect(() => {
		if (!editor) return;
		const check = () => setTick((t) => t + 1);
		editor.on('selectionUpdate', check);
		return () => {
			editor.off('selectionUpdate', check);
		};
	}, [editor]);

	const groups: ToolGroup[] = [
		{
			items: [
				{
					key: 'bold',
					icon: Bold,
					title: '粗体',
					action: () => editor?.chain().focus().toggleBold().run(),
					isActive: (e) => e.isActive('bold'),
				},
				{
					key: 'italic',
					icon: Italic,
					title: '斜体',
					action: () => editor?.chain().focus().toggleItalic().run(),
					isActive: (e) => e.isActive('italic'),
				},
				{
					key: 'underline',
					icon: Underline,
					title: '下划线',
					action: () => editor?.chain().focus().toggleUnderline().run(),
					isActive: (e) => e.isActive('underline'),
				},
				{
					key: 'strike',
					icon: Strikethrough,
					title: '删除线',
					action: () => editor?.chain().focus().toggleStrike().run(),
					isActive: (e) => e.isActive('strike'),
				},
				{
					key: 'code',
					icon: Code,
					title: '行内代码',
					action: () => editor?.chain().focus().toggleCode().run(),
					isActive: (e) => e.isActive('code'),
				},
				{
					key: 'link',
					icon: Link,
					title: '链接',
					action: () => {},
					isActive: (e) => e.isActive('link'),
					asPopoverTrigger: true,
				},
				{
					key: 'image',
					icon: Image,
					title: '图片',
					action: onImageClick,
					isActive: () => false,
				},
			],
		},
		{
			items: [
				{
					key: 'h1',
					icon: Heading1,
					title: '标题 1',
					action: () => editor?.chain().focus().toggleHeading({ level: 1 }).run(),
					isActive: (e) => e.isActive('heading', { level: 1 }),
				},
				{
					key: 'h2',
					icon: Heading2,
					title: '标题 2',
					action: () => editor?.chain().focus().toggleHeading({ level: 2 }).run(),
					isActive: (e) => e.isActive('heading', { level: 2 }),
				},
				{
					key: 'h3',
					icon: Heading3,
					title: '标题 3',
					action: () => editor?.chain().focus().toggleHeading({ level: 3 }).run(),
					isActive: (e) => e.isActive('heading', { level: 3 }),
				},
			],
		},
		{
			items: [
				{
					key: 'bulletList',
					icon: List,
					title: '无序列表',
					action: () => editor?.chain().focus().toggleBulletList().run(),
					isActive: (e) => e.isActive('bulletList'),
				},
				{
					key: 'orderedList',
					icon: ListOrdered,
					title: '有序列表',
					action: () => editor?.chain().focus().toggleOrderedList().run(),
					isActive: (e) => e.isActive('orderedList'),
				},
				{
					key: 'blockquote',
					icon: Quote,
					title: '引用',
					action: () => editor?.chain().focus().toggleBlockquote().run(),
					isActive: (e) => e.isActive('blockquote'),
				},
				{
					key: 'taskList',
					icon: CheckSquare,
					title: '任务列表',
					action: () => editor?.chain().focus().toggleTaskList().run(),
					isActive: (e) => e.isActive('taskList'),
				},
			],
		},
	];

	const buttonClass = (active: boolean) =>
		cn(
			'inline-grid size-7 cursor-pointer place-items-center rounded-md text-muted-foreground transition-colors disabled:cursor-not-allowed disabled:opacity-40',
			active ? 'bg-background text-foreground shadow-xs' : 'hover:bg-background/70 hover:text-foreground',
		);

	return (
		<div className="flex flex-wrap items-center gap-0.5 border-b border-border bg-muted/40 px-1.5 py-1">
			{groups.map((group, i) => (
				<div key={group.items[0]?.key} className="contents">
					{i > 0 && <div aria-hidden="true" className="mx-1 h-4 w-px bg-border" />}
					{group.items.map((item) => {
						const active = editor ? item.isActive(editor) : false;
						const button = (
							<button
								key={item.key}
								type="button"
								disabled={disabled}
								onClick={item.action}
								className={buttonClass(active)}
								title={item.title}
							>
								<item.icon size={16} />
							</button>
						);
						return item.asPopoverTrigger ? (
							<PopoverTrigger key={item.key} asChild>
								{button}
							</PopoverTrigger>
						) : (
							button
						);
					})}
				</div>
			))}
			<button
				type="button"
				onClick={onPreviewToggle}
				className="ml-auto inline-flex h-7 cursor-pointer items-center gap-1.5 rounded-md px-2 text-xs font-medium text-muted-foreground transition-colors hover:bg-background/70 hover:text-foreground"
			>
				{preview ? (
					<>
						<PenLine size={12} aria-hidden="true" /> 编辑
					</>
				) : (
					<>
						<Eye size={12} aria-hidden="true" /> 预览
					</>
				)}
			</button>
		</div>
	);
}
