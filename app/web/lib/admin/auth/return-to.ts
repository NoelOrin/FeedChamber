'use client';

/**
 * 「进后台之前那一页」的记录与还原。
 *
 * 后台对普通用户是关门的：检测到没有后台权限时，直接把人送回他点进后台前所在的页面，
 * 而不是把他按在一个 403 页面上。还原顺序：
 *
 * 1. `sessionStorage` 里记录的上一次来路（进后台时抓一次，之后后台内部跳转不覆盖）；
 * 2. `document.referrer`；
 * 3. 站点根路径（`/settings/public` 的 `site_url`，或登录时缓存的值）。
 *
 * 后台现在挂在主站的 `/admin` 下，所以同源地址是正常情况——只要不是 `/admin`、
 * `/login`、`/register`、`/oauth` 这些会打转的目标就接受。
 * 拿不到任何可用目标时返回 `null`，调用方退回 403 页面。
 */

const RETURN_KEY = 'admin_return_to';
const SITE_URL_KEY = 'site_url';

/** 跳回去只会打转的路径：后台自身 + 登录/注册/OAuth。 */
const LOOP_PATHS = ['/admin', '/login', '/register', '/oauth'];

function parseUrl(raw: string | null | undefined): URL | null {
	if (!raw) return null;
	try {
		return new URL(raw, window.location.href);
	} catch {
		return null;
	}
}

function isLoopPath(pathname: string): boolean {
	return LOOP_PATHS.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`));
}

/** 可作为回跳目标：同源的非循环路径，或任意站外地址。 */
function isUsableTarget(raw: string | null | undefined): boolean {
	const url = parseUrl(raw);
	if (!url) return false;
	if (url.origin !== window.location.origin) return true;
	return !isLoopPath(url.pathname);
}

/** 进入后台时调用一次：把「点进来之前那一页」记进 sessionStorage。 */
export function captureReturnTarget(): void {
	if (typeof window === 'undefined') return;
	try {
		if (sessionStorage.getItem(RETURN_KEY)) return;
		const referrer = document.referrer;
		if (isUsableTarget(referrer)) sessionStorage.setItem(RETURN_KEY, referrer);
	} catch {
		// 隐私模式 / 存储被禁用时静默降级到站点根路径。
	}
}

/**
 * 解析回跳目标。`siteUrl` 来自公开设置（站点地址），可缺省。
 * 返回 `null` 表示没有安全可用的目标，调用方应展示 403。
 */
export function resolveReturnTarget(siteUrl?: string | null): string | null {
	if (typeof window === 'undefined') return null;

	try {
		const stored = sessionStorage.getItem(RETURN_KEY);
		if (stored && isUsableTarget(stored)) return stored;
	} catch {
		// 忽略存储异常。
	}

	if (isUsableTarget(document.referrer)) return document.referrer;

	const cached = (() => {
		try {
			return localStorage.getItem(SITE_URL_KEY);
		} catch {
			return null;
		}
	})();
	const fallback = siteUrl || cached || '/timeline';
	return isUsableTarget(fallback) ? fallback : null;
}

/** 站点地址缓存：壳层拿到公开设置后写一份，供 403 回跳兜底。 */
export function rememberSiteUrl(url: string | undefined | null): void {
	if (!url) return;
	try {
		localStorage.setItem(SITE_URL_KEY, url);
	} catch {
		// 忽略存储异常。
	}
}
