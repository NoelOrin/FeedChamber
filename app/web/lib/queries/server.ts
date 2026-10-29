import type { SpaceNode, Tag, User } from '@feedchamber/api-client';
import { settingService, spaceService, tagService, userService } from '@feedchamber/api-client';
import { cache } from 'react';

import '~/lib/api/adapter';

/**
 * RSC 侧数据接缝。全部用 React `cache()` 包一层：同一次请求内
 * （generateMetadata → layout → page 连续取同一份数据）只发一次后端调用。
 * 失败一律降级成空值，让页面骨架照常 SSR，而不是整棵树抛错。
 */

export const loadPublicSettings = cache(async (): Promise<Record<string, string>> => {
	try {
		return (await settingService.getPublic()) ?? {};
	} catch {
		return {};
	}
});

export const loadProfile = cache(async (): Promise<User | null> => {
	try {
		return await userService.getProfile();
	} catch {
		return null;
	}
});

export const loadSpaceTree = cache(async (): Promise<SpaceNode[]> => {
	try {
		return (await spaceService.tree()) ?? [];
	} catch {
		return [];
	}
});

/** 返回 `{ data, total }` 信封，与 tagService.list() 一致，否则水合形状对不上。 */
export const loadTags = cache(async (): Promise<{ data: Tag[]; total: number }> => {
	try {
		const { data, total } = await tagService.list({ page: 1, page_size: 50 });
		return { data: data ?? [], total: total ?? 0 };
	} catch {
		return { data: [], total: 0 };
	}
});

export const loadShellData = cache(async () => {
	const [settings, user, spaces, tags] = await Promise.all([
		loadPublicSettings(),
		loadProfile(),
		loadSpaceTree(),
		loadTags(),
	]);
	return { settings, user, spaces, tags };
});
