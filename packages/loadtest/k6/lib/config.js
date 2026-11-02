// 压测目标与档位。全部可用 k6 的 `-e` 覆盖，例如：
//   k6 run -e BASE_URL=https://dev-forum.noelorin.top -e MAX_VUS=1000 k6/stress.js
export const BASE_URL = (__ENV.BASE_URL || 'https://dev-forum.noelorin.top').replace(/\/+$/, '');

// 报告文件名里的档位后缀：full / smoke / baseline
export const PROFILE = __ENV.PROFILE || 'full';

export const HTTP_TIMEOUT = __ENV.HTTP_TIMEOUT || '15s';

// 端点类型过滤：KINDS=json 只压 API 面，KINDS=html 只压 SSR/静态面，默认全量混合。
// 用来把「API 能扛多少」和「SSR 页面能扛多少」分开测，避免一个慢端点掩盖另一个的能力上限。
export const KIND_FILTER = (__ENV.KINDS || '')
	.split(',')
	.map((value) => value.trim())
	.filter(Boolean);

// 压测流量在服务端日志里可辨识（不伪装成真实用户 UA）
export const USER_AGENT = __ENV.USER_AGENT || 'Feed Chamber-LoadTest/0.1 (k6)';

// 报告里要 p50/p95/p99 同口径出现：k6 的 summaryTrendStats 决定 values 里有哪些分位
export const TREND_STATS = ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'];

export const JSON_HEADERS = {
	'Content-Type': 'application/json',
	Accept: 'application/json',
};

export function positiveInt(raw, fallback) {
	const parsed = Number.parseInt(raw === undefined ? '' : raw, 10);
	return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

/**
 * 千级阶梯：10% → 30% → 50% → 75% → 100% × MAX_VUS，每级 LEVEL_SECONDS，
 * 顶点保持 HOLD_SECONDS 后 30s 降载。唯一的加压曲线定义，reads/stress 共用。
 */
export function rampStages() {
	const max = positiveInt(__ENV.MAX_VUS, 1000);
	const level = __ENV.LEVEL_SECONDS || '60s';
	const hold = __ENV.HOLD_SECONDS || '120s';
	const ladder = [0.1, 0.3, 0.5, 0.75, 1].map((factor) => Math.max(1, Math.round(max * factor)));
	return [
		{ duration: '30s', target: Math.min(50, ladder[0]) },
		...ladder.map((target) => ({ duration: level, target })),
		{ duration: hold, target: max },
		{ duration: '30s', target: 0 },
	];
}

/**
 * 中止保护：错误率 / 断言通过率越界的阈值交给 k6 的 abortOnFail，
 * 让源站被打挂时自动停手，而不是继续轰。delayAbortEval 保证先拿到一段有效样本。
 */
export const ABORT_THRESHOLDS = {
	http_req_failed: [{ threshold: 'rate<0.02', abortOnFail: true, delayAbortEval: '30s' }],
	checks: [{ threshold: 'rate>0.97', abortOnFail: true, delayAbortEval: '30s' }],
};
