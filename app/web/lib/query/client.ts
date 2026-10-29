import { ApiError, HttpError, NetworkError } from '@orinforum/api-client';
import { QueryClient } from '@tanstack/react-query';
import { cache } from 'react';

/*
 * staleTime 取值说明（迁移计划 §8 开放项 2）：
 * 全站默认沿用 dashboard 的 60s（同栈同行为、少一次重取）；
 * 只有信息流列表按旧 router 的 10s 覆写，因为那是用户会盯着看是否更新的区域。
 */
const DEFAULT_STALE_TIME = 60_000;
export const FEED_STALE_TIME = 10_000;

/*
 * 默认 retry: 1 会把 4xx 也重发一遍：实测 `/user/{id}/activity` 的 403、
 * 以及部署链路漏配 /api 时的成片 404 都被各发 2 次。权限/参数/不存在类错误
 * 重试必然同样失败，只对真正可能自愈的传输错误与 5xx 重试。
 */
function shouldRetry(failureCount: number, error: unknown) {
	if (failureCount >= 1) return false;
	if (error instanceof NetworkError) return true;
	if (error instanceof HttpError) return error.status >= 500;
	if (error instanceof ApiError) return error.code >= 5000;
	return false;
}

function createQueryClient() {
	return new QueryClient({
		defaultOptions: {
			queries: {
				refetchOnWindowFocus: false,
				retry: shouldRetry,
				staleTime: DEFAULT_STALE_TIME,
				gcTime: 300_000,
			},
			mutations: { retry: 0 },
		},
	});
}

/** RSC 用：每个请求一份缓存，避免跨请求串号。 */
export const getQueryClient = cache(createQueryClient);

let browserClient: QueryClient | null = null;

/** 客户端用：模块级单例，供 Provider 与 Server Actions 重定向后的 router refresh 共享。 */
export function getBrowserQueryClient() {
	if (!browserClient) browserClient = createQueryClient();
	return browserClient;
}
