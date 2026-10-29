'use client';

import { createContext, type ReactNode, useContext, useMemo } from 'react';

import { canAccess } from '~/lib/admin/auth/access';

interface AccessContextValue {
	permissions: string[];
	permissionsLoaded: boolean;
}

const AccessContext = createContext<AccessContextValue>({ permissions: [], permissionsLoaded: false });

/** 把准入阶段拿到的权限码下发给壳层与页面，避免各处重复请求。 */
export function AccessProvider({
	permissions,
	permissionsLoaded,
	children,
}: AccessContextValue & { children: ReactNode }) {
	const value = useMemo(() => ({ permissions, permissionsLoaded }), [permissions, permissionsLoaded]);
	return <AccessContext.Provider value={value}>{children}</AccessContext.Provider>;
}

/** 当前会话是否拥有 `anyOf` 中的任一权限码；不传参数表示只要求已登录。 */
export function useCan(anyOf?: readonly string[]): boolean {
	const { permissions, permissionsLoaded } = useContext(AccessContext);
	return canAccess(permissions, permissionsLoaded, anyOf);
}

/** 原始权限码，供需要自行组合判断的场景使用。 */
export function useAccessPermissions(): AccessContextValue {
	return useContext(AccessContext);
}
