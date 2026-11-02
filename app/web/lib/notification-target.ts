import type { Notification } from '@feedchamber/api-client';

/**
 * 通知深链：把一条通知映射成站内路由。
 *
 * 两侧（顶栏通知铃 `components/layout/notification-bell.tsx` 与通知中心
 * `components/user/notifications-page.tsx`）此前各自维护一份逐字相同的 switch，
 * 结果两份**同时**漏掉了 mention / watch / motion——同一个缺陷要在两个文件里各修一次。
 * 现在只留这一个出口。
 *
 * `reference_id` 的语义随 type 变化，由后端 `service/notification.go` 的订阅端决定：
 * - comment / like / mention / watch → post id（评论、点赞、提及、关注话题都落在帖子上）
 * - follow                          → 关注者的 user id
 * - pm                              → conversation id
 * - motion                          → motion id（目前**没有**详情路由，只能落大厅）
 *
 * 返回 null 表示「无跳转」：调用方应只做标记已读，不要 navigate。
 */
export function notificationHref(n: Notification): string | null {
	const ref = n.reference_id;
	if (!ref) return null;
	const id = encodeURIComponent(ref);

	switch (n.type) {
		// 帖子类：评论、回复、点赞、提及、关注话题都深链到帖子详情。
		// 带 -ed 的是历史命名，后端现在只产出不带 -ed 的，保留以免旧数据失效。
		case 'comment':
		case 'replied':
		case 'like':
		case 'liked':
		case 'mention':
		case 'watch':
			return `/timeline/post?id=${id}`;
		case 'follow':
		case 'followed':
			return `/user/${id}`;
		case 'pm':
			return `/messages/${id}`;
		case 'motion':
			return '/motions';
		default:
			return null;
	}
}
