import rehypeSanitize from 'rehype-sanitize';
import rehypeStringify from 'rehype-stringify';
import remarkGfm from 'remark-gfm';
import remarkParse from 'remark-parse';
import remarkRehype from 'remark-rehype';
import { unified } from 'unified';

import { markdownSanitizeSchema, sanitizeHtml } from './sanitize';

/**
 * Solid 版 markdown-bridge 平移。差异：html→md（turndown）改为 doc→md（见 docToMd），
 * md→html 管线与防注入（rehype-sanitize + DOMPurify）保持原样。
 */
export async function mdToHtml(md: string): Promise<string> {
	if (!md) return '';
	const file = await unified()
		.use(remarkParse)
		.use(remarkGfm)
		.use(remarkRehype)
		.use(rehypeSanitize, markdownSanitizeSchema)
		.use(rehypeStringify)
		.process(md);
	return sanitizeHtml(String(file));
}

interface DebounceBridge {
	setDirty: () => void;
	flush: () => void;
	destroy: () => void;
}

/**
 * 与旧版同构：编辑器更新 → 防抖 500ms → 回写 markdown。
 * getMarkdown 直接读取 PM 文档树（docToMd），不再经过 HTML。
 */
export function createDebounceBridge(
	getMarkdown: () => string,
	onFlushed: (md: string) => void,
	wait = 500,
): DebounceBridge {
	let dirty = false;
	let timer: ReturnType<typeof setTimeout> | null = null;

	const convert = () => {
		if (!dirty) return;
		dirty = false;
		onFlushed(getMarkdown());
	};

	const setDirty = () => {
		dirty = true;
		if (timer) clearTimeout(timer);
		timer = setTimeout(convert, wait);
	};

	const flush = () => {
		if (timer) clearTimeout(timer);
		timer = null;
		convert();
	};

	const destroy = () => {
		if (timer) clearTimeout(timer);
		timer = null;
	};

	return { setDirty, flush, destroy };
}
