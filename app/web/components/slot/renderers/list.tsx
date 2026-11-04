import type { SlotEntry } from '@feedchamber/api-client';

/** 列表渲染器：props.items 是字符串数组。 */
export function ListRenderer({ entry, props }: { entry: SlotEntry; props: Record<string, unknown> }) {
	const items = Array.isArray(props.items) ? (props.items as unknown[]).map(String) : [];
	if (items.length === 0) return null;

	return (
		<ul className="flex flex-col gap-1 px-2 text-[13px] text-muted-foreground">
			{items.map((item) => (
				// 条目内容可能重复，用 id + 内容做 key；仍重复时 React 只告警不报错。
				<li key={`${entry.id}-${item}`} className="truncate">
					{item}
				</li>
			))}
		</ul>
	);
}
