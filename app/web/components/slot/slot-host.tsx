import type { SlotEntry } from '@feedchamber/api-client';
import { useQuery } from '@tanstack/react-query';
import { ListRenderer } from './renderers/list';
import { MarkdownRenderer } from './renderers/markdown';
import { StatsRenderer } from './renderers/stats';
import { entriesForSlot, parseProps } from './slot-logic';

/**
 * 渲染器注册表。键是条目 `props.renderer`（仅 kind=widget 时有意义），
 * 不是 kind —— widget 是「插件/站长提供数据」的通用容器，
 * 具体长什么样由 renderer 决定。
 */
const RENDERERS: Record<string, (p: { entry: SlotEntry; props: Record<string, unknown> }) => React.ReactNode> = {
	list: ListRenderer,
	stats: StatsRenderer,
	markdown: MarkdownRenderer,
};

/** 降级渲染：未注册的 renderer 退回纯文本，而不是整块消失。 */
function fallback({ props }: { entry: SlotEntry; props: Record<string, unknown> }) {
	return <span className="px-2 text-[13px] text-muted-foreground">{String(props.text ?? '')}</span>;
}

/**
 * 插槽宿主：把某插槽的条目渲染成 DOM。
 *
 * 三条刻意的行为（都有对应的纯函数测试语义，见 slot-logic.ts）：
 * - 未知插槽名 → 返回 null，不渲染任何东西；
 * - 无条目 → 返回 null，壳层保持引入插槽前的结构；
 * - 未知 kind / 未注册 renderer → 跳过或降级，不影响同插槽其他条目。
 */
export function SlotHost({ name, entries }: { name: string; entries: SlotEntry[] | undefined }) {
	const items = entriesForSlot(entries, name);
	if (items.length === 0) return null;

	return (
		<div className="flex flex-col gap-1" data-slot={name}>
			{items.map((entry) => {
				const props = parseProps(entry.props);

				if (entry.kind === 'divider') {
					return <hr key={entry.id} className="mx-2 border-border" />;
				}

				if (entry.kind === 'text') {
					return (
						<span key={entry.id} className="px-2 text-[13px] text-muted-foreground">
							{String(props.text ?? '')}
						</span>
					);
				}

				if (entry.kind === 'link') {
					return (
						<a
							key={entry.id}
							href={String(props.href ?? '#')}
							className="inline-flex h-8 items-center rounded-md px-2 text-[13px] text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
						>
							{String(props.label ?? props.text ?? props.href ?? '')}
						</a>
					);
				}

				if (entry.kind === 'image') {
					const src = String(props.src ?? '');
					if (!src) return null;
					// 用原生 img 而非 next/image：src 由站长配置，
					// 不受 next/image 的域名白名单约束。
					return <img key={entry.id} src={src} alt={String(props.alt ?? '')} className="mx-2 rounded-md" />;
				}

				if (entry.kind === 'widget') {
					const key = String(props.renderer ?? 'list');
					const Renderer = RENDERERS[key] ?? fallback;
					return (
						<div key={entry.id}>
							<Renderer entry={entry} props={props} />
						</div>
					);
				}

				// 未知 kind：跳过，不影响其他条目。
				return null;
			})}
		</div>
	);
}

/**
 * 从公开端点取插槽数据。
 *
 * `staleTime` 5 分钟：插槽不是高频变化的数据，且插件写入是异步的
 * （cron.daily 级别），没必要频繁拉。与 `publicCustomizationOptions` 同量级。
 */
export const publicSlotsOptions = {
	queryKey: ['slots', 'public'] as const,
	queryFn: async (): Promise<SlotEntry[]> => {
		const { slotService } = await import('@feedchamber/api-client');
		const data = await slotService.public();
		return data ?? [];
	},
	staleTime: 300_000,
	retry: false,
};

/** 便捷组件：自行取数并渲染。壳层各处直接用这个。 */
export function PublicSlot({ name }: { name: string }) {
	const { data } = useQuery(publicSlotsOptions);
	return <SlotHost name={name} entries={data} />;
}
