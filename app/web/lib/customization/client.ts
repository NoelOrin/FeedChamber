'use client';

import { customizeService, type PublicCustomization } from '@feedchamber/api-client';
import { queryOptions } from '@tanstack/react-query';

import { qk } from '~/lib/query/keys';

/**
 * 前台自定义配置的客户端查询。
 *
 * 与 RSC 侧的 `loadPublicCustomization` 指向同一后端接口，缓存 key 共用
 * `qk.customization`：后台改完配置后直接 invalidate 这一个 key，前后台一致。
 *
 * `staleTime` 长一些（5 分钟）：导航与首页模块不是高频变化的数据，
 * 而且它们已经在 RSC 侧注入过一遍，客户端再取通常只是为交互组件兜底。
 */
export const publicCustomizationOptions = queryOptions({
	queryKey: qk.customization,
	queryFn: async (): Promise<PublicCustomization> => {
		const data = await customizeService.getPublicCustomization();
		return {
			nav: data?.nav ?? [],
			home_modules: data?.home_modules ?? [],
			head_snippets: data?.head_snippets ?? [],
			body_snippets: data?.body_snippets ?? [],
			theme_tokens: data?.theme_tokens ?? [],
		};
	},
	staleTime: 300_000,
	retry: false,
});
