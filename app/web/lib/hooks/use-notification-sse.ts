'use client';

import { tryRefreshToken } from '@feedchamber/api-client';
import { SSEClient, type SSEClientOptions } from '@feedchamber/sse-client';
import { useCallback, useEffect, useRef, useState } from 'react';

/*
 * 浏览器侧必须是相对路径：dev 由 vite 代理到 :8080，生产同源。
 * 不依赖 getApiBase()——那个值会被 SSR 侧 setApiBaseUrl 改成绝对地址，
 * 而这条流只在浏览器建连（见下方 start() 的 window 守卫）。
 */
const SSE_URL = '/api/v1/notifications/stream';

interface SSEError {
	code: number;
	msg: string;
}

interface NotificationSSE {
	unread: number;
	error: SSEError | null;
	start: () => void;
	stop: () => void;
}

/**
 * 实时未读数订阅（旧 Solid hooks/useNotificationSSE.ts 平移）。
 * 客户端不自动建连——沿用旧约定，由调用方在挂载时 `start()`，卸载时由本 hook 断流。
 */
export function useNotificationSSE(): NotificationSSE {
	const [unread, setUnread] = useState(0);
	const [error, setError] = useState<SSEError | null>(null);
	const clientRef = useRef<SSEClient | null>(null);

	const ensureClient = useCallback((): SSEClient | null => {
		// 服务端不建连：SSEClient 会 fetch + 注册 visibilitychange，只能在浏览器里构造
		if (typeof window === 'undefined') return null;
		if (clientRef.current) return clientRef.current;

		clientRef.current = new SSEClient({
			url: SSE_URL,
			method: 'POST',
			maxRetries: 5,
			// HttpOnly cookie 会话：JS 读不到 token，靠 credentials: 'include'
			withCredentials: true,
			refreshToken: () => tryRefreshToken(),
			onData: (data) => {
				if (typeof data.unread === 'number') {
					setUnread(data.unread);
				}
			},
			onError: (err) => {
				setError(err);
			},
		} satisfies SSEClientOptions);

		return clientRef.current;
	}, []);

	const start = useCallback(() => {
		ensureClient()?.start();
	}, [ensureClient]);

	const stop = useCallback(() => {
		clientRef.current?.stop();
	}, []);

	// Solid 的 onCleanup(() => client.stop())：卸载即断流，避免路由切换后残留连接
	useEffect(
		() => () => {
			clientRef.current?.stop();
			clientRef.current = null;
		},
		[],
	);

	return { unread, error, start, stop };
}
