import type { PostListItem } from '@feedchamber/api-client';
import { postService, settingService } from '@feedchamber/api-client';
import {
	type DehydratedState,
	dehydrate,
	type InfiniteData,
	infiniteQueryOptions,
	type QueryClient,
	queryOptions,
} from '@tanstack/react-query';

import { FEED_STALE_TIME } from '~/lib/query/client';
import { qk } from '~/lib/query/keys';

/*
 * 信息流取数契约（迁移计划 §3.3 / §3.4）。
 *
 * RSC 预取与客户端 useInfiniteQuery 必须算出**同一个** queryKey，否则水合后首帧立刻重发。
 * 因此筛选参数的白名单校验、参数拼装、分页尺寸来源全部收口在这里，
 * 服务端读 `await searchParams` 与客户端读 nuqs 都走同一组 normalize* 函数。
 * 旧 Solid 侧的 validateSearch 语义：非法值 → 当作缺省（等价「最新」），URL 本身不改写。
 */

export type ListView = 'new' | 'unread' | 'read' | 'posted';
export type SortBy = 'hot' | 'most_commented' | 'active';

const LIST_VIEWS: ListView[] = ['new', 'unread', 'read', 'posted'];
const SORTS: SortBy[] = ['hot', 'most_commented', 'active'];

/**
 * URL 参数名唯一来源：旧 LinkGroup 生成的是 `/timeline?list_view=new`、`/timeline?sort_by=hot`，
 * RSC 读 searchParams 与客户端 nuqs 读写都从这里取，避免两端拼出不同的 key。
 */
export const TIMELINE_QUERY_KEYS = {
	tag: 'tag',
	sortBy: 'sort_by',
	listView: 'list_view',
} as const;

/** 旧 `validateSearch` 的枚举白名单：非法值归零为 null。 */
export function normalizeListView(raw: string | null | undefined): ListView | null {
	const value = raw ?? '';
	return LIST_VIEWS.find((view) => (view as string) === value) ?? null;
}

export function normalizeSortBy(raw: string | null | undefined): SortBy | null {
	const value = raw ?? '';
	return SORTS.find((sort) => (sort as string) === value) ?? null;
}

/** 标签名不做白名单（后端是模糊匹配），但空串要塌成 null，兼容旧 `?tag=` 空值。 */
export function normalizeTag(raw: string | null | undefined): string | null {
	const value = (raw ?? '').trim();
	return value || null;
}

/** App Router 的 searchParams 值可能是数组（重复参数），旧版只取单值，这里取最后一个。 */
export function readParam(raw: string | string[] | null | undefined): string {
	return Array.isArray(raw) ? (raw[raw.length - 1] ?? '') : (raw ?? '');
}

/** 列表筛选参数（page / page_size 由 query 层补）。写成 type alias：qk.posts.list 入参要隐式索引签名。 */
export type PostFeedParams = {
	tag_name?: string;
	/** 版面 ID：与 include_sub 搭配时才生效（后端只对 space_id 递归子版）。 */
	space_id?: string;
	/** 递归包含所有子版面的帖子；父版面首页据此聚合子版内容。 */
	include_sub?: boolean;
	space_slug?: string;
	sort_by?: string;
	list_view?: ListView;
};

export type PostFeedPage = Awaited<ReturnType<typeof postService.list>>;
/** pageParam 保持 unknown：展开 options 传给 useInfiniteQuery 时 TS 不会把字面量 1 推成 number。 */
export type PostFeedData = InfiniteData<PostFeedPage>;

/** null → 字段缺席：两端拼出同一份 key，也让 stableFeedKey 的比对稳定。 */
export function feedParamsOf(input: {
	tag?: string | null;
	/** 版面 ID 与 includeSub 必须成对出现：只传 ID 时后端仍按「仅本版」过滤。 */
	spaceId?: string | null;
	includeSub?: boolean;
	spaceSlug?: string | null;
	sortBy?: SortBy | null;
	listView?: ListView | null;
}): PostFeedParams {
	const params: PostFeedParams = {};
	if (input.tag) params.tag_name = input.tag;
	if (input.spaceId) {
		params.space_id = input.spaceId;
		// 后端 space_id 优先于 space_slug，两者同传时 slug 不参与过滤，故无需再带 slug。
		if (input.includeSub) params.include_sub = true;
	} else if (input.spaceSlug) {
		params.space_slug = input.spaceSlug;
	}
	if (input.sortBy) params.sort_by = input.sortBy;
	if (input.listView) params.list_view = input.listView;
	return params;
}

/** 参数集的确定性指纹：RSC 把它交给客户端，用于判断「首屏那份数据是否属于当前这组参数」。 */
export function stableFeedKey(params: PostFeedParams, pageSize: number): string {
	const merged: Record<string, unknown> = { ...params, page_size: pageSize };
	const keys = Object.keys(merged)
		.filter((key) => merged[key] !== undefined)
		.sort();
	return JSON.stringify(merged, keys);
}

/**
 * 无限列表：`postPaginated` 返回 `{ data, total }`，翻页判定照旧 loader 的
 * `items.length < total`（累计已载入条数对比总数），空页直接收尾避免死循环。
 */
export function postListInfiniteOptions(params: PostFeedParams, pageSize: number) {
	const query = { ...params, page_size: pageSize };
	return infiniteQueryOptions({
		queryKey: qk.posts.list(query),
		queryFn: ({ pageParam }) => postService.list({ ...query, page: pageParam }),
		initialPageParam: 1,
		getNextPageParam: (lastPage, allPages) => {
			const loaded = allPages.reduce((count, page) => count + (page.data?.length ?? 0), 0);
			if ((lastPage.data?.length ?? 0) === 0) return undefined;
			return loaded < (lastPage.total ?? 0) ? allPages.length + 1 : undefined;
		},
		/* 信息流是唯一会被人盯着刷新的区域，走 FEED_STALE_TIME（10s）而非全站 60s。 */
		staleTime: FEED_STALE_TIME,
	});
}

export function postsOf(data: PostFeedData | undefined): PostListItem[] {
	return data?.pages.flatMap((page) => page.data ?? []) ?? [];
}

/** 眉标的「N 篇档案」照旧取 loader 的 total（首页响应的 total 即全集计数）。 */
export function totalOf(data: PostFeedData | undefined): number {
	return data?.pages[0]?.total ?? 0;
}

/** 分页尺寸来自站点设置；根 layout 已把 qk.settings 水合进缓存，这里不会额外发请求。 */
export function publicSettingsOptions() {
	return queryOptions({
		queryKey: qk.settings,
		queryFn: () => settingService.getPublic(),
		staleTime: Infinity,
	});
}

/**
 * 只把本页预取的那条 query 交给 HydrationBoundary。
 * 外壳级数据（settings / spaces tree / tags / profile）根 layout 已经水合过一次，
 * 整份 dehydrate 会把同一批数据再塞进每个页面的 payload。
 */
export function dehydrateFeedQuery(queryClient: QueryClient, queryKey: readonly unknown[]): DehydratedState {
	const wanted = new Set(queryClient.getQueryCache().findAll({ queryKey }));
	return dehydrate(queryClient, {
		shouldDehydrateQuery: (query) => wanted.has(query) && query.state.status === 'success',
	});
}
