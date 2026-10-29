/**
 * Web (React + vinext / Next App Router) auth adapter.
 *
 * Unlike the dashboard SPA, the web app renders on the server, where the
 * incoming request's cookies are only reachable through an awaitable API. This
 * package stays framework-agnostic, so the host supplies the reader:
 *
 *   registerWebAdapter({
 *     readRequestCookie: async () => (await headers()).get('cookie'),
 *   })
 *
 * The backend origin is process-level config, not per-request — set it once with
 * `setApiBaseUrl()` rather than through the adapter.
 */
import { type AuthAdapter, defaultAdapter, registerAdapter } from '../../adapter';

export interface WebSsrSources {
	/** Read the raw `Cookie` header of the request currently being rendered. */
	readRequestCookie(): string | null | Promise<string | null>;

	/**
	 * Read the `X-Request-ID` header of the request currently being rendered.
	 *
	 * nginx assigns it to every proxied request, so the SSR render already has
	 * one; forwarding it makes the backend log the same trace id that nginx
	 * logged, instead of minting a fresh one and leaving two unrelated ids.
	 */
	readRequestId(): string | null | Promise<string | null>;
}

export function createWebAdapter(sources: WebSsrSources): AuthAdapter {
	return {
		...defaultAdapter,
		isSSR: () => typeof window === 'undefined',
		fetchCredentials: () => 'include',
		getSsrCookie: () => sources.readRequestCookie(),
		getSsrRequestId: () => sources.readRequestId(),
	};
}

export function registerWebAdapter(sources: WebSsrSources): AuthAdapter {
	const adapter = createWebAdapter(sources);
	registerAdapter(adapter);
	return adapter;
}
