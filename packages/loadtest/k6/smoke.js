import { sleep } from 'k6';

import { TREND_STATS } from './lib/config.js';
import { createMetrics, ENDPOINTS, hit, seedPostIds } from './lib/endpoints.js';
import { summaryHandler } from './lib/summary.js';

// 压测前的正确性门：1 VU 把所有端点各打两轮，任何端点不 200 / code≠0 都判失败。
// 先跑它，再跑压力场景，避免把 404 当成"性能问题"。
export const options = {
	scenarios: {
		sanity: {
			executor: 'shared-iterations',
			vus: 1,
			iterations: 2,
			maxDuration: '2m',
		},
	},
	thresholds: {
		checks: ['rate==1'],
	},
	summaryTrendStats: TREND_STATS,
};

const metrics = createMetrics();

export function setup() {
	return { postIds: seedPostIds() };
}

export default function (ctx) {
	for (const endpoint of ENDPOINTS) {
		hit(endpoint, ctx, metrics);
		sleep(0.1);
	}
}

export const handleSummary = summaryHandler({ scenario: 'smoke', note: '端点可用性自检' });
