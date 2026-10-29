'use client';

import { type ReactNode, useEffect, useState } from 'react';

import { captureReturnTarget } from '~/lib/admin/auth/return-to';

/**
 * 后台的客户端挂载门：服务端与首次水合只吐骨架，挂载完成后才渲染应用。
 *
 * 后台要「完全动态、不走 SSR」——antd / Pro Components 的样式注入、主题判定、
 * 权限探测都只发生在浏览器。主站的根布局仍然是 SSR 的（前台需要），
 * 这层门把后台这段路由挡在服务端渲染之外。
 */
export function AdminRoot({ children }: { children: ReactNode }) {
	const [mounted, setMounted] = useState(false);

	useEffect(() => {
		// 第一件事：记下来路，无权限时要原路送回。
		captureReturnTarget();
		setMounted(true);
	}, []);

	if (!mounted) return <BootSplash />;
	return <>{children}</>;
}

/** 纯 DOM 的启动骨架：刻意不用 antd，避免在服务端引入组件库运行时。 */
export function BootSplash() {
	return (
		<div className="bg-background flex min-h-screen w-full items-center justify-center">
			<div className="flex flex-col items-center gap-3">
				<span aria-hidden className="border-border border-t-primary size-8 animate-spin rounded-full border-2" />
				<span className="text-muted-foreground text-sm">正在加载后台…</span>
			</div>
		</div>
	);
}
