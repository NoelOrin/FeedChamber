'use client';

import { useEffect } from 'react';

import { ErrorFallback } from '~/components/error/error-fallback';

/* 根 layout 自身崩溃时 Next 会渲染这个组件替换整份文档，
  此时 layout 里的样式与 Provider 都不存在：样式在此自取，html/body 也必须自带 */
import './globals.css';

/** 根级兜底：连 root layout 都渲染失败时仍要能出页面，故不依赖任何 context / window 读取。 */
export default function GlobalError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
	useEffect(() => {
		console.error('[app/global-error]', error);
	}, [error]);

	return (
		<html lang="zh-CN" suppressHydrationWarning>
			<head>
				<meta charSet="utf-8" />
				<meta name="viewport" content="width=device-width, initial-scale=1.0" />
				<meta name="theme-color" content="#8c2de4" />
				<title>OrinForum</title>
			</head>
			<body className="relative min-h-screen">
				<ErrorFallback error={error} reset={reset} variant="global" />
			</body>
		</html>
	);
}
