import type { SpaceNode } from '@feedchamber/api-client';
import Link from 'next/link';
import { SpaceIcon } from '~/components/space/space-icon';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { ChevronRight } from '~/lib/icons';
import { cn } from '~/lib/utils';

export function spaceHref(slug: string) {
	return `/spaces/${encodeURIComponent(slug)}`;
}

/** 深度优先摊平（旧 flatten）：版面总数计数与按 slug 反查都靠它。 */
export function flattenSpaceTree(nodes: SpaceNode[]): SpaceNode[] {
	const out: SpaceNode[] = [];
	const walk = (list: SpaceNode[]) => {
		for (const node of list) {
			out.push(node);
			const children = node.children ?? [];
			if (children.length > 0) walk(children);
		}
	};
	walk(nodes);
	return out;
}

/** 大区块：标题行 + 子版面列表（一整张描边卡，行间用 divide-y）。 */
export function SpaceRegion({ node, className }: { node: SpaceNode; order?: number; className?: string }) {
	const children = node.children ?? [];

	return (
		<Card className={cn('gap-0 overflow-hidden p-0', className)}>
			<div className="flex items-center gap-3 px-4 py-3">
				<span className="inline-grid size-8 shrink-0 place-items-center rounded-md bg-muted">
					<SpaceIcon icon={node.icon} size={15} className="text-muted-foreground" />
				</span>
				<div className="min-w-0 flex-1">
					<h2 className="truncate text-[15px] font-semibold tracking-tight text-foreground">{node.name}</h2>
					<p className="truncate text-xs text-muted-foreground">{node.description || `/${node.slug}`}</p>
				</div>
				<Button asChild variant="outline" size="sm" className="shrink-0">
					<Link href={spaceHref(node.slug)}>进入</Link>
				</Button>
			</div>

			{children.length > 0 ? (
				<ul className="m-0 list-none divide-y divide-border/60 border-t border-border p-0">
					{children.map((child) => (
						<li key={child.id}>
							<Link
								href={spaceHref(child.slug)}
								className="group flex items-center gap-3 px-4 py-2.5 transition-colors hover:bg-muted/50"
							>
								<span className="inline-grid size-7 shrink-0 place-items-center rounded-md bg-muted">
									<SpaceIcon icon={child.icon} size={13} className="text-muted-foreground" />
								</span>
								<span className="min-w-0 flex-1">
									<span className="block truncate text-[13px] font-medium text-foreground">{child.name}</span>
									<span className="block truncate text-[11px] text-muted-foreground">
										{child.description || `/${child.slug}`}
									</span>
								</span>
								<ChevronRight
									size={14}
									aria-hidden="true"
									className="shrink-0 text-muted-foreground transition-colors group-hover:text-foreground"
								/>
							</Link>
						</li>
					))}
				</ul>
			) : null}
		</Card>
	);
}
