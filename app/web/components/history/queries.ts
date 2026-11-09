import { historyService } from '@feedchamber/api-client';
import { queryOptions } from '@tanstack/react-query';

import { DEFAULT_PAGE_SIZE } from '~/lib/pagination';
import { qk } from '~/lib/query/keys';

/*
 * 浏览记录取数契约：RSC 预取与客户端 useQuery 共用同一份 key/params，
 * 否则水合后首帧会立刻重发一次请求。
 *
 * 分页字段用 page / pageSize（openspec api-style 的规范名），不是旧 ListParams 的 page_size。
 */
export function historyListOptions(page = 1, pageSize = DEFAULT_PAGE_SIZE) {
	const params = { page, pageSize };
	return queryOptions({
		queryKey: qk.history.list(params),
		queryFn: () => historyService.list(params),
	});
}

export function totalPages(total: number, pageSize = DEFAULT_PAGE_SIZE) {
	return Math.max(1, Math.ceil(total / pageSize));
}
