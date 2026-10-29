import { setApiBaseUrl } from '@orinforum/api-client';
import { registerWebAdapter } from '@orinforum/api-client/adapters/web';
import { headers } from 'next/headers';

const API_PREFIX = '/api/v1';

/**
 * Wires the shared api-client request layer to vinext's per-request headers.
 * Must be imported before any service call on the server.
 */
registerWebAdapter({
	readRequestCookie: async () => (await headers()).get('cookie'),
	// nginx stamps every proxied request with X-Request-ID. Forwarding it makes
	// the backend adopt it as the trace id, so this render and the API calls it
	// triggers share one id instead of each minting their own.
	readRequestId: async () => (await headers()).get('x-request-id'),
});

// Process-level, not per-request: the backend origin never varies by visitor, and
// Node's fetch cannot resolve a relative URL the way a proxied browser can.
if (typeof window === 'undefined') {
	setApiBaseUrl(`${process.env.API_BASE_URL ?? 'http://localhost:8080'}${API_PREFIX}`);
}
