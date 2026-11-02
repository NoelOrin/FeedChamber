import {
	badgeService,
	checkinService,
	dashboardService,
	favoriteService,
	followService,
	notificationService,
	postService,
	userService,
} from '@feedchamber/api-client';
import { queryOptions } from '@tanstack/react-query';

import { DEFAULT_PAGE_SIZE } from '~/lib/pagination';
import { qk } from '~/lib/query/keys';

/**
 * 用户中心取数契约（照 lib/queries/shell.ts 的形式）：
 * RSC 侧 `prefetchQuery(xxxOptions())`、客户端 `useQuery(xxxOptions())` 共用同一份
 * key + queryFn，两端才能命中同一份水合缓存。key 一律出自 `qk`。
 */

/* 旧 index.tsx / $userId.tsx 的 `const PAGE_SIZE = 10`：个人页帖子列表不进 URL，留在模块内。 */
export const USER_POST_PAGE_SIZE = 10;

export type FollowTab = 'following' | 'followers';
const READ_VALUES = ['all', 'unread', 'read'] as const;
type ReadFilter = (typeof READ_VALUES)[number];

/** 把 URL 上的原始值收敛成服务端可接受的联合类型（非法值退回默认，与旧 validateSearch 一致）。 */
export function toReadFilter(value: string | string[] | undefined): ReadFilter {
	const raw = Array.isArray(value) ? value[0] : value;
	return READ_VALUES.includes(raw as ReadFilter) ? (raw as ReadFilter) : 'all';
}

export function toPage(value: string | string[] | undefined): number {
	const raw = Array.isArray(value) ? value[0] : value;
	return Number.parseInt(raw ?? '', 10) > 0 ? Number.parseInt(raw ?? '', 10) : 1;
}

export function toFirstValue(value: string | string[] | undefined, fallback: string): string {
	const raw = Array.isArray(value) ? value[0] : value;
	return raw && raw.length > 0 ? raw : fallback;
}

export function totalPages(total: number, pageSize: number) {
	return Math.max(1, Math.ceil(total / pageSize));
}

/* ——— 我的主页 /user ——— */

export function dashboardHomeOptions() {
	return queryOptions({ queryKey: qk.home, queryFn: () => dashboardService.home() });
}

export function publicProfileOptions(userId: string) {
	return queryOptions({
		queryKey: qk.user.publicProfile(userId),
		queryFn: () => userService.getPublicProfile(userId),
	});
}

export function userPostsOptions(authorId: string, page: number) {
	const params = { author_id: authorId, page, page_size: USER_POST_PAGE_SIZE };
	return queryOptions({ queryKey: qk.posts.list(params), queryFn: () => postService.list(params) });
}

export function userBadgesOptions(userId: string) {
	return queryOptions({ queryKey: qk.badges.byUser(userId), queryFn: () => badgeService.listByUser(userId) });
}

/* 旧 ActivityHeatmap 只需要 entries 表，这里就只取它，省去每个消费点重复解包。 */
export function userActivityOptions(userId: string, year: number) {
	return queryOptions({
		queryKey: qk.user.activity(userId, { year }),
		queryFn: async () => (await userService.getActivity(userId, year)).entries ?? {},
	});
}

export function checkinStatusOptions() {
	return queryOptions({ queryKey: qk.checkin.status, queryFn: () => checkinService.status() });
}

export function checkinHistoryOptions(year: number, month: number) {
	return queryOptions({
		queryKey: qk.checkin.history(`${year}-${String(month).padStart(2, '0')}`),
		queryFn: () => checkinService.history({ year, month }),
	});
}

/* ——— 通知 /user/notifications ——— */

interface NotificationFilter {
	page: number;
	type: string;
	read: ReadFilter;
}

function notificationListParams({ page, type, read }: NotificationFilter) {
	return {
		page,
		page_size: DEFAULT_PAGE_SIZE,
		type: type === 'all' ? undefined : type,
		read: read === 'all' ? undefined : read,
	};
}

export function notificationsOptions(filter: NotificationFilter) {
	const params = notificationListParams(filter);
	return queryOptions({
		queryKey: qk.notifications.list(params),
		queryFn: () => notificationService.list(params),
	});
}

/* ——— 关注 /user/follow ——— */

export function followListOptions(userId: string, tab: FollowTab, page: number) {
	const params = { page, page_size: DEFAULT_PAGE_SIZE };
	return queryOptions({
		queryKey: tab === 'followers' ? qk.follow.followers(userId, params) : qk.follow.following(userId, params),
		queryFn: () =>
			tab === 'followers' ? followService.followers(userId, params) : followService.following(userId, params),
	});
}

/* 旧 loader 无条件先拉一页「我关注的人」用来标注关注状态，这里保留同一次调用。 */
export function followingIdsOptions(userId: string) {
	const params = { page: 1, page_size: DEFAULT_PAGE_SIZE };
	return queryOptions({
		queryKey: qk.follow.following(userId, params),
		queryFn: () => followService.following(userId, params),
	});
}

export function followCountsOptions(userId: string) {
	return queryOptions({ queryKey: qk.follow.counts(userId), queryFn: () => followService.counts(userId) });
}

/* key 里带上页码：qk.user.search 只收一个字符串，搜索分支虽然旧版锁在第 1 页，
   仍把 page 编进去避免手动改 URL 时读到串页缓存。 */
export function userSearchOptions(keyword: string, page: number) {
	const params = { keyword, page, page_size: DEFAULT_PAGE_SIZE };
	return queryOptions({ queryKey: qk.user.search(`${keyword}@${page}`), queryFn: () => userService.search(params) });
}

/* ——— 收藏 /user/favorites ——— */

export function favoriteIdsOptions(page: number) {
	const params = { page, page_size: DEFAULT_PAGE_SIZE };
	return queryOptions({ queryKey: qk.favorite.list(params), queryFn: () => favoriteService.list(params) });
}

/* 旧 favorites loader 的两跳：先拿 id 列表，再批量换帖子；null 位（帖子已删）照旧丢掉。 */
export function favoritePostsOptions(ids: string[]) {
	return queryOptions({
		queryKey: qk.posts.byIds(ids),
		queryFn: async () => {
			const posts = await postService.listByIds(ids);
			return (posts ?? []).filter((post) => post !== null);
		},
		enabled: ids.length > 0,
	});
}
