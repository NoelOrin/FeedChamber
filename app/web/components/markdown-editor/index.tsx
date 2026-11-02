'use client';

import { uploadService } from '@feedchamber/api-client';
import type { Editor } from '@tiptap/core';
import { EditorContent, useEditor } from '@tiptap/react';
import type { JSX } from 'react';
import { useEffect, useMemo, useRef, useState } from 'react';

import { Popover } from '~/components/ui/popover';
import { Skeleton } from '~/components/ui/skeleton';
import { Layers } from '~/lib/icons';
import { toast } from '~/lib/toast';
import { cn } from '~/lib/utils';
import { clearDraft, draftToMarkdown, loadDraft, saveDraft } from './draft';
import './editor.css';
import { docToMd } from './doc-to-md';
import { getExtensions } from './extensions';
import { createImageHandler } from './image-upload';
import { LinkPopover } from './link-popover';
import { createDebounceBridge, mdToHtml } from './markdown-bridge';
import { EditorToolbar } from './toolbar';

interface MarkdownEditorProps {
	value: string;
	onChange: (markdown: string) => void;
	/** 编辑器内容区的可访问名称（Tiptap 输出的是 contenteditable，没有它读屏只会播报「编辑区」）。 */
	ariaLabel: string;
	placeholder?: string;
	onSubmit?: () => void;
	submitting?: boolean;
	draftKey?: string;
	minHeight?: number;
}

/** 旧版 EditorSkeleton 的等价物（React 侧 ui/skeleton 只提供基础 Skeleton）。 */
function SkeletonRow({ count, className }: { count: number; className: string }) {
	return (
		<>
			{Array.from({ length: count }, (_, i) => (
				// biome-ignore lint/suspicious/noArrayIndexKey: 纯装饰静态占位，数量固定无重排
				<Skeleton key={i} className={className} />
			))}
		</>
	);
}

function EditorSkeletonBody() {
	return (
		<div aria-hidden="true" className="w-full space-y-3 px-3 py-3 text-sm">
			<div className="flex flex-wrap items-center gap-1">
				<SkeletonRow count={9} className="h-7 w-7 rounded" />
				<div className="mx-1 h-5 w-px bg-border" />
				<SkeletonRow count={3} className="h-7 w-7 rounded" />
				<div className="mx-1 h-5 w-px bg-border" />
				<SkeletonRow count={4} className="h-7 w-7 rounded" />
			</div>
			<div className="space-y-3">
				<Skeleton className="h-3 w-full rounded" />
				<Skeleton className="h-3 w-3/4 rounded" />
				<Skeleton className="h-3 w-1/2 rounded" />
			</div>
		</div>
	);
}

/**
 * Solid 版 MarkdownEditor 的 React 平移（Tiptap 官方 useEditor + EditorContent）。
 * 对外契约与迁移计划 §3.2 一致：value 是 Markdown，onChange 防抖 500ms 回写。
 */
export function MarkdownEditor(props: MarkdownEditorProps): JSX.Element {
	const { value, onChange, ariaLabel, placeholder, onSubmit, submitting, draftKey, minHeight } = props;

	// 最新 props 的镜像 ref：编辑器/handler 只建一次，回调经 ref 转发（替代 Solid 的 props 惰性读取）
	const latest = useRef({ value, onChange, ariaLabel, placeholder, onSubmit, draftKey });
	latest.current = { value, onChange, ariaLabel, placeholder, onSubmit, draftKey };

	const internalUpdate = useRef(false);
	const syncVersion = useRef(0);
	const editorRef = useRef<Editor | null>(null);
	const [mounted, setMounted] = useState(false);
	const [uploading, setUploading] = useState(false);
	const [preview, setPreview] = useState(false);
	const [linkOpen, setLinkOpen] = useState(false);

	const bridge = useMemo(
		() =>
			createDebounceBridge(
				() => (editorRef.current ? docToMd(editorRef.current.getJSON()) : ''),
				(md) => {
					latest.current.onChange(md);
					if (latest.current.draftKey) saveDraft(latest.current.draftKey, md);
				},
				500,
			),
		[],
	);

	const editor = useEditor({
		// vinext SSR：编辑器实例延后到客户端挂载再创建（hook 因此返回 Editor | null）
		immediatelyRender: false,
		extensions: getExtensions(() => latest.current.placeholder || 'Write something...'),
		content: '',
		editorProps: {
			attributes: {
				class: 'text-sm leading-[1.75em] text-foreground max-w-none',
				'aria-label': latest.current.ariaLabel,
			},
			handleKeyDown: (_view, event) => {
				if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
					event.preventDefault();
					bridge.flush(); // ⌘+Enter 先回写 markdown，父组件立刻提交不丢最新输入
					latest.current.onSubmit?.();
					return true;
				}
				return false;
			},
		},
		onUpdate: () => {
			internalUpdate.current = true;
			bridge.setDirty();
		},
	});
	editorRef.current = editor;

	// 图片上传（拖拽/粘贴/文件选择共用），接口走 api-client 的 uploadService
	const imageHandler = useMemo(
		() =>
			createImageHandler({
				editor: () => editorRef.current,
				onUpload: async (file: File) => {
					const res = await uploadService.upload(file);
					return res.url;
				},
				onUploadStart: () => setUploading(true),
				onUploadEnd: () => setUploading(false),
				onError: (err) => {
					console.error('Image upload error:', err);
					toast.error(err?.message || 'Image upload failed');
				},
			}),
		[],
	);

	// 挂载：读草稿（沿用旧版 localStorage 键语义）→ markdown→html 初始化内容
	useEffect(() => {
		if (!editor) return;
		let cancelled = false;
		const init = async () => {
			const draft = latest.current.draftKey ? loadDraft<unknown>(latest.current.draftKey) : null;
			const draftMd = draftToMarkdown(draft);
			const initial = draftMd ?? latest.current.value;
			if (draftMd && draftMd !== latest.current.value) {
				latest.current.onChange(draftMd);
			}
			const html = initial ? await mdToHtml(initial) : '';
			if (cancelled) return;
			if (html) editor.commands.setContent(html, { emitUpdate: false });
			setMounted(true);
		};
		init();
		return () => {
			cancelled = true;
		};
	}, [editor]);

	// 外部 value 变化回灌编辑器（表单重置 / 提交后清空）。
	// 沿用旧版：internalUpdate 跳过编辑器自身触发的回写；版本号丢弃过期异步同步。
	useEffect(() => {
		if (!editor || !mounted) return;
		if (internalUpdate.current) {
			internalUpdate.current = false;
			return;
		}
		const version = ++syncVersion.current;
		const doSync = async () => {
			const expectedHtml = value ? await mdToHtml(value) : '';
			if (version !== syncVersion.current) return;
			const currentHtml = editor.getHTML();
			if (currentHtml !== expectedHtml) {
				editor.commands.setContent(expectedHtml, { emitUpdate: false });
				// 父级清空内容（典型：发布成功后重置）→ 同步清掉本地草稿
				if (!value && draftKey) clearDraft(draftKey);
			}
		};
		doSync();
	}, [editor, mounted, value, draftKey]);

	// 粘贴 / 拖拽图片监听（挂到 ProseMirror DOM 上，卸载时移除）
	useEffect(() => {
		if (!editor) return;
		const dom = editor.view.dom;
		dom.addEventListener('paste', imageHandler.handlePaste);
		dom.addEventListener('drop', imageHandler.handleDrop);
		return () => {
			dom.removeEventListener('paste', imageHandler.handlePaste);
			dom.removeEventListener('drop', imageHandler.handleDrop);
		};
	}, [editor, imageHandler]);

	// 预览 / 提交中 → 只读（预览复用同一 ProseMirror 视图，不新增 innerHTML 渲染路径）
	useEffect(() => {
		if (!editor) return;
		editor.setEditable(mounted && !preview && !submitting);
	}, [editor, mounted, preview, submitting]);

	// minHeight（px 数值）应用到编辑区
	useEffect(() => {
		if (!editor) return;
		const dom = editor.view.dom as HTMLElement;
		dom.style.minHeight = minHeight ? `${minHeight}px` : '';
	}, [editor, minHeight]);

	useEffect(
		() => () => {
			bridge.destroy();
		},
		[bridge],
	);

	const handleImageClick = () => {
		const input = document.createElement('input');
		input.type = 'file';
		input.accept = 'image/*';
		input.onchange = () => {
			const files = input.files;
			if (files?.length) imageHandler.handleFileSelect(files);
		};
		input.click();
	};

	return (
		<div className="relative">
			<Popover open={linkOpen} onOpenChange={setLinkOpen}>
				{mounted && (
					<EditorToolbar
						editor={editor}
						disabled={submitting}
						onImageClick={handleImageClick}
						preview={preview}
						onPreviewToggle={() => setPreview((p) => !p)}
					/>
				)}

				<div
					className={cn(
						'w-full rounded-md border border-input bg-background text-sm transition-[color,box-shadow]',
						'focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/80',
						submitting && 'cursor-not-allowed opacity-50',
						!mounted && 'hidden',
						preview && 'prose-mirror-preview',
					)}
				>
					<EditorContent editor={editor} />
				</div>
				{!mounted && <EditorSkeletonBody />}

				{uploading && (
					<div className="absolute inset-0 z-10 flex items-center justify-center rounded-md bg-background/70">
						<span className="text-xs text-muted-foreground">图片上传中…</span>
					</div>
				)}

				<LinkPopover editor={editor} onClose={() => setLinkOpen(false)} />
			</Popover>

			{draftKey && (
				<div className="flex items-center gap-1.5 pt-1 text-[11px] text-muted-foreground">
					<Layers size={11} aria-hidden="true" /> 草稿自动保存 · 支持 Markdown
				</div>
			)}
		</div>
	);
}

export default MarkdownEditor;
