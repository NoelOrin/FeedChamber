import type { Snippet, SnippetKind } from '@feedchamber/api-client';

/**
 * 自定义片段的落位与容器选择。
 *
 * 这里是全站唯一把站长内容变成 DOM 的地方，因此规则写在一处、由 layout 消费，
 * 不在 layout 里按 kind 临时判断：
 *
 * | kind | 容器 | 为什么 |
 * |------|------|--------|
 * | css  | `<style>` | 内容按 CSS 解析 |
 * | js   | `<script>` | 内容按 JS 解析 |
 * | html | 容器 `<div>` | 内容要按 HTML 解析；塞进 `<script>` 只会变成语法错误 |
 *
 * 服务端写入时已拒绝含 `</style` / `</script` 的内容（`validateSnippetContent`）。
 * 这里再做一次中和，理由不是「不信任服务端」，而是这条链路的失效模式是
 * 全站脚本执行：两处都必须独立成立，任何一处被改坏都不该直接崩塌。
 */

export type HeadSnippet = Exclude<EmittedSnippet, { tag: 'html' }>;

export type EmittedSnippet =
	| { tag: 'style'; id: string; text: string }
	| { tag: 'script'; id: string; text: string }
	| { tag: 'html'; id: string; html: string };

/** 中和会提前闭合容器的结束标签（HTML 解析器只认 `</style` / `</script` 字面量）。 */
export function neutralize(text: string, terminator: 'style' | 'script'): string {
	const pattern = new RegExp(`</${terminator}`, 'gi');
	return text.replace(pattern, `<\\/${terminator}`);
}

/** 一个片段应该落到的位置：html 一律 body_end（与服务端 EffectivePosition 一致）。 */
export function slotOf(snippet: Pick<Snippet, 'kind' | 'position'>): 'head' | 'body_end' {
	if (snippet.kind === 'html') return 'body_end';
	return snippet.position === 'head' ? 'head' : 'body_end';
}

function containerFor(kind: SnippetKind, content: string): EmittedSnippet | null {
	switch (kind) {
		case 'css':
			return { tag: 'style', id: '', text: neutralize(content, 'style') };
		case 'js':
			return { tag: 'script', id: '', text: neutralize(content, 'script') };
		case 'html':
			return { tag: 'html', id: '', html: content };
		default:
			return null;
	}
}

/**
 * 取出某个落位要注入的片段，已按 kind 选好容器并按 sort 排序。
 *
 * 忽略 `enabled`：公开接口返回的就是「生效中」的集合，这里再过滤一次会让
 * 「服务端已启用、前端却没渲染」变成无法定位的差异。需要停用请走接口。
 */
// head 段不可能出现 html（html 一律落 body_end），用重载把这个事实带到类型里，
// 调用方就不用为「不会发生的情况」写分支。
export function emitSnippets(snippets: Snippet[] | undefined, slot: 'head'): HeadSnippet[];
export function emitSnippets(snippets: Snippet[] | undefined, slot: 'body_end'): EmittedSnippet[];
export function emitSnippets(snippets: Snippet[] | undefined, slot: 'head' | 'body_end'): EmittedSnippet[] {
	if (!snippets || snippets.length === 0) return [];
	const out: EmittedSnippet[] = [];
	for (const snippet of snippets) {
		if (slotOf(snippet) !== slot) continue;
		const container = containerFor(snippet.kind, snippet.content);
		if (!container) continue;
		out.push({ ...container, id: snippet.id });
	}
	return out;
}
