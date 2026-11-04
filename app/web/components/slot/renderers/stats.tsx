import type { SlotEntry } from '@feedchamber/api-client';

/** 数字卡片渲染器：props.value + props.label。 */
export function StatsRenderer({ props }: { entry: SlotEntry; props: Record<string, unknown> }) {
	const value = String(props.value ?? '');
	const label = String(props.label ?? '');
	if (value === '') return null;

	return (
		<div className="flex items-baseline gap-1.5 px-2">
			<span className="text-lg font-semibold tabular-nums text-foreground">{value}</span>
			{label ? <span className="text-xs text-muted-foreground">{label}</span> : null}
		</div>
	);
}
