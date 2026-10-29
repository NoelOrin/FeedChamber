import { spaceService, tagService } from '@orinforum/api-client';
import { queryOptions } from '@tanstack/react-query';

import { qk } from '~/lib/query/keys';

/**
 * 外壳级（sidebar / nav）查询契约。
 *
 * 这里的两个函数是 RSC 预取与客户端缓存的同一份来源：主线程在 RSC 里
 * `prefetchQuery(spaceTreeOptions())`，组件里 `useQuery(spaceTreeOptions())`，
 * 因此 queryFn 必须保持无参、与服务端预取完全一致的形式——参数不要塞进组件。
 */

/* 旧 root 路由 staleTime 为 10 分钟，平移过来避免每次导航重取外壳数据。 */
const SHELL_STALE_TIME = 10 * 60_000;

/* 写成 type alias 而非 interface：qk.tags.list 的入参需要隐式索引签名。 */
type TagListParams = { page?: number; page_size?: number };

/* 与旧 root loader 的 `tagService.list({ page: 1, page_size: 50 })` 保持一致。 */
const DEFAULT_TAG_LIST_PARAMS: TagListParams = { page: 1, page_size: 50 };

/** 版面树（NGA 嵌套结构 SpaceNode[]，子级在 children 上）。 */
export function spaceTreeOptions() {
	return queryOptions({
		queryKey: qk.spaces.tree,
		queryFn: () => spaceService.tree(),
		staleTime: SHELL_STALE_TIME,
	});
}

/** 标签区列表；分页参数走默认值，两端 key 才能命中同一份缓存。 */
export function tagListOptions(params: TagListParams = DEFAULT_TAG_LIST_PARAMS) {
	return queryOptions({
		queryKey: qk.tags.list(params),
		queryFn: () => tagService.list(params),
		staleTime: SHELL_STALE_TIME,
	});
}
