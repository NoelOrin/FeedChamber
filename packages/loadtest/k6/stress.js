import { ABORT_THRESHOLDS, rampStages, TREND_STATS } from './lib/config.js';
import { createMetrics, seedPostIds } from './lib/endpoints.js';
import { summaryHandler } from './lib/summary.js';
import { readIteration } from './lib/workload.js';

// 千级阶梯：0 → 10% → 30% → 50% → 75% → 100% × MAX_VUS（默认 1000），
// 顶点保持 HOLD_SECONDS 后降载。错误率或断言通过率越界时 abortOnFail 自动停手。
export const options = {
	scenarios: {
		ramp: {
			executor: 'ramping-vus',
			startVUs: 0,
			stages: rampStages(),
			gracefulRampDown: '30s',
			gracefulStop: '30s',
		},
	},
	thresholds: {
		...ABORT_THRESHOLDS,
		http_req_duration: ['p(95)<3000', 'p(99)<8000'],
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

export const handleSummary = summaryHandler({ scenario: 'stress', note: '千级阶梯加压' });
