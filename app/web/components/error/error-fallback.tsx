'use client';

import { type ErrorView, resolveErrorView } from '@feedchamber/api-client';
import { QueryClientContext } from '@tanstack/react-query';
import { useContext, useMemo } from 'react';

import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { cn } from '~/lib/utils';

export type ErrorFallbackVariant = 'global' | 'section';

export interface ErrorFallbackProps {
	error: unknown;
	reset?: () => void;
	variant?: ErrorFallbackVariant;
	/** override retry handler; defaults to reset */
	onRetry?: () => void;
	/** hide the "back to home" action (useful when embedded) */
	hideHome?: boolean;
	/** override the home target; defaults to "/" (the admin shell passes "/admin") */
	onNavigateHome?: () => void;
	/** override the login target; defaults to a native "/login" link */
	onNavigateLogin?: () => void;
}

const GLYPH_BY_KIND: Partial<Record<ErrorView['kind'], string>> = {
	not_found: '404',
	auth: '401',
	forbidden: '403',
	banned: '403',
	network: '离线',
	rate_limited: '429',
	server: '500',
	frontend: '!',
};

const DEFAULT_GLYPH = '!';

function CodeBadge({ code, status }: { code?: number; status?: number }) {
	const label = code !== undefined ? `code ${code}` : status !== undefined ? `HTTP ${status}` : null;
	if (!label) return null;
	return (
		<span className="inline-flex items-center rounded-md border border-border px-2 py-0.5 text-[11px] text-muted-foreground">
			{label}
		</span>
	);
}

export function ErrorFallback({
	error,
	reset,
	variant,
	onRetry,
	hideHome,
	onNavigateHome,
	onNavigateLogin,
}: ErrorFallbackProps) {
	const isGlobal = (variant ?? 'section') === 'global';
	/* global-error.tsx 会替换整棵根布局（含 QueryClientProvider），
	 所以直接读 context（无 Provider 时为 undefined），不能用会抛错的 useQueryClient() */
	const queryClient = useContext(QueryClientContext);
	const view = useMemo(() => resolveErrorView(error), [error]);
	const message = error instanceof Error ? error.message : String(error ?? '');
	const stack = error instanceof Error ? error.stack : undefined;

	const handleRetry = () => {
		if (onRetry) onRetry();
		else if (reset) reset();
		else if (queryClient) queryClient.invalidateQueries();
		else if (typeof window !== 'undefined') window.location.reload();
	};

	const glyph = GLYPH_BY_KIND[view.kind] ?? DEFAULT_GLYPH;

	const body = (
		<div className="flex flex-col items-center">
			<div className="text-4xl leading-none font-semibold tracking-tight text-muted-foreground">{glyph}</div>
			<div className="mt-3 text-center text-lg leading-tight font-semibold text-foreground">{view.title}</div>
			<div className="mt-2 max-w-[44ch] text-center text-[13px] leading-relaxed text-muted-foreground">
				{view.description}
			</div>

			<div className="mt-3 flex flex-wrap items-center justify-center gap-2">
				<CodeBadge code={view.code} status={view.status} />
				{view.kind === 'frontend' ? <span className="text-[11px] text-muted-foreground">前端异常</span> : null}
			</div>

			{import.meta.env.DEV && (message || stack) ? (
				<details className="mt-4 w-full max-w-[560px] rounded-md border border-border bg-muted/40 p-3 text-left">
					<summary className="cursor-pointer text-xs font-medium text-muted-foreground">开发者信息</summary>
					{message ? <div className="mt-2 text-xs break-words text-muted-foreground">{message}</div> : null}
					{stack ? (
						<pre className="mt-2 max-h-[220px] overflow-auto rounded bg-background p-2 text-[11px] leading-relaxed whitespace-pre-wrap text-muted-foreground">
							{stack}
						</pre>
					) : null}
				</details>
			) : null}

			<div className="mt-6 flex flex-wrap items-center justify-center gap-2">
				{view.requiresLogin ? (
					onNavigateLogin ? (
						<Button size="sm" onClick={onNavigateLogin}>
							去登录
						</Button>
					) : (
						<Button asChild size="sm">
							{/* 错误树里 client router 不一定可用：用原生 <a> 全量导航 */}
							<a href="/login">去登录</a>
						</Button>
					)
				) : null}
				{!view.requiresLogin && view.retryable ? (
					<Button size="sm" onClick={handleRetry}>
						重试
					</Button>
				) : null}
				{view.kind === 'network' || view.kind === 'server' ? (
					<Button size="sm" variant="outline" onClick={() => window.location.reload()}>
						刷新页面
					</Button>
				) : null}
				{isGlobal && !hideHome ? (
					onNavigateHome ? (
						<Button size="sm" variant="outline" onClick={onNavigateHome}>
							返回首页
						</Button>
					) : (
						<Button asChild size="sm" variant="outline">
							<a href="/">返回首页</a>
						</Button>
					)
				) : null}
				{!isGlobal && !view.requiresLogin && !view.retryable ? (
					<Button size="sm" variant="outline" onClick={handleRetry}>
						重试
					</Button>
				) : null}
			</div>
		</div>
	);

	return (
		<div className={cn('flex w-full flex-col items-center justify-center', isGlobal ? 'min-h-[52vh] p-4' : 'p-6')}>
			{isGlobal ? (
				<Card className="w-full max-w-[640px] px-6 py-10 sm:px-8">{body}</Card>
			) : (
				<div className="flex w-full max-w-[620px] flex-col items-center rounded-lg border border-border bg-card px-6 py-8">
					{body}
				</div>
			)}
		</div>
	);
}
