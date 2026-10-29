'use client';

import Link from 'next/link';
import { parseAsString, useQueryState } from 'nuqs';
import { useCallback, useMemo } from 'react';
import { PostListPage } from '~/components/post/post-list-page';
import {
	feedParamsOf,
	normalizeSortBy,
	type PostFeedPage,
	type SortBy,
	TIMELINE_QUERY_KEYS,
} from '~/components/post/queries';
import { SpaceIcon } from '~/components/space/space-icon';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { Plus } from '~/lib/icons';

/*
 * /spaces/{slug} 单版首页（旧 pages/(app)/spaces/$slug.index.tsx 平移）。
 *
 * 结构照旧：面包屑 → 版面头部卡（图标 / 名称 / 描述）→ 复用 PostListPage。
 *
 * 头部卡右侧原本还有一颗「关注版面」按钮，2026-10-28 按产品决定移除：它与帖子详情的
 * 「关注话题」语义重叠，只保留后者。后端 `target_type: "space"` 同步不再接受（见
 * app/server/internal/model/watch.go），所以这里没有需要清理的取数或状态。
 * 排序胶囊仍走 URL 上的 ?sort_by=（旧版此处只有 sort_by，没有 list_view）。
 */

const SORT_PARAM = parseAsString.withDefault('').withOptions({ history: 'replace', scroll: false });

export interface SpaceHead {
	id: string;
	slug: string;
	name: string;
	icon: string;
	description: string;
}

export interface SpaceTrailItem {
	slug: string;
	name: string;
}

/** 直接子版面入口（父版页面顶部的横向胶囊）。 */
export interface SpaceChildLink {
	slug: string;
	name: string;
	icon?: string;
	description?: string;
}

export interface SpaceHomeProps {
	space: SpaceHead;
	/** 祖先链（不含自己），由 RSC 用 spaceService.breadcrumb 取好 */
	trail: SpaceTrailItem[];
	/** 直接子版面：父版页面列出入口，点进去是自己的聚合页 */
	subSpaces?: SpaceChildLink[];
	initialPage?: PostFeedPage | null;
	initialPageKey?: string | null;
}

export function SpaceHome({ space, trail, subSpaces = [], initialPage, initialPageKey }: SpaceHomeProps) {
	const [rawSortBy, setSortBy] = useQueryState(TIMELINE_QUERY_KEYS.sortBy, SORT_PARAM);
	const sortBy = normalizeSortBy(rawSortBy);
	// 与 RSC 侧同一组参数：按 space_id 递归聚合子版面帖子（父版显示子版内容）。
	const params = useMemo(() => feedParamsOf({ spaceId: space.id, includeSub: true, sortBy }), [space.id, sortBy]);
	const changeSortBy = useCallback((next: SortBy | null) => void setSortBy(next ?? ''), [setSortBy]);

	return (
		<div className="flex w-full flex-col gap-3">
			<nav aria-label="版面路径" className="flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
				<Link href="/spaces" className="transition-colors hover:text-foreground">
					版面
				</Link>
				{trail.map((item) => (
					<span key={item.slug} className="inline-flex items-center gap-1">
						<span aria-hidden="true">/</span>
						<Link href={`/spaces/${encodeURIComponent(item.slug)}`} className="transition-colors hover:text-foreground">
							{item.name}
						</Link>
					</span>
				))}
				<span aria-hidden="true">/</span>
				<span className="text-foreground">{space.name}</span>
			</nav>

			<Card className="gap-0 p-4">
				<div className="flex flex-wrap items-center gap-3">
					<span className="inline-grid size-10 shrink-0 place-items-center rounded-md bg-muted">
						<SpaceIcon icon={space.icon} size={18} className="text-muted-foreground" />
					</span>
					<div className="min-w-0 flex-1">
						<h1 className="truncate text-xl font-semibold tracking-tight text-foreground">{space.name}</h1>
						{space.description ? <p className="mt-0.5 text-[13px] text-muted-foreground">{space.description}</p> : null}
					</div>
					<Button asChild size="sm" className="shrink-0 gap-1.5">
						<Link href={`/spaces/${encodeURIComponent(space.slug)}/post`}>
							<Plus className="size-3.5" aria-hidden="true" />
							发布主题
						</Link>
					</Button>
				</div>
			</Card>

			{subSpaces.length > 0 ? (
				<nav aria-label="子版面" className="flex flex-wrap items-center gap-2">
					<span className="text-xs text-muted-foreground">子版面</span>
					{subSpaces.map((child) => (
						<Link
							key={child.slug}
							href={`/spaces/${encodeURIComponent(child.slug)}`}
							title={child.description || `/${child.slug}`}
							className="inline-flex items-center gap-1.5 rounded-md border border-border px-2.5 py-1 text-xs text-foreground transition-colors hover:bg-muted/60"
						>
							<SpaceIcon icon={child.icon} size={12} className="text-muted-foreground" />
							{child.name}
						</Link>
					))}
				</nav>
			) : null}

			<PostListPage
				params={params}
				sortBy={sortBy}
				onSortByChange={changeSortBy}
				createLink={`/spaces/${encodeURIComponent(space.slug)}/post`}
				initialPage={initialPage}
				initialPageKey={initialPageKey}
				hideHeading
			/>
		</div>
	);
}
