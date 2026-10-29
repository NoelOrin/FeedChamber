/**
 * Auth adapter abstraction.
 *
 * Isolates environment-specific auth plumbing (cookie access, token storage,
 * fetch credentials) so core auth.ts / client.ts stay framework-agnostic.
 * Each frontend registers its own adapter at boot:
 *
 *   - packages/api-client/src/adapters/web/       — SolidJS + TanStack Start
 *   - packages/api-client/src/adapters/dashboard/ — React + Vite SPA
 *
 * If no adapter is registered, `defaultAdapter` (vanilla browser, same-origin
 * cookies via proxy, SSR via `window === undefined` detection) is used.
 */

export interface AuthAdapter {
	/** Read a cookie value by name. null when cookies are unreadable (SSR). */
	readCookie(name: string): string | null;

	/** Write a cookie with the given max-age in seconds. */
	writeCookie(name: string, value: string, maxAgeSeconds: number, path: string): void;

	/** Remove a cookie (set Max-Age=0). */
	removeCookie(name: string, path: string): void;

	/** Read a value from the token storage backend (localStorage-equivalent). */
	readStorage(key: string): string | null;

	/** Persist a value into the token storage backend. */
	writeStorage(key: string, value: string): void;

	/** Remove a key from the token storage backend. */
	removeStorage(key: string): void;

	/**
	 * Fetch `credentials` value for requests targeting the API base URL.
	 * Always 'include' — same-origin proxy handles both browser and SSR.
	 */
	fetchCredentials(): RequestCredentials;

	/** True when running on the server (no browser globals available). */
	isSSR(): boolean;

	/**
	 * Get the SSR cookie header for the current request. Returns null on browser.
	 *
	 * May be async because React Server Components expose the incoming request
	 * headers only through an awaitable API (`next/headers#headers()`), and the
	 * value is per-request rather than process-global.
	 */
	getSsrCookie(): string | null | Promise<string | null>;

	/**
	 * Get the correlation id of the request currently being served, so it can be
	 * forwarded to the backend as `X-Request-ID` and both sides log the same
	 * value. Returns null on the browser and when no id is available.
	 *
	 * nginx sets `X-Request-ID` on every proxied request (including the SSR
	 * navigation that renders the page), so the SSR path always has one. The
	 * browser has none of its own; it simply omits the header and the backend
	 * generates a trace id as usual.
	 */
	getSsrRequestId(): string | null | Promise<string | null>;
}

/** Default adapter: vanilla browser + same-origin proxy + generic SSR fallback. */
export const defaultAdapter: AuthAdapter = {
	readCookie(name) {
		if (typeof document === 'undefined') return null;
		const match = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`));
		return match ? decodeURIComponent(match[1]) : null;
	},
	writeCookie(name, value, maxAge, path) {
		if (typeof document === 'undefined') return;
		document.cookie = `${name}=${encodeURIComponent(value)}; Max-Age=${maxAge}; Path=${path}`;
	},
	removeCookie(name, path) {
		if (typeof document === 'undefined') return;
		document.cookie = `${name}=; Max-Age=0; Path=${path}`;
	},
	readStorage(key) {
		if (typeof localStorage === 'undefined') return null;
		return localStorage.getItem(key);
	},
	writeStorage(key, value) {
		if (typeof localStorage === 'undefined') return;
		localStorage.setItem(key, value);
	},
	removeStorage(key) {
		if (typeof localStorage === 'undefined') return;
		localStorage.removeItem(key);
	},
	fetchCredentials() {
		return 'include';
	},
	isSSR() {
		return typeof window === 'undefined';
	},
	getSsrCookie() {
		return null;
	},
	getSsrRequestId() {
		return null;
	},
};

let current: AuthAdapter = defaultAdapter;

/** Replace the active adapter. Pass `undefined` to restore the default. */
export function registerAdapter(adapter: AuthAdapter | undefined): void {
	current = adapter ?? defaultAdapter;
}

/** Get the active adapter (never null — falls back to default). */
export function getAdapter(): AuthAdapter {
	return current;
}
