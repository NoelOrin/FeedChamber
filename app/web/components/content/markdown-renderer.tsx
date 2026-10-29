'use client';

import DOMPurify from 'isomorphic-dompurify';
import { useMemo } from 'react';
import rehypeSanitize, { defaultSchema, type Options } from 'rehype-sanitize';
import rehypeStringify from 'rehype-stringify';
import remarkGfm from 'remark-gfm';
import remarkParse from 'remark-parse';
import remarkRehype from 'remark-rehype';
import { unified } from 'unified';

import { CodeBlock } from '~/components/content/code-block';
import { ImageItem } from '~/components/content/image-item';
import { Prose } from '~/components/ui/prose';
import { cn } from '~/lib/utils';

interface MarkdownRendererProps {
	mdText: string;
	className?: string;
}

/* 只声明这里真正读到的 mdast 字段（与旧 Solid 版 MarkdownRenderer 的 MdNode 同一份形状）。 */
interface MdNode {
	type: string;
	value?: string;
	lang?: string | null;
	meta?: string | null;
	url?: string;
	alt?: string;
	title?: string | null;
	children?: MdNode[];
}

type Segment =
	| { kind: 'html'; html: string }
	| { kind: 'code'; code: string; language: string; filename?: string }
	| { kind: 'image'; src: string; alt: string; title?: string };

/* ── 净化配置（沿用旧 utils/sanitize.ts 的两条防线）───────────────────────
   旧版：remark AST → JSX，只对 mdast 的 html 节点跑 DOMPurify 再 innerHTML。
   新版：整棵树走 rehype-sanitize（defaultSchema + 同一份协议白名单），
   原始 html 节点仍交 DOMPurify（ALLOWED_URI_REGEXP + 解码后二次拦截
   javascript:/vbscript:/data:/blob:），两份净化结果拼进同一个注入点——
   正文的 HTML 出口始终只有这里的一处 dangerouslySetInnerHTML。 */

const SAFE_URI_RE = /^(?:(?:https?|mailto):|\/(?!\/)|[?#]|\.\.?\/|[^:/?#\\]+(?:[/?#]|$))/i;
const DANGEROUS_SCHEME_RE = /^\s*(?:javascript|vbscript|data|blob):/i;
const RAW_TOKEN_PREFIX = '@@ORINFORUM_RAW_HTML_';

function decodeUriForCheck(uri: string): string {
	try {
		return decodeURIComponent(uri);
	} catch {
		return uri;
	}
}

let purifyHookReady = false;
function ensurePurifyHook() {
	if (purifyHookReady) return;
	purifyHookReady = true;
	DOMPurify.addHook('afterSanitizeAttributes', (node) => {
		// 先解码再判，堵住 %6a%61%76%61%73%63%72%69%70%74: 这类百分号混淆
		for (const attr of ['href', 'src', 'xlink:href']) {
			const val = node.getAttribute(attr);
			if (val && DANGEROUS_SCHEME_RE.test(decodeUriForCheck(val))) node.removeAttribute(attr);
		}
	});
}

function sanitizeRawHtml(html: string): string {
	ensurePurifyHook();
	try {
		return DOMPurify.sanitize(html, { ALLOWED_URI_REGEXP: SAFE_URI_RE, FORCE_BODY: true, RETURN_DOM: false }) as string;
	} catch {
		// jsdom 缺失等异常情形宁可丢弃这段原始 HTML，也不裸注入
		return '';
	}
}

const sanitizeSchema: Options = {
	...defaultSchema,
	protocols: {
		...defaultSchema.protocols,
		href: ['http', 'https', 'mailto'],
		src: ['http', 'https'],
	},
	tagNames: [...(defaultSchema.tagNames ?? []), 'del', 'ins', 'input', 'figure', 'figcaption'],
	attributes: {
		...defaultSchema.attributes,
		code: [...(defaultSchema.attributes?.code ?? []), ['className', /^language-./] as [string, RegExp]],
		input: [...(defaultSchema.attributes?.input ?? []), 'type', 'checked', 'disabled'],
	},
};

/* 旧 JSX 渲染器给链接自己挂了 rel="nofollow noopener" target="_blank"。走字符串管线后
   用一个 sanitize 之后的 hast 插件补回来：属性值由代码写死，不是用户输入。 */
function rehypeLinkTarget() {
	return (tree: unknown) => {
		walkHast(tree, (node) => {
			if (node.type !== 'element') return;
			const element = node as { tagName?: string; properties?: Record<string, unknown> };
			const props: Record<string, unknown> = element.properties ?? {};
			element.properties = props;
			if (element.tagName === 'a') {
				props.target = '_blank';
				props.rel = ['nofollow', 'noopener', 'noreferrer'];
			} else if (element.tagName === 'img') {
				props.loading = 'lazy';
			}
		});
	};
}

function walkHast(node: unknown, visit: (n: Record<string, unknown>) => void) {
	if (!node || typeof node !== 'object') return;
	visit(node as Record<string, unknown>);
	const children = (node as { children?: unknown[] }).children;
	if (Array.isArray(children)) {
		for (const child of children) walkHast(child, visit);
	}
}

const processor = unified()
	.use(remarkParse)
	.use(remarkGfm)
	.use(remarkRehype, { allowDangerousHtml: true })
	.use(rehypeSanitize, sanitizeSchema)
	.use(rehypeLinkTarget)
	.use(rehypeStringify);

/** 把 mdast 的原始 html 节点换成占位 text 节点，字符串化后再回填净化结果。 */
function cloneReplacingHtml(node: MdNode, bucket: string[]): MdNode {
	if (node.type === 'html') {
		bucket.push(sanitizeRawHtml(node.value ?? ''));
		return { type: 'text', value: `${RAW_TOKEN_PREFIX}${bucket.length - 1}@@` };
	}
	if (!node.children) return node;
	return { ...node, children: node.children.map((child) => cloneReplacingHtml(child, bucket)) };
}

function chunkToHtml(chunk: MdNode[]): string {
	if (chunk.length === 0) return '';
	const bucket: string[] = [];
	const root: MdNode = { type: 'root', children: chunk.map((node) => cloneReplacingHtml(node, bucket)) };
	let html = String(processor.stringify(processor.runSync(root as unknown as Parameters<typeof processor.runSync>[0])));
	// 倒序回填，保证占位符里出现的数字不会被再次当成索引
	for (let i = bucket.length - 1; i >= 0; i--) html = html.split(`${RAW_TOKEN_PREFIX}${i}@@`).join(bucket[i] ?? '');
	return html;
}

const SCHEME_RE = /^[a-z][a-z0-9+.-]*:/i;

/** 独占一行的图片走 ImageItem（§4.7 居中 / max-width / 圆角），其余仍是管线里的 <img>。 */
function standaloneImage(node: MdNode): MdNode | null {
	if (node.type !== 'paragraph') return null;
	const kids = node.children ?? [];
	if (kids.length !== 1 || kids[0].type !== 'image') return null;
	return kids[0];
}

/** 图片 URL 守卫与旧 utils/safe-url.ts 的 SRC_PROTOCOLS 一致：只放行 http/https 与相对路径。 */
function hasControlChar(value: string): boolean {
	for (const ch of value) {
		const code = ch.codePointAt(0) ?? 0;
		if (code <= 0x1f) return true;
	}
	return false;
}

function isSafeImageSrc(url: string): boolean {
	const trimmed = url.trim();
	// 与旧版同一道闸：串里还有空白/控制字符就判危险（旧 /[ -\s]+/）
	if (!trimmed || /\s/.test(trimmed) || hasControlChar(trimmed)) return false;
	let decoded: string;
	try {
		decoded = decodeURIComponent(trimmed);
	} catch {
		return false;
	}
	if (hasControlChar(decoded)) return false;
	if (!SCHEME_RE.test(decoded)) return !decoded.startsWith('//') && !decoded.startsWith('\\');
	if (!/^https?:/i.test(decoded)) return false;
	try {
		return Boolean(new URL(decoded));
	} catch {
		return false;
	}
}

function splitSegments(md: string): Segment[] {
	if (!md?.trim()) return [];
	let children: MdNode[];
	try {
		children = (processor.parse(md) as unknown as MdNode).children ?? [];
	} catch (error) {
		console.error('Markdown parse error:', error);
		return [];
	}

	const segments: Segment[] = [];
	let chunk: MdNode[] = [];
	const flush = () => {
		if (chunk.length === 0) return;
		const html = chunkToHtml(chunk);
		if (html.trim()) segments.push({ kind: 'html', html });
		chunk = [];
	};

	for (const node of children) {
		if (node.type === 'code') {
			flush();
			const meta = node.meta?.trim();
			segments.push({
				kind: 'code',
				code: node.value ?? '',
				language: node.lang?.trim() || 'text',
				filename: meta && /\.[a-z0-9]{1,8}$/i.test(meta) ? meta : undefined,
			});
			continue;
		}
		const image = standaloneImage(node);
		if (image?.url && isSafeImageSrc(image.url)) {
			flush();
			segments.push({ kind: 'image', src: image.url, alt: image.alt ?? '', title: image.title ?? undefined });
			continue;
		}
		chunk.push(node);
	}
	flush();
	return segments;
}

export function MarkdownRenderer({ mdText, className }: MarkdownRendererProps) {
	/* Solid 的 createMemo → useMemo：旧版每次 props.mdText 变化重算 AST，这里同构。
	   分段的 key 用「类型 + 位置」而不是下标：Markdown 段落本身没有 id，位置即身份，
	   且同一份文本重算出的分段序列是确定的。 */
	const segments = useMemo(() => splitSegments(mdText), [mdText]);

	if (segments.length === 0) return null;

	return (
		<Prose className={cn('max-w-none', className)}>
			{segments.map((segment, index) => {
				const key = `seg-${segment.kind}-${index}`;
				if (segment.kind === 'code') {
					return <CodeBlock key={key} code={segment.code} language={segment.language} filename={segment.filename} />;
				}
				if (segment.kind === 'image') {
					return <ImageItem key={key} src={segment.src} alt={segment.alt} title={segment.title} />;
				}
				return (
					// biome-ignore lint/security/noDangerouslySetInnerHtml: 正文唯一 HTML 出口，内容已由 rehype-sanitize + DOMPurify 双重净化
					<div key={key} dangerouslySetInnerHTML={{ __html: segment.html }} />
				);
			})}
		</Prose>
	);
}

export default MarkdownRenderer;
