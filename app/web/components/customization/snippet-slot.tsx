import type { EmittedSnippet } from '~/lib/customization/emit';

/**
 * 把一条自定义片段渲染成对应容器。
 *
 * 这是全站**唯一**允许 dangerouslySetInnerHTML 的地方（除根布局的首绘主题脚本
 * 与 JSON-LD）：容器由 `emitSnippets` 按 kind 决定，这里只负责落 DOM。
 * 内容已经在写入侧校验（`service.validateSnippetContent` 拒绝 `</style` /
 * `</script`），`emit.ts` 再做一次中和——两处都独立成立，任一处被改坏都不至于
 * 直接变成全站脚本执行。
 *
 * 把三条注入收在一个组件里，是为了让 biome 的 suppression 紧贴各自的 JSX，
 * 也让 review 时「哪些地方允许 innerHTML」只查一个文件。
 */
export function SnippetSlot({ snippet }: { snippet: EmittedSnippet }) {
	if (snippet.tag === 'html') {
		return <div data-fc-snippet={snippet.id} dangerouslySetInnerHTML={{ __html: snippet.html }} />;
	}
	if (snippet.tag === 'style') {
		return <style id={`fc-custom-css-${snippet.id}`} dangerouslySetInnerHTML={{ __html: snippet.text }} />;
	}
	return <script id={`fc-custom-js-${snippet.id}`} dangerouslySetInnerHTML={{ __html: snippet.text }} />;
}
