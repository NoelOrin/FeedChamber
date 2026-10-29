import { resolveErrorView } from '@orinforum/api-client';
import { Component, type ErrorInfo, type ReactNode } from 'react';
import { Button } from '~/components/ui/button';
import ErrorFallback from './ErrorFallback';

interface GlobalProps {
	children: ReactNode;
	fallback?: (error: Error, reset: () => void) => ReactNode;
}

interface GlobalState {
	error: Error | null;
}

/**
 * Global error boundary: app-level catch-all for render/lifecycle crashes.
 * Uses resolveErrorView so backend code and frontend runtime errors render differently.
 * Keeps any custom fallback passed by caller; otherwise renders ErrorFallback variant=global.
 */
export class GlobalErrorBoundary extends Component<GlobalProps, GlobalState> {
	state: GlobalState = { error: null };

	static getDerivedStateFromError(error: Error): GlobalState {
		return { error };
	}

	componentDidCatch(error: Error, info: ErrorInfo) {
		const view = resolveErrorView(error);
		console.error(`[GlobalErrorBoundary:${view.kind}]`, error, info.componentStack);
	}

	reset = () => this.setState({ error: null });

	render() {
		const { error } = this.state;
		if (!error) return this.props.children;
		if (this.props.fallback) return this.props.fallback(error, this.reset);
		return <ErrorFallback error={error} reset={this.reset} variant="global" />;
	}
}

interface SectionProps {
	children: ReactNode;
	resetKeys?: unknown[];
	fallback?: (error: Error, reset: () => void) => ReactNode;
	onError?: (error: unknown) => void;
	title?: string;
}

interface SectionState {
	error: Error | null;
	prevKeys: string;
}

/**
 * Section (local) error boundary: isolates failures to a widget/section so
 * the rest of the page stays interactive. Resets when resetKeys change.
 */
export class SectionErrorBoundary extends Component<SectionProps, SectionState> {
	state: SectionState = { error: null, prevKeys: this.keys() };

	private keys(): string {
		try {
			return JSON.stringify(this.props.resetKeys ?? null);
		} catch {
			return String(this.props.resetKeys);
		}
	}

	static getDerivedStateFromError(error: Error): Partial<SectionState> {
		return { error };
	}

	static getDerivedStateFromProps(props: SectionProps, state: SectionState): Partial<SectionState> | null {
		let nextKeys = '';
		try {
			nextKeys = JSON.stringify(props.resetKeys ?? null);
		} catch {
			nextKeys = String(props.resetKeys);
		}
		if (nextKeys !== state.prevKeys) return { error: null, prevKeys: nextKeys };
		return null;
	}

	componentDidCatch(error: Error, info: ErrorInfo) {
		const view = resolveErrorView(error);
		this.props.onError?.(error);
		console.error(`[SectionErrorBoundary:${view.kind}]`, error, info.componentStack);
	}

	reset = () => this.setState({ error: null });

	render() {
		const { error } = this.state;
		if (!error) return this.props.children;
		if (this.props.fallback) return this.props.fallback(error, this.reset);
		return (
			<ErrorFallback
				error={error}
				reset={this.reset}
				variant="section"
				onRetry={this.reset}
				onNavigateHome={() => window.location.assign('/')}
				onNavigateLogin={() => window.location.assign('/login')}
			/>
		);
	}
}

/** Back-compat: default export keeps existing imports working. */
export class ErrorBoundary extends GlobalErrorBoundary {}

// Keep previous file's simple fallback available for tests/docs if needed.
export function LegacyFallback({ error, reset }: { error: Error; reset: () => void }) {
	return (
		<div className="flex min-h-[50vh] flex-col items-center justify-center gap-3 px-6 text-center">
			<h1 className="text-lg font-semibold tracking-tight text-foreground">页面出错了</h1>
			<p className="text-muted-foreground max-w-[440px] text-[13px]">发生了未预期的错误，您可以重试或刷新页面。</p>
			<div className="flex flex-wrap items-center justify-center gap-2">
				<Button size="sm" onClick={reset}>
					重试
				</Button>
				<Button variant="outline" size="sm" onClick={() => window.location.reload()}>
					刷新页面
				</Button>
			</div>
			{import.meta.env.DEV ? (
				<code className="bg-destructive/10 text-destructive rounded px-1.5 py-0.5 text-xs break-words whitespace-pre-wrap">
					{error.message}
				</code>
			) : null}
		</div>
	);
}
