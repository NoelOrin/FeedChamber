'use client';

import { useEffect } from 'react';

import { ErrorFallback } from '~/components/error/error-fallback';

/** 旧 `__root.tsx` 的 errorComponent + GlobalErrorBoundary 位置：路由树内的错误在这里兜底。 */
export default function AppError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
	useEffect(() => {
		// 与旧 GlobalErrorBoundary 的 window 监听同理：只落控制台，不改浏览器 DevTools 行为
		console.error('[app/error]', error);
	}, [error]);

	return <ErrorFallback error={error} reset={reset} variant="global" />;
}
