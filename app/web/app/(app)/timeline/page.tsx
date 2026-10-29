import type { Metadata } from 'next';

import {
	dehydrateFeedQuery,
	feedParamsOf,
	type ListView,
	normalizeListView,
	normalizeSortBy,
	normalizeTag,
	type PostFeedData,
	postListInfiniteOptions,
	readParam,
	type SortBy,
	stableFeedKey,
	TIMELINE_QUERY_KEYS,
} from '~/components/post/queries';
import { HydrationBoundary } from '~/components/query-hydration';
import { TimelineList } from './timeline-list';
import '~/lib/api/adapter';
import { requireAuthBySetting } from '~/lib/auth/guards';
import { getArticlePageSize } from '~/lib/pagination';
import { loadPublicSettings } from '~/lib/queries/server';
import { getQueryClient } from '~/lib/query/client';
import { listPageMetadataFromSettings } from '~/lib/seo';

export async function generateMetadata(): Promise<Metadata> {
	const settings = await loadPublicSettings();
	/*
	 * `/` 重定向到 `/timeline`，所以本页就是站点的实际首页。
	 * spec 要求「首页 <title> MUST 包含 seo_title」，因此标题取 seo_title 而非写死的
	 * 「时间线」——否则管理员改 seo_title 对首页完全无效。时间线作为列表页的身份
	 * 由 canonical（/timeline）保留。
	 */
	const title = settings.seo_title || '时间线';
	return listPageMetadataFromSettings(settings, { title, path: '/timeline', absoluteTitle: true });
}

/*
 * /timeline —— RSC 侧只负责「首屏那一页」。
 *
 * 参数读取与客户端 nuqs 共用同一组 normalize*（tag / sort_by / list_view），
 * 两端算出的 queryKey 完全一致，水合后不会立刻重发；旧 LinkGroup 生成的
 * `/timeline?list_view=new`、`/timeline?sort_by=hot` 这类链接因此照旧可用。
 * (app)/layout 已经 requireAuth，这里不再重复取 profile。
 */

type SearchParams = Record<string, string | string[] | undefined>;

export default async function TimelinePage({ searchParams }: { searchParams: Promise<SearchParams> }) {
	const raw = await searchParams;

	// 公开内容面的动态登录门：post_auth_required=true 时匿名访客跳登录并带回跳。
	await requireAuthBySetting('post_auth_required', '/timeline');

	const tag = normalizeTag(readParam(raw[TIMELINE_QUERY_KEYS.tag]));
	const sortBy: SortBy | null = normalizeSortBy(readParam(raw[TIMELINE_QUERY_KEYS.sortBy]));
	const listView: ListView | null = normalizeListView(readParam(raw[TIMELINE_QUERY_KEYS.listView]));
	const params = feedParamsOf({ tag, sortBy, listView });

	const pageSize = getArticlePageSize(await loadPublicSettings());
	const options = postListInfiniteOptions(params, pageSize);

	const queryClient = getQueryClient();
	/* prefetchInfiniteQuery 会吞掉错误：预取失败时 seed 为空，列表在客户端退回骨架屏自行取数。 */
	await queryClient.prefetchInfiniteQuery(options);
	const seed = queryClient.getQueryData<PostFeedData>(options.queryKey);

	return (
		<HydrationBoundary state={dehydrateFeedQuery(queryClient, options.queryKey)}>
			<TimelineList initialPage={seed?.pages?.[0] ?? null} initialPageKey={stableFeedKey(params, pageSize)} />
		</HydrationBoundary>
	);
}
