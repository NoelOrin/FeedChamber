import { ABORT_THRESHOLDS, positiveInt, TREND_STATS } from './lib/config.js';
import { createMetrics, seedPostIds } from './lib/endpoints.js';
import { summaryHandler } from './lib/summary.js';
import { readIteration } from './lib/workload.js';

// 固定并发基线：用来拿"这台机器 + 这条网络 + 这个源站"的稳态指标，
// 也是改动压测脚本后最快的回归验证（默认 50 VU × 60s）。
export const options = {
	scenarios: {
		read_mix: {
			executor: 'constant-vus',
			vus: positiveInt(__ENV.VUS, 50),
			duration: __ENV.DURATION || '60s',
			gracefulStop: '15s',
		},
	},
	thresholds: {
		...ABORT_THRESHOLDS,
		http_req_duration: ['p(95)<3000'],
	},
	summaryTrendStats: TREND_STATS,
};

const metrics = createMetrics();

export function setup() {
	return { postIds: seedPostIds() };
}

export default function (ctx) {
	readIteration(ctx, metrics);
}

export const handleSummary = summaryHandler({ scenario: 'baseline', note: '固定并发基线' });
