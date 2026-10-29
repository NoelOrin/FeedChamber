import { useEffect } from 'react';
import { create } from 'zustand';

const messages: Record<string, Record<string, string>> = {
	'zh-CN': {
		skip_to_content: '跳到主要内容',
		nav_latest: '最新',
		nav_new: '新帖',
		nav_unread: '未读',
		nav_read: '已读',
		nav_posted: '我的帖子',
		nav_hot: '最热',
		nav_commented: '评论最多',
		nav_active: '活跃',
		nav_search: '搜索',
		nav_spaces: '版面',
		nav_tags: '标签',
		nav_create: '发帖',
		composer_title: '发表新帖',
		composer_title_label: '标题',
		composer_space_label: '版面',
		composer_tags_label: '标签',
		composer_content_label: '内容',
		composer_publish: '发布',
		composer_cancel: '取消',
	},
	en: {
		skip_to_content: 'Skip to main content',
		nav_latest: 'Latest',
		nav_new: 'New',
		nav_unread: 'Unread',
		nav_read: 'Read',
		nav_posted: 'My posts',
		nav_hot: 'Hot',
		nav_commented: 'Most commented',
		nav_active: 'Active',
		nav_search: 'Search',
		nav_spaces: 'Spaces',
		nav_tags: 'Tags',
		nav_create: 'Create',
		composer_title: 'New Topic',
		composer_title_label: 'Title',
		composer_space_label: 'Space',
		composer_tags_label: 'Tags',
		composer_content_label: 'Content',
		composer_publish: 'Publish',
		composer_cancel: 'Cancel',
	},
};

function getLocale(): string {
	if (typeof window === 'undefined') return 'zh-CN';
	return window.localStorage.getItem('language') || navigator.language || 'zh-CN';
}

export function t(key: string, locale = getLocale()): string {
	const table = messages[locale] || messages['zh-CN'] || {};
	return table[key] || key;
}

export const SUPPORTED_LOCALES = ['zh-CN', 'en-US'] as const;

/**
 * 语言偏好的唯一事实源：localStorage `language` + `<html lang>`。
 * 后台的 antd locale 与前台文案都从这一份读，避免两套开关互相覆盖。
 */
const useLocaleStore = create<{ locale: string; set: (locale: string) => void }>((set) => ({
	locale: 'zh-CN',
	set: (locale) => set({ locale }),
}));

/** 写入语言偏好：持久化 + 同步 <html lang>。 */
export function setLocale(locale: string): void {
	if (!SUPPORTED_LOCALES.includes(locale as (typeof SUPPORTED_LOCALES)[number])) return;
	localStorage.setItem('language', locale);
	document.documentElement.lang = locale;
	useLocaleStore.getState().set(locale);
}

/** 读取并订阅当前语言。首帧用 'zh-CN' 与 SSR 一致，挂载后同步真实值。 */
export function useLocale(): { locale: string; setLocale: (locale: string) => void } {
	const locale = useLocaleStore((state) => state.locale);

	useEffect(() => {
		const stored = getLocale();
		const normalized = SUPPORTED_LOCALES.includes(stored as (typeof SUPPORTED_LOCALES)[number]) ? stored : 'zh-CN';
		if (useLocaleStore.getState().locale !== normalized) useLocaleStore.getState().set(normalized);
	}, []);

	return { locale, setLocale };
}
