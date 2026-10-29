'use client';

import type { User } from '@feedchamber/api-client';
import { type DehydratedState, hydrate, QueryClientProvider, useQueryClient } from '@tanstack/react-query';
import { NuqsAdapter } from 'nuqs/adapters/next/app';
import type * as React from 'react';
import { useEffect, useState } from 'react';

import { ToastViewport } from '~/components/ui/toast';
import { TooltipProvider } from '~/components/ui/tooltip';
import { authClient, authSession, startAuthSync } from '~/lib/auth';
import { getBrowserQueryClient, getQueryClient } from '~/lib/query/client';
import { qk } from '~/lib/query/keys';

export function Providers({
	children,
	dehydratedState,
	initialUser,
}: {
	children: React.ReactNode;
	dehydratedState?: DehydratedState;
	initialUser?: User | null;
}) {
	const [client] = useState(() => {
		// SSR 必须用 cache() 的每请求实例：浏览器单例在 Node 里会跨请求存活，
		// 上一位访客的信息流可能被渲进这一位的 HTML。
		const next = typeof window === 'undefined' ? getQueryClient() : getBrowserQueryClient();
		if (dehydratedState) hydrate(next, dehydratedState);
		return next;
	});

	return (
		<QueryClientProvider client={client}>
			<NuqsAdapter>
				<TooltipProvider delayDuration={200}>
					<AuthSessionSeed user={initialUser} />
					{children}
					<ToastViewport />
					<AuthSync />
					<WindowErrorLog />
				</TooltipProvider>
			</NuqsAdapter>
		</QueryClientProvider>
	);
}

/** RSC 已把 profile 水合进缓存；这里只把它同步给命令式的 authSession 门面。 */
function AuthSessionSeed({ user }: { user?: User | null }) {
	useEffect(() => {
		if (user) authSession.setAuthenticated(user);
		else if (authSession.status === 'unknown') authSession.setAnonymous();
	}, [user]);

	return null;
}

/** 跨标签页登录/登出同步（旧 __root 的 createEffect 平移到此，启动点随 Provider 挂载）。 */
function AuthSync() {
	const queryClient = useQueryClient();

	useEffect(() => {
		const refetchProfile = async () => {
			try {
				const user = await authClient.loadProfile();
				queryClient.setQueryData(qk.profile, user);
				/* 只写 Query 缓存不够：全站登录态读的是 zustand。LOGOUT 分支老实打了
				   setAnonymous，登录分支漏了对称的一半 → 另一标签页登录后本页 store
				   仍是 anonymous，回复框/编辑入口/气泡方向/铃铛全部错位。 */
				authSession.setAuthenticated(user);
			} catch {
				authSession.setAnonymous();
				queryClient.removeQueries({ queryKey: qk.profile });
			}
		};

		return startAuthSync({
			AUTH_LOGIN: refetchProfile,
			AUTH_REFRESHED: refetchProfile,
			PROFILE_UPDATED: refetchProfile,
			AUTH_LOGOUT: () => {
				authSession.setAnonymous();
				queryClient.clear();
			},
			AUTH_EXPIRED: () => {
				authSession.setExpired();
				queryClient.clear();
			},
		});
	}, [queryClient]);

	return null;
}

/** 旧 GlobalErrorBoundary 的 window 级兜底：不拦截、只把逃逸出 React 树的错误记进日志。 */
function WindowErrorLog() {
	useEffect(() => {
		// 水合完成的确定性标记，e2e 靠它决定何时开始交互（SSR 已渲染 DOM，但事件要等水合）
		document.documentElement.dataset.hydrated = '1';

		const onError = (event: ErrorEvent) => {
			console.error('[window error]', event.error ?? event.message);
		};
		const onRejection = (event: PromiseRejectionEvent) => {
			console.error('[unhandledrejection]', event.reason);
		};
		window.addEventListener('error', onError);
		window.addEventListener('unhandledrejection', onRejection);
		return () => {
			window.removeEventListener('error', onError);
			window.removeEventListener('unhandledrejection', onRejection);
		};
	}, []);

	return null;
}
