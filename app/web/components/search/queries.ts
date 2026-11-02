import { postService } from '@feedchamber/api-client';
import { queryOptions } from '@tanstack/react-query';

import { qk } from '~/lib/query/keys';

/* 旧 search loader 的 page_size=20；key 只带 q/page，尺寸变化必须同步这里。 */
export const SEARCH_PAGE_SIZE = 20;

/* 旧 loader 的 `Number(search.page) || 1`：非法/缺失一律回到第 1 页。
   RSC 预取与客户端读 URL 都走这里，避免两处 key 漂移。 */
export function normalizePage(value: number | null | undefined): number {
	return typeof value === 'number' && Number.isFinite(value) && value >= 1 ? Math.floor(value) : 1;
}

export function searchPostsOptions(q: string, page: number) {
	return queryOptions({
		queryKey: qk.search({ q, page }),
		queryFn: () => postService.search({ q, page, page_size: SEARCH_PAGE_SIZE }),
		/* 旧 loader：q 为空不发请求，直接返回空结果 */
		enabled: q.length > 0,
	});
}
