import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { get, postPaginated } from './client';
import { ApiError, AuthError, HttpError, NetworkError } from './errors';

const mockFetch = vi.fn();

beforeEach(() => {
	vi.stubGlobal('fetch', mockFetch);
	vi.stubGlobal('localStorage', {
		getItem: vi.fn((key: string) => (key === 'access_token' ? 'test-token' : null)),
		setItem: vi.fn(),
		removeItem: vi.fn(),
	});
});

afterEach(() => {
	vi.unstubAllGlobals();
	mockFetch.mockReset();
});

function makeResponse(status: number, body: unknown): Response {
	return {
		ok: status >= 200 && status < 300,
		status,
		statusText: String(status),
		json: async () => body,
	} as unknown as Response;
}

describe('client typed errors', () => {
	it('throws ApiError when server returns code !== 0', async () => {
		mockFetch.mockResolvedValue(makeResponse(200, { code: 1001, msg: 'user not found', data: null }));
		await expect(get('/users/1')).rejects.toSatisfy(
			(e: unknown) =>
				e instanceof ApiError && (e as ApiError).code === 1001 && (e as ApiError).message === 'user not found',
		);
	});

	it('throws ApiError when requestPaginated gets null data', async () => {
		mockFetch.mockResolvedValue(makeResponse(200, { code: 0, msg: 'ok', data: null }));
		await expect(postPaginated('/posts', {})).rejects.toSatisfy(
			(e: unknown) => e instanceof ApiError && (e as ApiError).message === 'response data is null',
		);
	});

	it('throws HttpError on HTTP 500', async () => {
		mockFetch.mockResolvedValue(makeResponse(500, { msg: 'internal server error' }));
		await expect(get('/users')).rejects.toSatisfy(
			(e: unknown) => e instanceof HttpError && (e as HttpError).status === 500,
		);
	});

	it('throws HttpError on HTTP 403', async () => {
		mockFetch.mockResolvedValue(makeResponse(403, { msg: 'forbidden' }));
		await expect(get('/admin')).rejects.toSatisfy(
			(e: unknown) =>
				e instanceof HttpError && (e as HttpError).status === 403 && (e as HttpError).message === 'forbidden',
		);
	});

	it('throws AuthError on HTTP 401 with no refresh token', async () => {
		vi.stubGlobal('localStorage', {
			getItem: vi.fn(() => null),
			setItem: vi.fn(),
			removeItem: vi.fn(),
		});
		mockFetch.mockResolvedValue(makeResponse(401, {}));
		await expect(get('/users')).rejects.toSatisfy((e: unknown) => e instanceof AuthError);
	});

	it('throws NetworkError on fetch failure', async () => {
		mockFetch.mockRejectedValue(new TypeError('Failed to fetch'));
		await expect(get('/users')).rejects.toSatisfy((e: unknown) => e instanceof NetworkError);
	});

	it('throws NetworkError on request timeout (AbortError)', async () => {
		const abortError = new DOMException('signal is aborted', 'AbortError');
		mockFetch.mockRejectedValue(abortError);
		await expect(get('/users')).rejects.toSatisfy(
			(e: unknown) => e instanceof NetworkError && (e as NetworkError).message.includes('timed out'),
		);
	});
});
