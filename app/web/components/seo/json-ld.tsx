/*
 * JSON-LD 的唯一出口。
 *
 * app/web/AGENTS.md 规定「不要新增 dangerouslySetInnerHTML 路径」，正文渲染统一走
 * rehype-sanitize。JSON-LD 是**受控例外**：schema.org 要求结构化数据以内联
 * <script type="application/ld+json"> 出现，React 无法用普通 props 表达脚本内容。
 *
 * 受控点：
 *   1. 输入是结构化对象（不是字符串拼接），序列化由 JSON.stringify 完成；
 *   2. 序列化后强制把 "<" 替换为 \u003c，使任何正文都无法提前闭合 </script>；
 *   3. 注入点只有这一个组件，全站不得出现第二处 application/ld+json 的内联脚本。
 *
 * 若将来需要别的内联脚本，先扩展本组件，而不是在调用点再写一次
 * dangerouslySetInnerHTML。
 */

/** Escape "<" so user content can never terminate the surrounding script block. */
export function serializeJsonLd(data: unknown): string {
	return JSON.stringify(data).replace(/</g, '\\u003c');
}

interface JsonLdProps {
	data: Record<string, unknown> | Array<Record<string, unknown>>;
}

export function JsonLd({ data }: JsonLdProps) {
	return (
		// biome-ignore lint/security/noDangerouslySetInnerHtml: 受控例外，见文件头注释
		<script type="application/ld+json" dangerouslySetInnerHTML={{ __html: serializeJsonLd(data) }} />
	);
}
