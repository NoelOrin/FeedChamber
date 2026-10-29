export class ApiError extends Error {
	readonly code: number;
	readonly msg: string;

	constructor(code: number, msg: string) {
		super(msg);
		this.name = 'ApiError';
		this.code = code;
		this.msg = msg;
	}
}

export class HttpError extends Error {
	readonly status: number;

	constructor(status: number, message: string) {
		super(message);
		this.name = 'HttpError';
		this.status = status;
	}
}

export class AuthError extends Error {
	constructor(message = 'Unauthorized') {
		super(message);
		this.name = 'AuthError';
	}
}

export class NetworkError extends Error {
	constructor(message: string) {
		super(message);
		this.name = 'NetworkError';
	}
}

export type AppError = ApiError | HttpError | AuthError | NetworkError;
