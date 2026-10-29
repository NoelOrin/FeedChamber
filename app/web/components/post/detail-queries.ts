import { contentService, postService } from '@orinforum/api-client';
import { queryOptions } from '@tanstack/react-query';

import { qk } from '~/lib/query/keys';

/**
 * 帖子详情这条垂直切片的查询契约（迁移计划 §3.3）：
 * RSC 用 `prefetchQuery(postDetailOptions(id))` 预取，客户端 `useQuery` 复用同一份
 * key/queryFn，两端必须从这里取，否则缓存对不上、首屏会二次请求。
 */

/** 评论分页参数：key 与请求体共用同一个对象，避免两端拼出不同的 key。 */
function contentListParams(page: number, pageSize: number) {
	return { page, page_size: pageSize };
}

export function postDetailOptions(id: string) {
	return queryOptions({
		queryKey: qk.posts.detail(id),
		queryFn: () => postService.getDetail(id),
	});
}

/** 帖子下的评论（后端返回任意深度的平铺列表，层级在前端按 parent_id 组树）。 */
export function postContentsOptions(id: string, page: number, pageSize: number) {
	const params = contentListParams(page, pageSize);
	return queryOptions({
		queryKey: qk.contents.list(id, params),
		queryFn: () => contentService.list(id, params),
	});
}

/** 相关主题（旧 RelatedTopics 的 createResource → useQuery，key 走 qk）。 */
export function postSuggestionsOptions(id: string) {
	return queryOptions({
		queryKey: qk.posts.suggestions(id),
		queryFn: () => postService.suggestions(id),
		enabled: Boolean(id),
	});
}
