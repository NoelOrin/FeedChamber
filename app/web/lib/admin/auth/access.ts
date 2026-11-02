'use client';

import { AuthError, HttpError, permissionService, type User, userService } from '@feedchamber/api-client';
import { useQuery } from '@tanstack/react-query';

/**
 * 后台准入鉴权（唯一的准入口）。
 *
 * 判定顺序：
 * 1. 用会话 cookie 拉 `/user/profile` —— 服务端说了算，不信任 localStorage 里的角色投影；
 * 2. 角色在 `DASHBOARD_ROLES` 里 → 放行；
 * 3. 否则拉该角色的权限码，命中 `DASHBOARD_PERMISSION_CODES` 任一 → 放行（兼容自定义角色）；
 * 4. 其余一律拒绝，由调用方渲染 403（未登录则由调用方跳登录页）。
 */

/** 视为「后台角色」的角色名。改这里即改后台准入门槛。 */
export const DASHBOARD_ROLES = ['admin', 'moderator'] as const;

/**
 * 角色之外的兜底：命中任一权限码也允许进入后台。
 * 刻意不含 `user:read` / `role:read` / `stats:read` 这类普通用户也有的读权限。
 */
export const DASHBOARD_PERMISSION_CODES = [
	'setting:manage',
	'role:manage',
	'user:update',
	'user:delete',
	'banned:manage',
	'batch:edit',
	'announcement:manage',
	'report:manage',
	'space:update',
	'space:delete',
	'tag:update',
	'tag:delete',
	'post:pin',
	'post:lock',
	'admin:search',
	'admin:metrics',
	'backup:manage',
	'plugin:manage',
	'level:manage',
	'gate:manage',
	'reconcile:counters',
] as const;

export type AccessDenyReason = 'role' | 'permission' | 'error';

export type AccessState =
	| { status: 'checking' }
	| { status: 'anonymous' }
	| { status: 'denied'; reason: AccessDenyReason; user?: User; detail?: string }
	| {
			status: 'granted';
			user: User;
			/** 该角色在后端的全部权限码。 */
			permissions: string[];
			/** 权限码是否真的取到了；取不到时页面级门控一律放行（后端仍会把关）。 */
			permissionsLoaded: boolean;
	  };

/** 角色或权限码是否够格进后台（纯函数，便于单测与复用）。 */
export function hasDashboardAccess(role: string | undefined, permissions: readonly string[] = []): boolean {
	if (!role) return false;
	if ((DASHBOARD_ROLES as readonly string[]).includes(role)) return true;
	return permissions.some((code) => (DASHBOARD_PERMISSION_CODES as readonly string[]).includes(code));
}

/** 权限判定：`anyOf` 为空表示不需要额外权限；未取到权限码时放行（UI 门控不承担安全职责）。 */
export function canAccess(
	permissions: readonly string[],
	permissionsLoaded: boolean,
	anyOf?: readonly string[],
): boolean {
	if (!anyOf || anyOf.length === 0) return true;
	if (!permissionsLoaded) return true;
	return anyOf.some((code) => permissions.includes(code));
}

function isUnauthorized(error: unknown): boolean {
	if (error instanceof AuthError) return true;
	if (error instanceof HttpError) return error.status === 401;
	return false;
}

function isForbidden(error: unknown): boolean {
	return error instanceof HttpError && error.status === 403;
}

function errorDetail(error: unknown): string {
	return error instanceof Error ? error.message : String(error ?? '');
}

export const profileQueryKey = ['auth', 'profile'] as const;

/**
 * 解析当前会话的后台准入状态。必须在客户端组件里调用。
 */
export function useDashboardAccess(): AccessState {
	const profile = useQuery({
		queryKey: profileQueryKey,
		queryFn: () => userService.getProfile(),
		retry: false,
		staleTime: 60_000,
		gcTime: 300_000,
	});

	const role = profile.data?.role;
	const roleAllowed = role ? (DASHBOARD_ROLES as readonly string[]).includes(role) : false;

	// 角色一旦确定就取该角色的权限码：准入判定与页面/菜单门控共用这一份。
	const perms = useQuery({
		queryKey: ['auth', 'role-permissions', role],
		queryFn: () => permissionService.getByRole(role as string),
		enabled: !!role,
		retry: false,
		staleTime: 300_000,
	});

	if (profile.isPending) return { status: 'checking' };

	if (profile.isError) {
		if (isUnauthorized(profile.error)) return { status: 'anonymous' };
		if (isForbidden(profile.error)) {
			return { status: 'denied', reason: 'permission', detail: errorDetail(profile.error) };
		}
		return { status: 'denied', reason: 'error', detail: errorDetail(profile.error) };
	}

	const user = profile.data;

	// 角色不足时必须等权限码回来才能判断；角色本身够格也等，避免菜单先闪全量再收缩。
	if (perms.isPending) return { status: 'checking' };

	const permissions = perms.data?.permissions ?? [];
	const permissionsLoaded = perms.isSuccess;

	if (roleAllowed || hasDashboardAccess(user.role, permissions)) {
		return { status: 'granted', user, permissions, permissionsLoaded };
	}

	return { status: 'denied', reason: 'role', user };
}
