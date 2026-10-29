import type { JSONContent } from '@tiptap/core';

/**
 * ProseMirror doc JSON → Markdown。
 * Solid 版用 turndown（html→md），React 侧无 turndown 依赖，改为直接从 PM 文档树序列化，
 * 分隔符与 turndown 旧配置保持一致：headingStyle atx / codeBlockStyle fenced /
 * bulletListMarker '-' / emDelimiter '*' / strongDelimiter '**'。
 */

type Mark = { type?: string; attrs?: Record<string, unknown> };

const ESCAPABLE = '\\`*[]_<>|^{}'.split('');

function escapeInlineText(text: string): string {
	let out = '';
	for (let i = 0; i < text.length; i++) {
		const ch = text[i];
		const prev = text[i - 1];
		if (ESCAPABLE.includes(ch)) {
			out += `\\${ch}`;
		} else if ((ch === '-' || ch === '+' || ch === '.') && (prev === undefined || prev === '\n')) {
			// 行首的列表/数字序号字符需转义，避免回读时被重新解析
			out += `\\${ch}`;
		} else if (ch === '#' && prev === undefined) {
			out += `\\${ch}`;
		} else {
			out += ch;
		}
	}
	// 行首引用/标题样式的转义在 escapeBlockText 里处理
	return out;
}

function escapeBlockStart(line: string): string {
	return line.replace(/^(#{1,6})(\s)/, '\\$1$2').replace(/^>/, '\\>');
}

function inlineText(node: JSONContent, marks: Mark[]): string {
	if (node.type === 'text') {
		let text = escapeInlineText(node.text ?? '');
		for (const mark of marks) {
			switch (mark.type) {
				case 'bold':
				case 'strong':
					text = `**${text}**`;
					break;
				case 'italic':
				case 'em':
					text = `*${text}*`;
					break;
				case 'strike': {
					const hasTilde = text.includes('~');
					text = hasTilde ? `~ ${text} ~` : `~~${text}~~`;
					break;
				}
				case 'code': {
					// 内容含反引号时用更长的围栏（CommonMark）
					const runs = text.match(/`+/g) ?? [];
					const longest = runs.reduce((m, r) => Math.max(m, r.length), 0);
					const fence = '`'.repeat(longest + 1);
					const pad = text.startsWith('`') || text.endsWith('`') ? ' ' : '';
					text = `${fence}${pad}${text}${pad}${fence}`;
					break;
				}
				case 'underline':
					// Markdown 无下划线语义，与旧 turndown 行为一致：丢弃格式保留文字
					break;
				case 'link': {
					const href = (mark.attrs?.href as string) ?? '';
					text = `[${text}](${href})`;
					break;
				}
				default:
					break;
			}
		}
		return text;
	}

	if (node.type === 'hardBreak') return '  \n';

	if (node.type === 'image') {
		const src = (node.attrs?.src as string) ?? '';
		const alt = (node.attrs?.alt as string) ?? '';
		const title = node.attrs?.title as string | undefined;
		return title ? `![${alt}](${src} "${title}")` : `![${alt}](${src})`;
	}

	// 其余节点（如 inline atom）退化为序列化其子节点
	let result = '';
	for (const child of node.content ?? []) {
		result += inlineText(child, marks);
	}
	return result;
}

function renderInline(node: JSONContent): string {
	return node.content?.map((child) => inlineText(child, [])).join('') ?? '';
}

function renderTextblock(node: JSONContent): string {
	const lines = renderInline(node).split('\n');
	return lines.map(escapeBlockStart).join('\n');
}

interface ListCtx {
	ordered: boolean;
	depth: number;
}

function renderChildren(node: JSONContent, ctx: ListCtx): string {
	const blocks: string[] = [];
	const items = node.content ?? [];
	items.forEach((item, index) => {
		if (item.type === 'listItem' || item.type === 'taskItem') {
			const indent = '  '.repeat(ctx.depth);
			const checked = item.attrs?.checked === true;
			const marker = item.type === 'taskItem' ? (checked ? '- [x] ' : '- [ ] ') : ctx.ordered ? `${index + 1}. ` : '- ';
			const parts: string[] = [];
			for (const child of item.content ?? []) {
				if (child.type === 'bulletList' || child.type === 'orderedList' || child.type === 'taskList') {
					parts.push(renderNode(child, { ordered: child.type !== 'bulletList', depth: ctx.depth + 1 }));
				} else {
					parts.push(renderTextblock(child));
				}
			}
			const joined = parts.join('\n\n');
			const body = parts.length > 1 ? parts[0] : joined;
			const rest = parts.slice(1).join('\n\n');
			const cont = rest ? `\n${rest.replace(/\n/g, `\n${indent}  `)}` : '';
			blocks.push(`${indent}${marker}${body}${cont}`);
		} else {
			blocks.push(renderNode(item, { ...ctx, depth: ctx.depth + 1 }));
		}
	});
	return blocks.join('\n');
}

function renderNode(node: JSONContent, ctx: ListCtx): string {
	switch (node.type) {
		case 'doc':
			return (node.content ?? []).map((child) => renderNode(child, ctx)).join('\n\n');
		case 'paragraph':
			return renderTextblock(node);
		case 'heading': {
			const level = Math.min(Math.max((node.attrs?.level as number) ?? 1, 1), 6);
			return `${'#'.repeat(level)} ${renderInline(node)}`;
		}
		case 'blockquote': {
			const inner = (node.content ?? []).map((child) => renderNode(child, ctx)).join('\n\n');
			return inner
				.split('\n')
				.map((line) => `> ${line}`)
				.join('\n');
		}
		case 'codeBlock': {
			const language = (node.attrs?.language as string) || '';
			const code = node.content?.map((child) => child.text ?? '').join('') ?? '';
			return `\`\`\`${language}\n${code}\n\`\`\``;
		}
		case 'bulletList':
		case 'taskList':
			return renderChildren(node, { ordered: false, depth: ctx.depth });
		case 'orderedList':
			return renderChildren(node, { ordered: true, depth: ctx.depth });
		case 'horizontalRule':
			return '---';
		default:
			return renderTextblock(node);
	}
}

export function docToMd(doc: JSONContent): string {
	if (doc?.type !== 'doc') return '';
	return renderNode(doc, { ordered: false, depth: 0 });
}
