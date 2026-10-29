/** Universal error view resolver. Works via duck-typing so it handles both
 *  api-client errors and web's ~/lib/api/errors without cross-package instanceof. */

export type ErrorViewKind =
	| 'auth'
	| 'forbidden'
	| 'banned'
	| 'validation'
	| 'not_found'
	| 'conflict'
	| 'rate_limited'
	| 'payload_too_large'
	| 'server'
	| 'network'
	| 'frontend'
	| 'unknown';

export interface ErrorView {
	kind: ErrorViewKind;
	code?: number;
	status?: number;
	title: string;
	description: string;
	retryable: boolean;
	/** whether the primary action should be "go login" */
	requiresLogin: boolean;
}

function hasCode(x: unknown): x is { code: number; message?: string; msg?: string } {
	return typeof x === 'object' && x !== null && typeof (x as Record<string, unknown>).code === 'number';
}

function hasStatus(x: unknown): x is { status: number; message?: string } {
	return typeof x === 'object' && x !== null && typeof (x as Record<string, unknown>).status === 'number';
}

function errName(x: unknown): string {
	return typeof x === 'object' && x !== null && 'name' in x ? String((x as Record<string, unknown>).name) : '';
}

function errMessage(x: unknown): string {
	if (x instanceof Error) return x.message;
	if (typeof x === 'object' && x !== null) {
		const m = (x as Record<string, unknown>).message ?? (x as Record<string, unknown>).msg;
		if (typeof m === 'string') return m;
	}
	if (typeof x === 'string') return x;
	return '';
}

export function resolveErrorView(err: unknown): ErrorView {
	const msg = errMessage(err) || '';

	// Backend ApiError with numeric code — highest priority
	if (hasCode(err)) {
		const code = (err as { code: number }).code;
		switch (code) {
			case 1001:
			case 1002:
			case 1003:
			case 1014:
				return {
					kind: 'auth',
					code,
					title: '登录已过期',
					description: msg || '登录状态已失效，请重新登录后再试。',
					retryable: false,
					requiresLogin: true,
				};
			case 1010:
				return {
					kind: 'auth',
					code,
					title: '密码错误',
					description: msg || '密码不正确，请检查后重试。',
					retryable: true,
					requiresLogin: false,
				};
			case 1011:
				return {
					kind: 'not_found',
					code,
					title: '用户不存在',
					description: msg || '该用户不存在或已被删除。',
					retryable: false,
					requiresLogin: false,
				};
			case 1012:
				return {
					kind: 'conflict',
					code,
					title: '用户名已存在',
					description: msg || '该用户名已被占用，请换一个。',
					retryable: false,
					requiresLogin: false,
				};
			case 1016:
				return {
					kind: 'conflict',
					code,
					title: '邮箱已存在',
					description: msg || '该邮箱已被注册。',
					retryable: false,
					requiresLogin: false,
				};
			case 1013:
				return {
					kind: 'forbidden',
					code,
					title: '无权限访问',
					description: msg || '你没有权限执行此操作。',
					retryable: false,
					requiresLogin: false,
				};
			case 1015:
				return {
					kind: 'banned',
					code,
					title: '账号已被封禁',
					description: msg || '你的账号已被封禁，请联系管理员。',
					retryable: false,
					requiresLogin: false,
				};
			case 2001:
				return {
					kind: 'validation',
					code,
					title: '参数错误',
					description: msg || '请求参数不正确，请检查输入。',
					retryable: true,
					requiresLogin: false,
				};
			case 3001:
				return {
					kind: 'not_found',
					code,
					title: '资源不存在',
					description: msg || '请求的资源不存在或已被删除。',
					retryable: false,
					requiresLogin: false,
				};
			case 3002:
				return {
					kind: 'conflict',
					code,
					title: '资源已存在',
					description: msg || '该资源已存在，请勿重复创建。',
					retryable: false,
					requiresLogin: false,
				};
			case 4001:
				return {
					kind: 'rate_limited',
					code,
					title: '操作太频繁',
					description: msg || '请求过于频繁，请稍后再试。',
					retryable: true,
					requiresLogin: false,
				};
			case 4002:
				return {
					kind: 'payload_too_large',
					code,
					title: '请求体过大',
					description: msg || '上传内容过大，请减小后重试。',
					retryable: false,
					requiresLogin: false,
				};
			case 5001:
			case 5004:
				return {
					kind: 'server',
					code,
					title: code === 5004 ? '请求超时' : '服务异常',
					description: msg || '服务器开小差了，请稍后重试。',
					retryable: true,
					requiresLogin: false,
				};
			default: {
				if (code >= 1000 && code < 2000)
					return {
						kind: 'auth',
						code,
						title: '认证失败',
						description: msg || '认证失败，请重新登录。',
						retryable: false,
						requiresLogin: true,
					};
				if (code >= 2000 && code < 3000)
					return {
						kind: 'validation',
						code,
						title: '参数错误',
						description: msg || '请求参数有误。',
						retryable: true,
						requiresLogin: false,
					};
				if (code >= 3000 && code < 4000)
					return {
						kind: 'not_found',
						code,
						title: '资源不存在',
						description: msg || '资源不存在。',
						retryable: false,
						requiresLogin: false,
					};
				if (code >= 4000 && code < 5000)
					return {
						kind: 'rate_limited',
						code,
						title: '请求受限',
						description: msg || '请求被限流，请稍后再试。',
						retryable: true,
						requiresLogin: false,
					};
				if (code >= 5000)
					return {
						kind: 'server',
						code,
						title: '服务异常',
						description: msg || '服务器异常，请稍后重试。',
						retryable: true,
						requiresLogin: false,
					};
				return {
					kind: 'unknown',
					code,
					title: '未知错误',
					description: msg || `未知错误（code=${code}）`,
					retryable: true,
					requiresLogin: false,
				};
			}
		}
	}

	// HTTP status errors
	if (hasStatus(err)) {
		const status = (err as { status: number }).status;
		if (status === 401)
			return {
				kind: 'auth',
				status,
				title: '未登录',
				description: msg || '请先登录后再继续操作。',
				retryable: false,
				requiresLogin: true,
			};
		if (status === 403)
			return {
				kind: 'forbidden',
				status,
				title: '无权限访问',
				description: msg || '你没有权限访问该资源。',
				retryable: false,
				requiresLogin: false,
			};
		if (status === 404)
			return {
				kind: 'not_found',
				status,
				title: '资源不存在',
				description: msg || '请求的页面或资源不存在。',
				retryable: false,
				requiresLogin: false,
			};
		if (status === 409)
			return {
				kind: 'conflict',
				status,
				title: '资源冲突',
				description: msg || '资源已存在或状态冲突。',
				retryable: false,
				requiresLogin: false,
			};
		if (status === 413)
			return {
				kind: 'payload_too_large',
				status,
				title: '请求体过大',
				description: msg || '请求内容过大。',
				retryable: false,
				requiresLogin: false,
			};
		if (status === 429)
			return {
				kind: 'rate_limited',
				status,
				title: '操作太频繁',
				description: msg || '请求过于频繁，请稍后再试。',
				retryable: true,
				requiresLogin: false,
			};
		if (status === 408)
			return {
				kind: 'server',
				status,
				title: '请求超时',
				description: msg || '请求超时，请检查网络后重试。',
				retryable: true,
				requiresLogin: false,
			};
		if (status >= 500)
			return {
				kind: 'server',
				status,
				title: '服务异常',
				description: msg || '服务器异常，请稍后重试。',
				retryable: true,
				requiresLogin: false,
			};
		return {
			kind: 'unknown',
			status,
			title: '请求失败',
			description: msg || `请求失败（HTTP ${status}）`,
			retryable: true,
			requiresLogin: false,
		};
	}

	const name = errName(err);
	if (
		name === 'NetworkError' ||
		name === 'TypeError' ||
		/network|fetch|timeout|Failed to fetch|Load failed/i.test(msg)
	) {
		// Heuristic: TypeError with fetch failure is typically a network error and has no code/status.
		if (name === 'NetworkError' || /network|fetch|timeout|Failed to fetch|Load failed/i.test(msg)) {
			return {
				kind: 'network',
				title: '网络异常',
				description: msg || '网络连接失败，请检查网络后重试。',
				retryable: true,
				requiresLogin: false,
			};
		}
	}
	if (name === 'AuthError' || name === 'AuthExpiredError') {
		return {
			kind: 'auth',
			title: '登录已过期',
			description: msg || '登录已过期，请重新登录。',
			retryable: false,
			requiresLogin: true,
		};
	}

	// Frontend runtime / render errors — no code/status, plain Error
	if (err instanceof Error) {
		return {
			kind: 'frontend',
			title: '页面出现错误',
			description: msg || '页面渲染时出现未预期错误，请重试或刷新。',
			retryable: true,
			requiresLogin: false,
		};
	}
	if (typeof err === 'string' && err.trim()) {
		return {
			kind: 'frontend',
			title: '页面出现错误',
			description: err,
			retryable: true,
			requiresLogin: false,
		};
	}

	return {
		kind: 'unknown',
		title: '未知错误',
		description: msg || '发生了未知错误，请重试。',
		retryable: true,
		requiresLogin: false,
	};
}
