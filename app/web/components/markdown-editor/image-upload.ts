import type { Editor } from '@tiptap/core';

import { isSafeSrc } from './safe-url';

interface ImageHandlerOptions {
	editor: () => Editor | null;
	onUpload: (file: File) => Promise<string>;
	onUploadStart?: () => void;
	onUploadEnd?: () => void;
	onError?: (err: Error) => void;
	maxSizeMB?: number;
}

/** Check first bytes (magic numbers) match the declared MIME type. */
function validateImageMagicBytes(file: File): Promise<boolean> {
	return new Promise((resolve) => {
		const reader = new FileReader();
		reader.onload = () => {
			const buf = reader.result as ArrayBuffer;
			if (!buf || buf.byteLength < 4) {
				resolve(false);
				return;
			}
			const view = new Uint8Array(buf);
			const sig = (b0: number, b1: number, b2?: number, b3?: number) => {
				if (b2 !== undefined && b3 !== undefined)
					return view[0] === b0 && view[1] === b1 && view[2] === b2 && view[3] === b3;
				return view[0] === b0 && view[1] === b1;
			};
			// JPEG: FF D8 FF
			if (sig(0xff, 0xd8, 0xff)) {
				resolve(file.type === 'image/jpeg' || file.type === 'image/jpg');
				return;
			}
			// PNG: 89 50 4E 47
			if (sig(0x89, 0x50, 0x4e, 0x47)) {
				resolve(file.type === 'image/png');
				return;
			}
			// GIF: 47 49 46
			if (sig(0x47, 0x49, 0x46)) {
				resolve(file.type === 'image/gif');
				return;
			}
			// WebP: 52 49 46 46 + 57 45 42 50 at offset 8
			if (
				sig(0x52, 0x49, 0x46, 0x46) &&
				view.length >= 12 &&
				view[8] === 0x57 &&
				view[9] === 0x45 &&
				view[10] === 0x42 &&
				view[11] === 0x50
			) {
				resolve(file.type === 'image/webp');
				return;
			}
			// BMP: 42 4D
			if (sig(0x42, 0x4d)) {
				resolve(file.type === 'image/bmp');
				return;
			}
			// SVG doesn't have reliable magic bytes; accept based on MIME only
			if (file.type === 'image/svg+xml') {
				resolve(true);
				return;
			}
			resolve(false);
		};
		reader.onerror = () => resolve(false);
		reader.readAsArrayBuffer(file.slice(0, 16));
	});
}

/** Solid 版 image-upload.ts 平移：魔数校验、5MB 上限、中文报错文案均保持原样。 */
export function createImageHandler(options: ImageHandlerOptions) {
	const { editor, onUpload, onUploadStart, onUploadEnd, onError, maxSizeMB } = options;
	const maxBytes = (maxSizeMB ?? 5) * 1024 * 1024;

	const uploadAndInsert = async (file: File) => {
		if (!file.type.startsWith('image/')) return;
		if (file.size > maxBytes) {
			onError?.(new Error(`图片大小不能超过 ${maxSizeMB ?? 5}MB`));
			return;
		}
		const valid = await validateImageMagicBytes(file);
		if (!valid) {
			onError?.(new Error('图片格式与文件内容不匹配'));
			return;
		}
		onUploadStart?.();
		try {
			const url = await onUpload(file);
			if (!isSafeSrc(url)) throw new Error('Invalid image URL');
			editor()?.chain().focus().setImage({ src: url }).run();
		} catch (err) {
			onError?.(err instanceof Error ? err : new Error('Upload failed'));
		} finally {
			onUploadEnd?.();
		}
	};

	const handlePaste = (e: ClipboardEvent) => {
		const items = e.clipboardData?.items;
		if (!items) return;
		for (const item of Array.from(items)) {
			if (item.type.startsWith('image/')) {
				e.preventDefault();
				const file = item.getAsFile();
				if (file) uploadAndInsert(file);
				return;
			}
		}
	};

	const handleDrop = (e: DragEvent) => {
		const files = e.dataTransfer?.files;
		if (!files?.length) return;
		for (const file of Array.from(files)) {
			if (file.type.startsWith('image/')) {
				e.preventDefault();
				uploadAndInsert(file);
				return;
			}
		}
	};

	const handleFileSelect = (files: FileList) => {
		for (const file of Array.from(files)) {
			if (file.type.startsWith('image/')) {
				uploadAndInsert(file);
				return;
			}
		}
	};

	return { handlePaste, handleDrop, handleFileSelect, uploadAndInsert };
}
