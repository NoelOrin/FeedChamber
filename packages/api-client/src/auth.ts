import { getAdapter } from './adapter';
import { getApiBase } from './client';
import type { ApiResponse } from './types';

/**
 * Centralised auth state helpers.
 *
 * Storage model — cookie-only, no JS-readable tokens:
 *   - `master_session`: HttpOnly, holds the opaque session id minted by the
 *     server's SessionStore. It is the user's only credential; the server
 *     resolves it against `user_sessions` on every request, so revocation is
 *     "delete the row" rather than a blacklist lookup.
 *   - `master_auth`: non-HttpOnly "1"/"" flag so `isAuthenticated()` can answer
 *     without ever exposing a credential.
 *
 * There is no refresh token any more. `/auth/refresh` rotates the live session
 * (sliding expiry) and re-issues `master_session`; if the session has already
 * expired the call fails and the user must sign in again.
 *
 * On the server there is no cookie jar, so `client.ts` forwards the incoming
 * request's `Cookie` header explicitly (see `AuthAdapter.getSsrCookie`).
 */

const AUTH_FLAG_COOKIE = 'master_auth';
const AUTH_FLAG_COOKIE_PATH = '/';

/**
 * Check whether the user is "logged in".
 * Browser: presence of the non-HttpOnly `master_auth` flag cookie.
 * SSR: always false — server code should resolve the session via an API call
 * (or read the flag from forwarded headers) instead.
 */
export function isAuthenticated(): boolean {
	return getAdapter().readCookie(AUTH_FLAG_COOKIE) === '1';
}

/** Drop all client-readable auth state. HttpOnly cookies are cleared by the server's /auth/logout. */
export function clearAuthTokens(): void {
	const adapter = getAdapter();
	adapter.removeStorage('user_profile');
	adapter.removeCookie(AUTH_FLAG_COOKIE, AUTH_FLAG_COOKIE_PATH);
}

/** Dedup guard so concurrent 401s trigger exactly one refresh round-trip. */
let refreshPromise: Promise<boolean> | null = null;

/**
 * Rotate the live session via its HttpOnly `master_session` cookie, extending
 * the sliding expiry. Nothing is returned to JS — no credential is readable.
 */
export async function tryRefreshToken(): Promise<boolean> {
	if (refreshPromise) return refreshPromise;

	const promise = (async () => {
		try {
			const adapter = getAdapter();
			const headers: Record<string, string> = { 'Content-Type': 'application/json' };
			if (adapter.isSSR()) {
				const ssrCookie = await adapter.getSsrCookie();
				if (ssrCookie) headers['cookie'] = ssrCookie;
			}

			const res = await fetch(`${getApiBase()}/auth/refresh`, {
				method: 'POST',
				headers,
				credentials: adapter.fetchCredentials(),
				body: '{}',
			});
			if (!res.ok) return false;
			const json: ApiResponse<unknown> = await res.json();
			return json.code === 0 || json.code === 200;
		} catch {
			return false;
		} finally {
			refreshPromise = null;
		}
	})();

	refreshPromise = promise;
	return promise;
}

/** Redirect to login page (browser only). */
export function redirectToLogin() {
	if (typeof window !== 'undefined') {
		window.location.href = '/login';
	}
}
