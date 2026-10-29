import { describe, expect, it } from 'vitest';
import { ApiError, AuthError, HttpError, NetworkError } from './errors';

describe('ApiError', () => {
	it('extends Error with name, code, msg', () => {
		const err = new ApiError(1001, 'user not found');
		expect(err instanceof Error).toBe(true);
		expect(err.name).toBe('ApiError');
		expect(err.code).toBe(1001);
		expect(err.msg).toBe('user not found');
		expect(err.message).toBe('user not found');
	});
});

describe('HttpError', () => {
	it('extends Error with name and status', () => {
		const err = new HttpError(404, 'Not Found');
		expect(err instanceof Error).toBe(true);
		expect(err.name).toBe('HttpError');
		expect(err.status).toBe(404);
		expect(err.message).toBe('Not Found');
	});
});

describe('AuthError', () => {
	it('defaults message to Unauthorized', () => {
		const err = new AuthError();
		expect(err instanceof Error).toBe(true);
		expect(err.name).toBe('AuthError');
		expect(err.message).toBe('Unauthorized');
	});
	it('accepts custom message', () => {
		expect(new AuthError('Token expired').message).toBe('Token expired');
	});
});

describe('NetworkError', () => {
	it('extends Error with name and message', () => {
		const err = new NetworkError('timeout');
		expect(err instanceof Error).toBe(true);
		expect(err.name).toBe('NetworkError');
		expect(err.message).toBe('timeout');
	});
});
