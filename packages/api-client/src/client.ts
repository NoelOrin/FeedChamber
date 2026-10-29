import { getAdapter } from './adapter';
import { clearAuthTokens, redirectToLogin, tryRefreshToken } from './auth';
import { ApiError, AuthError, HttpError, NetworkError } from './errors';
import type { ApiResponse, PaginatedResponse } from './types';

let BASE = '/api/v1';

/** Override the API base URL (e.g. for SSR where relative URLs fail in Node.js fetch). */
export function setApiBaseUrl(url: string) {
	BASE = url;
}

/** Get the current API base URL. Used by auth.ts for refresh URL construction. */
export function getApiBase(): string {
	return BASE;
}

const DEFAULT_TIMEOUT_MS = 3000;

async function fetchWithTimeout(url: string, options: RequestInit = {}): Promise<Response> {
	const controller = new AbortController();
	const timeoutId = setTimeout(() => controller.abort(), DEFAULT_TIMEOUT_MS);
	const existingSignal = options.signal;
	if (existingSignal) {
		existingSignal.addEventListener('abort', () => controller.abort(), { once: true });
	}
	try {
		return await fetch(url, { ...options, signal: controller.signal });
	} finally {
		clearTimeout(timeoutId);
	}
}

async function authFetch(path: string, options: RequestInit): Promise<Response> {
	const adapter = getAdapter();
	const isFormData = typeof FormData !== 'undefined' && options.body instanceof FormData;
	const headers: Record<string, string> = {
		...(isFormData ? {} : { 'Content-Type': 'application/json' }),
		...(options.headers as Record<string, string>),
	};

	const isSSR = adapter.isSSR();

	// BASE is relative ('/api/v1') in the browser, where the dev/proxy layer routes
	// it to the backend, and absolute on the server, where `setApiBaseUrl` is called
	// once at boot because Node fetch cannot resolve a relative URL.
	const url = `${BASE}${path}`;

	// Node.js fetch has no cookie jar, so the incoming request's cookies must be
	// forwarded explicitly to keep the HttpOnly session credential attached.
	if (isSSR) {
		const ssrCookie = await adapter.getSsrCookie();
		if (ssrCookie) headers['cookie'] = ssrCookie;

		// Same reasoning for the correlation id: nginx assigned one to the SSR
		// request that is rendering this page, so pass it on and let the backend
		// reuse it as the trace id. Without this the SSR render and the API call it
		// triggers log two unrelated ids and cannot be joined.
		const requestId = await adapter.getSsrRequestId();
		if (requestId) headers['X-Request-ID'] = requestId;
	}

	const fetchOpts: RequestInit = {
		...options,
		headers,
		credentials: adapter.fetchCredentials(),
	};

	let res: Response;
	try {
		res = await fetchWithTimeout(url, fetchOpts);
	} catch (err) {
		if (err instanceof DOMException && err.name === 'AbortError') {
			throw new NetworkError(`Request timed out: ${path}`);
		}
		throw new NetworkError(err instanceof Error ? err.message : 'Network request failed');
	}

	const isAuthEndpoint = path.startsWith('/auth/');

	if (res.status === 401 && !isAuthEndpoint) {
		const refreshed = await tryRefreshToken();
		if (refreshed) {
			// Both browser and SSR forward cookies automatically via credentials: 'include'.
			// New HttpOnly cookie already set by server Set-Cookie; just retry.
			try {
				res = await fetchWithTimeout(url, { ...fetchOpts, headers });
			} catch (err) {
				if (err instanceof DOMException && err.name === 'AbortError') {
					throw new NetworkError(`Request timed out: ${path}`);
				}
				throw new NetworkError(err instanceof Error ? err.message : 'Network request failed');
			}
		}
		if (res.status === 401) {
			clearAuthTokens();
			redirectToLogin();
			throw new AuthError();
		}
	}

	if (!res.ok) {
		const body = await res.json().catch(() => null);
		throw new HttpError(res.status, body?.msg || `HTTP ${res.status}: ${res.statusText}`);
	}

	return res;
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
	const res = await authFetch(path, options);
	const json: ApiResponse<T> = await res.json();

	if (json.code !== 0 && json.code !== 200) {
		throw new ApiError(json.code, json.msg || 'request failed');
	}
	return json.data as T;
}

async function requestPaginated<T>(path: string, options: RequestInit = {}): Promise<{ data: T; total: number }> {
	const res = await authFetch(path, options);
	const json: PaginatedResponse<T> = await res.json();

	if (json.code !== 0 && json.code !== 200) {
		throw new ApiError(json.code, json.msg || 'request failed');
	}
	if (!json.data) {
		throw new ApiError(json.code, 'response data is null');
	}
	return {
		data: json.data.items as T,
		total: json.data.total,
	};
}

export function get<T>(path: string): Promise<T> {
	return request<T>(path, { method: 'GET' });
}

export function post<T>(path: string, body?: unknown): Promise<T> {
	return request<T>(path, {
		method: 'POST',
		body: body !== undefined ? JSON.stringify(body) : undefined,
	});
}

export async function postBlob(path: string, body?: unknown): Promise<Blob> {
	const res = await authFetch(path, {
		method: 'POST',
		body: body !== undefined ? JSON.stringify(body) : undefined,
	});
	return res.blob();
}

export function put<T>(path: string, body?: unknown): Promise<T> {
	return request<T>(path, {
		method: 'PUT',
		body: body !== undefined ? JSON.stringify(body) : undefined,
	});
}

export function patch<T>(path: string, body?: unknown): Promise<T> {
	return request<T>(path, {
		method: 'PATCH',
		body: body !== undefined ? JSON.stringify(body) : undefined,
	});
}

export function del<T>(path: string, body?: unknown): Promise<T> {
	return request<T>(path, {
		method: 'DELETE',
		body: body !== undefined ? JSON.stringify(body) : undefined,
	});
}

export function postPaginated<T>(path: string, body?: unknown): Promise<{ data: T; total: number }> {
	return requestPaginated<T>(path, {
		method: 'POST',
		body: body !== undefined ? JSON.stringify(body) : undefined,
	});
}

export function postForm<T>(path: string, form: FormData): Promise<T> {
	// Let the browser set Content-Type automatically (multipart/form-data with boundary).
	// Do NOT pass Content-Type: application/json — FormData requires multipart.
	return request<T>(path, { method: 'POST', body: form, headers: {} as Record<string, string> });
}
