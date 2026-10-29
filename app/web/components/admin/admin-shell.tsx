'use client';

import { useQueryClient } from '@tanstack/react-query';
import { usePathname, useRouter } from 'next/navigation';
import { useEffect } from 'react';
import { AccessProvider } from '~/components/admin/access/access-context';
import { AccessDenied } from '~/components/admin/access/access-denied';
import { ModuleForbidden } from '~/components/admin/access/module-forbidden';
import { AdminRoot, BootSplash } from '~/components/admin/admin-root';
import { AdminShell as AdminChrome } from '~/components/admin/layout/admin-shell';
import { canAccess, profileQueryKey, useDashboardAccess } from '~/lib/admin/auth/access';
import { rememberSiteUrl, resolveReturnTarget } from '~/lib/admin/auth/return-to';
import { requiredPermissionsFor } from '~/lib/admin/auth/route-access';
import { setSessionUser } from '~/lib/auth';
import { usePublicSettings } from '~/lib/settings';

/**
 * 后台分区（`/admin/**`）：与前台同一个应用，但内容只跑在浏览器里。
 *
 * - 未登录 → 主站登录页（带 redirect 回后台）
 * - 已登录但无后台权限 → 原路返回（拿不到来路才整页 403）
 * - 进了后台但没有该模块权限 → 壳层内 403（菜单已隐藏入口）
 */
export function AdminShell({ children }: { children: React.ReactNode }) {
	return (
		<AdminRoot>
			<AdminGate>{children}</AdminGate>
		</AdminRoot>
	);
}

function AdminGate({ children }: { children: React.ReactNode }) {
	const router = useRouter();
	const pathname = usePathname() ?? '/admin';
	const queryClient = useQueryClient();
	const access = useDashboardAccess();
	const grantedUser = access.status === 'granted' ? access.user : null;

	// 只有被拒时才需要主站地址（回跳兜底）；顺带缓存，供下次直接访问时使用。
	const denied = access.status === 'denied';
	const publicSettings = usePublicSettings();

	useEffect(() => {
		rememberSiteUrl(publicSettings.data?.site_url);
	}, [publicSettings.data]);

	const returnTarget = denied && access.reason !== 'error' ? resolveReturnTarget(publicSettings.data?.site_url) : null;

	useEffect(() => {
		if (returnTarget) window.location.replace(returnTarget);
	}, [returnTarget]);

	// 服务端档案即权威投影：同步进主站的会话 store，前台头像/导航也能立刻一致。
	useEffect(() => {
		if (grantedUser) setSessionUser(grantedUser);
	}, [grantedUser]);

	useEffect(() => {
		if (access.status === 'anonymous') {
			router.replace(`/login?redirect=${encodeURIComponent(pathname)}`);
		}
	}, [access.status, pathname, router]);

	if (access.status === 'checking' || access.status === 'anonymous') return <BootSplash />;

	if (denied) {
		const waitingTarget = access.reason !== 'error' && (returnTarget || publicSettings.isPending);
		if (waitingTarget) return <BootSplash />;

		return (
			<AccessDenied
				username={access.user?.username}
				role={access.user?.role}
				detail={access.detail}
				onRetry={() => queryClient.invalidateQueries({ queryKey: profileQueryKey })}
			/>
		);
	}

	const required = requiredPermissionsFor(pathname);
	const moduleAllowed = canAccess(access.permissions, access.permissionsLoaded, required);

	return (
		<AccessProvider permissions={access.permissions} permissionsLoaded={access.permissionsLoaded}>
			<AdminChrome user={access.user}>{moduleAllowed ? children : <ModuleForbidden required={required} />}</AdminChrome>
		</AccessProvider>
	);
}
