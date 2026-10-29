import { sleep } from 'k6';

import { hit, pickEndpoint } from './endpoints.js';

/**
 * 一次"用户阅读"迭代：按权重随机命中一个端点，再停留 0.2~0.7s 模拟思考时间。
 * reads.js 与 stress.js 共用，保证基线与阶梯压测的负载组成完全一致。
 */
export function readIteration(ctx, metrics) {
	hit(pickEndpoint(), ctx, metrics);
	sleep(0.2 + Math.random() * 0.5);
}
