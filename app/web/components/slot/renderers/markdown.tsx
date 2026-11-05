import type { SlotEntry } from '@feedchamber/api-client';

/**
 * 受限 markdown 渲染器。
 *
 * 刻意不引入正文渲染管线（`components/content/markdown-renderer.tsx`）：
 * 那条链路支持图片、代码块、外链卡片等，对插槽这种小面板过重，
 * 且插槽内容来自插件/站长，渲染面积越小越好。这里只做纯文本换行。
 */
export function MarkdownRenderer({ props }: { entry: SlotEntry; props: Record<string, unknown> }) {
	const text = String(props.markdown ?? props.text ?? '');
	if (text === '') return null;

	return <div className="flex flex-col gap-1 px-2 text-[13px] whitespace-pre-wrap text-muted-foreground">{text}</div>;
}
