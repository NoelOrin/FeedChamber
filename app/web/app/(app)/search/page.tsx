import { dehydrate } from '@tanstack/react-query';
import type { Metadata } from 'next';
import { normalizePage, searchPostsOptions } from '~/components/search/queries';
import { SearchPanel } from '~/components/search/search-panel';
import { loadPublicSettings } from '~/lib/queries/server';
import { getQueryClient } from '~/lib/query/client';
import { listPageMetadataFromSettings } from '~/lib/seo';

/* 旧 `(app)/search` 的 validateSearch + loader：q/page 来自 URL，空 q 不发请求。 */

/*
 * 搜索结果页不索引：同一批内容在 /timeline 与帖子页已有规范地址，收录搜索页只会
 * 制造无限重复入口（每个关键词一个 URL）。`follow: true` 保留，让爬虫仍能沿结果里的
 * 链接走到帖子本身。
 */
export async function generateMetadata(): Promise<Metadata> {
	const settings = await loadPublicSettings();
	return listPageMetadataFromSettings(settings, { title: '搜索', path: '/search', indexable: false });
}

export default async function SearchPage({
	searchParams,
}: {
	searchParams: Promise<{ q?: string | string[]; page?: string | string[] }>;
}) {
	const params = await searchParams;
	const q = typeof params.q === 'string' ? params.q : '';
	const page = normalizePage(Number(typeof params.page === 'string' ? params.page : ''));

	const queryClient = getQueryClient();
	if (q) await queryClient.prefetchQuery(searchPostsOptions(q, page));

	return (
		<HydrationBoundary state={dehydrate(queryClient)}>
			<SearchPanel />
		</HydrationBoundary>
	);
}

import { HydrationBoundary } from '~/components/query-hydration';
