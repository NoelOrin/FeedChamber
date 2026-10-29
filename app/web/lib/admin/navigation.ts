'use client';

import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { useCallback, useMemo } from 'react';

/**
 * 后台的导航适配层。
 *
 * 路由由 vinext 的 App Router 承担，页面里只需要三件事：跳转、拿路径、读查询参数。
 * 这里把这三件事收敛成三个 hook，避免每个页面各自拼 `window.location`，也避免
 * 直接散布 `useRouter` 的 push/replace 细节。查询参数一律按字符串读，保持与
 * 旧路由表 `validateSearch` 的行为一致（`?id=` / `?tab=` / `?redirect=`）。
 */

export interface NavigateOptions {
	/** 站内路径，如 `/users/detail`。 */
	to: string;
	/** 查询参数，值会被 `String()` 后拼到 URL 上。 */
	search?: Record<string, string | number | boolean | undefined | null>;
	replace?: boolean;
}

export type Navigate = (options: NavigateOptions | string) => void;

function buildHref(to: string, search?: NavigateOptions['search']): string {
	if (!search) return to;
	const params = new URLSearchParams();
	for (const [key, value] of Object.entries(search)) {
		if (value === undefined || value === null || value === '') continue;
		params.set(key, String(value));
	}
	const query = params.toString();
	if (!query) return to;
	return `${to}${to.includes('?') ? '&' : '?'}${query}`;
}

export function useNavigate(): Navigate {
	const router = useRouter();
	// 必须稳定：页面里大量 `useEffect(..., [navigate])`，引用每帧变化会打成无限渲染。
	return useCallback<Navigate>(
		(options) => {
			const { to, search, replace } = typeof options === 'string' ? { to: options } : options;
			const href = buildHref(to, search);
			if (replace) router.replace(href);
			else router.push(href);
		},
		[router],
	);
}

/** 与旧 `useLocation()` 同形：页面只用得到 `pathname`。 */
export function useLocation(): { pathname: string } {
	const pathname = usePathname();
	return useMemo(() => ({ pathname: pathname ?? '/' }), [pathname]);
}

/** 查询参数对象；缺省返回空对象，调用方按需取值。 */
export function useSearch(_options?: unknown): Record<string, string> {
	const searchParams = useSearchParams();
	// 按序列化后的字符串记忆：`useSearchParams()` 的实例身份在部分场景会变，
	// 直接依赖它会让 `useEffect(..., [search])` 反复触发。
	const key = searchParams.toString();
	return useMemo(() => {
		const entries: Record<string, string> = {};
		for (const [name, value] of new URLSearchParams(key).entries()) entries[name] = value;
		return entries;
	}, [key]);
}
