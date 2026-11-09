import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// clearAuthTokens touches localStorage; stub the whole dependency so the test
// only observes SSEClient's error-handling behaviour.
vi.mock('@feedchamber/api-client', () => ({ clearAuthTokens: vi.fn() }));

import { SSEClient } from './client';

const mockFetch = vi.fn();

beforeEach(() => {
	vi.stubGlobal('fetch', mockFetch);
});

afterEach(() => {
	vi.unstubAllGlobals();
	mockFetch.mockReset();
});

/** A non-ok HTTP response carrying a unified error envelope. */
function errorResponse(status: number, code: number, msg: string): Response {
	return {
		ok: false,
		status,
		json: async () => ({ code, msg, data: null }),
	} as unknown as Response;
}

/** Drive one connect() attempt without letting the client actually retry. */
async function connectOnce(opts: { onError: (e: { code: number; msg: string }) => void }) {
	mockFetch.mockResolvedValue(errorResponse(503, 5005, '站点维护中，请稍后再试'));
	const client = new SSEClient({
		url: '/api/v1/notifications/stream',
		method: 'POST',
		withCredentials: true,
		onData: () => {},
		onError: opts.onError,
		// Prevent the scheduled reconnect from firing during the test.
		maxRetries: 1,
	});
	client.start();
	// Let the async fetch/response.json() microtasks settle.
	await vi.waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(1));
	client.stop();
	return client;
}

describe('SSEClient non-OK responses', () => {
	it('surfaces 5xxx server errors (e.g. maintenance 5005) via onError', async () => {
		const onError = vi.fn();
		await connectOnce({ onError });
		await vi.waitFor(() => expect(onError).toHaveBeenCalledTimes(1));
		expect(onError).toHaveBeenCalledWith({ code: 5005, msg: '站点维护中，请稍后再试' });
	});

	it('still surfaces 4xxx client errors via onError', async () => {
		const onError = vi.fn();
		mockFetch.mockResolvedValue(errorResponse(403, 3001, '无权限'));
		const client = new SSEClient({
			url: '/api/v1/notifications/stream',
			method: 'POST',
			withCredentials: true,
			onData: () => {},
			onError,
			maxRetries: 1,
		});
		client.start();
		await vi.waitFor(() => expect(onError).toHaveBeenCalledTimes(1));
		expect(onError).toHaveBeenCalledWith({ code: 3001, msg: '无权限' });
		client.stop();
	});
});
