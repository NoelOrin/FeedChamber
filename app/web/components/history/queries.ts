import { historyService } from '@feedchamber/api-client';
import { queryOptions } from '@tanstack/react-query';

import { DEFAULT_PAGE_SIZE } from '~/lib/pagination';
import { qk } from '~/lib/query/keys';

/*
 * 浏览记录取数契约：RSC 预取与客户端 useQuery 共用同一份 key/params，
 * 否则水合后首帧会立刻重发一次请求。
 *
 * 分页字段在网络层统一 page / page_size（openspec api-style 的规范名，2026-11-11 全站统一）；
 * service 边界把 pageSize 转成 page_size，页面组件仍用 TS 习惯名。
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
