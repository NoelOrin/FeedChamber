'use client';

import { settingService } from '@orinforum/api-client';
import { queryOptions, useQuery } from '@tanstack/react-query';

import { qk } from '~/lib/query/keys';

const DEFAULT_SITE_NAME = 'OrinForum';

/**
 * 公开站点设置（站点名、site_url 等）。前后台共用一份 query key 与缓存，
 * 后台不再自己发 `/settings/public`。服务端侧的同名接缝是 `loadPublicSettings()`。
 */
export function publicSettingsOptions() {
	return queryOptions({
		queryKey: qk.settings,
		queryFn: () => settingService.getPublic(),
		staleTime: 300_000,
		retry: false,
	});
}

export function usePublicSettings() {
	return useQuery(publicSettingsOptions());
}

/**
 * 站点名。优先用服务端下发的初始值（如果有），其次本地缓存，最后默认名，
 * 避免首帧出现空标题。
 */
export function useSiteName(initialName?: string): string {
	const { data } = usePublicSettings();
	const fromQuery = data?.site_name;
	if (fromQuery) return fromQuery;
	if (initialName) return initialName;
	if (typeof window !== 'undefined') {
		const cached = window.localStorage.getItem('site_name');
		if (cached) return cached;
	}
	return DEFAULT_SITE_NAME;
}
