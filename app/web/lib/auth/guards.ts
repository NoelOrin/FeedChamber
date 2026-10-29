import type { User } from '@orinforum/api-client';
import { headers } from 'next/headers';
import { redirect } from 'next/navigation';

import { loadProfile, loadPublicSettings } from '~/lib/queries/server';

const AUTH_FLAG = /(?:^|;\s*)master_auth=1/;

/** Cheap cookie-presence check — avoids a profile round-trip for the common case. */
async function hasSessionFlag(): Promise<boolean> {
	return AUTH_FLAG.test((await headers()).get('cookie') ?? '');
}

/** Resolve the current user from the request cookies, or null when anonymous. */
async function getSessionUser(): Promise<User | null> {
	if (!(await hasSessionFlag())) return null;
	// loadProfile rather than userService.getProfile(): the root layout's shell load
	// already fetched it, and cache() collapses that into one backend call per request.
	return loadProfile();
}

/** Server-layout equivalent of TanStack Router's `requireAuth` beforeLoad hook. */
export async function requireAuth(pathname = '/timeline'): Promise<User> {
	const user = await getSessionUser();
	if (user) return user;
	redirect(`/login?redirect=${encodeURIComponent(pathname)}`);
}

/**
 * Resolve the viewer if present, without redirecting.
 *
 * Public content pages use this instead of `requireAuth`: whether an anonymous
 * visitor may read them is a server-side setting (`post_auth_required` etc.),
 * not a frontend decision. See openspec change 2026-10-28-seo-public-indexing.
 */
export async function optionalAuth(): Promise<User | null> {
	return getSessionUser();
}

/**
 * 公开内容页的动态登录门。
 *
 * 设置项为 `true` 时等价于 `requireAuth`（匿名跳登录并带回跳），为 `false`（默认）时
 * 放行并返回当前用户（可能为 null）。这是「管理员把设置改回 true 就能恢复登录墙」的
 * 落地实现——缺了它，设置项只是个改不动前端行为的死配置。
 *
 * 对应 openspec/specs/seo 的「管理员恢复登录墙」场景。设置读取走 `loadPublicSettings`，
 * 同一请求内由 `cache()` 与根布局的取数合并为一次后端调用。
 */
export async function requireAuthBySetting(settingKey: string, pathname: string): Promise<User | null> {
	const settings = await loadPublicSettings();
	// 只认字符串 'true'：未配置/空串一律视为放行，与后端 AuthGate 的语义一致。
	if (settings[settingKey] !== 'true') return getSessionUser();
	return requireAuth(pathname);
}

/** Server-layout equivalent of `requireGuest`. */
export async function requireGuest(): Promise<void> {
	if (await getSessionUser()) redirect('/timeline');
}
