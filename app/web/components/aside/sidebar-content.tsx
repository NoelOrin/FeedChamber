'use client';

import type { SpaceNode } from '@feedchamber/api-client';
import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { usePathname } from 'next/navigation';

import { MotionPanel } from '~/components/motion/motion-panel';
import { SpaceIcon } from '~/components/space/space-icon';
import { publicCustomizationOptions } from '~/lib/customization/client';
import { ChevronLeft } from '~/lib/icons';
import { spaceTreeOptions, tagListOptions } from '~/lib/queries/shell';
import { cn } from '~/lib/utils';

const skeletonIds = ['space-skeleton-1', 'space-skeleton-2', 'space-skeleton-3'];

function spaceHref(slug: string) {
	return `/spaces/${encodeURIComponent(slug)}`;
}

function countSpaces(nodes: SpaceNode[]): number {
	return nodes.reduce((total, node) => total + 1 + countSpaces(node.children ?? []), 0);
}

/**
 * CustomNav 渲染站长在「站点自定义 → 导航」里配置的入口。
 *
 * 与首页的快捷入口共用同一份 `publicCustomizationOptions`（同一 queryKey），
 * 因此后台改完配置只需失效一个 key，前台所有入口一起更新。
 *
 * **无配置时返回 null**：导航配置为空表示「用内置默认」，不是「显示空导航」
 * （见 site-customization 规范「无配置时回退默认」）。
 */
function CustomNav() {
	const { data } = useQuery(publicCustomizationOptions);
	const items = data?.nav ?? [];
	if (items.length === 0) return null;

	return (
		<nav aria-label="自定义导航" className="flex flex-col gap-1">
			<div className="px-2 pb-1">
				<span className="text-xs font-medium text-muted-foreground">导航</span>
			</div>
			<div className="flex flex-col gap-0.5">
				{items.map((item) => (
					<Link
						key={item.id}
						href={item.href}
						className="inline-flex h-8 min-w-0 items-center gap-1.5 rounded-md px-2 text-[13px] text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
					>
						<span className="truncate">{item.label}</span>
					</Link>
				))}
			</div>
		</nav>
	);
}

function BackToPrev() {
	const pathname = usePathname();
	if (!pathname.includes('post')) return null;

	return (
		<button
			type="button"
			onClick={() => window.history.back()}
			className="inline-flex h-8 w-full cursor-pointer items-center gap-1.5 rounded-md px-2 text-[13px] text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
		>
			<ChevronLeft size={14} aria-hidden="true" />
			返回上一页
		</button>
	);
}

function PanelHeader({ href, title, count }: { href: string; title: string; count: string }) {
	return (
		<div className="flex items-center justify-between px-2 pb-1">
			<Link href={href} className="text-xs font-medium text-muted-foreground transition-colors hover:text-foreground">
				{title}
			</Link>
			<span className="text-[11px] text-muted-foreground tabular-nums">{count}</span>
		</div>
	);
}

function EmptyIndex({ label }: { label: string }) {
	return <div className="px-2 py-1.5 text-xs text-muted-foreground">{label}</div>;
}

function SpaceBranch({ nodes, pathname, depth = 0 }: { nodes: SpaceNode[]; pathname: string; depth?: number }) {
	return (
		<ul className="flex flex-col gap-0.5">
			{nodes.map((node) => {
				const href = spaceHref(node.slug);
				const active = pathname === href || pathname.startsWith(`${href}/`);
				const children = node.children ?? [];

				return (
					<li key={node.id}>
						<Link
							href={href}
							title={node.name}
							className={cn(
								'group flex h-8 items-center gap-2 rounded-md px-2 text-[13px] transition-colors',
								active
									? 'bg-muted font-medium text-foreground'
									: 'text-muted-foreground hover:bg-muted hover:text-foreground',
							)}
							style={{ paddingLeft: depth > 0 ? 8 + depth * 12 : undefined }}
						>
							<SpaceIcon icon={node.icon} size={13} className="shrink-0 text-muted-foreground" />
							<span className="truncate">{node.name}</span>
						</Link>

						{children.length > 0 ? <SpaceBranch nodes={children} pathname={pathname} depth={depth + 1} /> : null}
					</li>
				);
			})}
		</ul>
	);
}

export function SidebarContent() {
	const pathname = usePathname();
	const spacesQuery = useQuery(spaceTreeOptions());
	const tagsQuery = useQuery(tagListOptions());

	const spaces = spacesQuery.data ?? [];
	const tags = tagsQuery.data?.data ?? [];
	const spacesPending = spacesQuery.isPending && spaces.length === 0;
	const tagsPending = tagsQuery.isPending && tags.length === 0;

	return (
		<div className="sticky top-20 flex min-w-0 flex-col gap-5">
			<BackToPrev />

			{/* 站长配置的自定义导航（P1）。无配置时组件返回 null，侧栏保持既有结构。 */}
			<CustomNav />

			<nav aria-label="版面导航" className="flex flex-col gap-1">
				<PanelHeader href="/spaces" title="版面" count={String(countSpaces(spaces))} />
				<div className="min-w-0">
					{spacesPending ? (
						<div className="flex flex-col gap-1.5 px-2">
							{skeletonIds.map((id) => (
								<div key={id} className="h-6 w-full animate-pulse rounded-md bg-muted" />
							))}
						</div>
					) : spaces.length > 0 ? (
						<SpaceBranch nodes={spaces} pathname={pathname} />
					) : (
						<EmptyIndex label="暂无版面" />
					)}
				</div>
			</nav>

			<MotionPanel />

			<nav aria-label="标签导航" className="flex flex-col gap-1">
				<PanelHeader href="/timeline/tag" title="标签" count={String(tags.length)} />
				<div className="flex flex-wrap gap-1.5 px-2">
					{tagsPending ? (
						skeletonIds.map((id) => <div key={id} className="h-6 w-14 animate-pulse rounded-md bg-muted" />)
					) : tags.length > 0 ? (
						tags.map((tag) => (
							<Link
								key={tag.id}
								href={`/timeline?tag=${encodeURIComponent(tag.name)}`}
								className="inline-flex h-6 items-center rounded-md border border-border px-1.5 text-[11px] text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
							>
								{tag.name}
							</Link>
						))
					) : (
						<EmptyIndex label="暂无标签" />
					)}
				</div>
			</nav>
		</div>
	);
}
