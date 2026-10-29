import { check } from 'k6';
import http from 'k6/http';
import { Counter, Trend } from 'k6/metrics';

import { BASE_URL, HTTP_TIMEOUT, JSON_HEADERS, KIND_FILTER, USER_AGENT } from './config.js';

/**
 * 匿名只读端点目录 —— 压测面的唯一可信来源，调整负载组成只改这个数组。
 *
 * 字段：
 *   id      指标前缀，同时作为 k6 的 request name tag
 *   weight  相对权重（每次迭代被选中的概率 = weight / Σweight）
 *   kind    json（API 信封）/ html（SSR 页面）/ redirect（根路径 307）
 *   method  GET / POST（本项目压测只覆盖只读端点，不产生任何写入）
 *   path    (ctx) => 相对路径
 *   body    (ctx) => 请求体对象，POST 必填
 *
 * 端点可用性在压测前由 smoke.js 全量校验；这里只描述"应该长什么样"。
 */
const CATALOGUE = [
	{
		id: 'posts_list',
		weight: 35,
		kind: 'json',
		method: 'POST',
		path: () => '/api/v1/posts/list',
		body: () => ({ page: 1 + Math.floor(Math.random() * 5), pageSize: 20 }),
	},
	{
		id: 'post_detail',
		weight: 20,
		kind: 'json',
		method: 'GET',
		path: (ctx) => `/api/v1/posts/${ctx.postIds[Math.floor(Math.random() * ctx.postIds.length)]}`,
	},
	{
		id: 'timeline_page',
		weight: 10,
		kind: 'html',
		method: 'GET',
		path: () => '/timeline',
	},
	{
		id: 'spaces_page',
		weight: 5,
		kind: 'html',
		method: 'GET',
		path: () => '/spaces',
	},
	{
		id: 'root_redirect',
		weight: 5,
		kind: 'redirect',
		method: 'GET',
		path: () => '/',
	},
	{
		id: 'tags_list',
		weight: 4,
		kind: 'json',
		method: 'POST',
		path: () => '/api/v1/tags/list',
		body: () => ({}),
	},
	{
		id: 'spaces_tree',
		weight: 3,
		kind: 'json',
		method: 'POST',
		path: () => '/api/v1/spaces/tree',
		body: () => ({}),
	},
	{
		id: 'spaces_flat',
		weight: 3,
		kind: 'json',
		method: 'POST',
		path: () => '/api/v1/spaces/flat',
		body: () => ({}),
	},
	{
		id: 'settings_public',
		weight: 3,
		kind: 'json',
		method: 'POST',
		path: () => '/api/v1/settings/public',
		body: () => ({}),
	},
	{
		id: 'latest_rss',
		weight: 3,
		kind: 'html',
		method: 'GET',
		path: () => '/latest.rss',
	},
	{
		id: 'themes_list',
		weight: 2,
		kind: 'json',
		method: 'POST',
		path: () => '/api/v1/themes/list',
		body: () => ({}),
	},
	{
		id: 'badges_list',
		weight: 2,
		kind: 'json',
		method: 'POST',
		path: () => '/api/v1/badges/list',
		body: () => ({}),
	},
	{
		id: 'leaderboard_list',
		weight: 2,
		kind: 'json',
		method: 'POST',
		path: () => '/api/v1/leaderboard/list',
		body: () => ({}),
	},
	{
		id: 'announcements_published',
		weight: 2,
		kind: 'json',
		method: 'POST',
		path: () => '/api/v1/announcements/published',
		body: () => ({ page: 1, pageSize: 10 }),
	},
	{
		id: 'sitemap_posts',
		weight: 1,
		kind: 'html',
		method: 'GET',
		path: () => '/sitemap-posts.xml',
	},
];

// 过滤器只影响"压哪些"，不影响断言与权重口径；权重按过滤后的集合重新归一化。
export const ENDPOINTS = CATALOGUE.filter(
	(endpoint) => KIND_FILTER.length === 0 || KIND_FILTER.includes(endpoint.kind),
);

/**
 * 形状断言。API 必须 200 + 信封 code=0（只判 200 会把 4xx 信封漏成"成功"）；
 * 页面必须 2xx 且有实质正文；根路径只做重定向，不跟随。
 */
export const EXPECT = {
	json(res) {
		if (res.status !== 200) {
			return false;
		}
		try {
			const body = res.json();
			return body !== null && typeof body === 'object' && body.code === 0;
		} catch (_err) {
			return false;
		}
	},
	html(res) {
		return res.status === 200 && typeof res.body === 'string' && res.body.length > 200;
	},
	redirect(res) {
		return res.status === 307 || res.status === 301 || res.status === 302 || res.status === 200;
	},
};

const TOTAL_WEIGHT = ENDPOINTS.reduce((sum, endpoint) => sum + endpoint.weight, 0);

export function pickEndpoint() {
	let cursor = Math.random() * TOTAL_WEIGHT;
	for (const endpoint of ENDPOINTS) {
		cursor -= endpoint.weight;
		if (cursor <= 0) {
			return endpoint;
		}
	}
	return ENDPOINTS[ENDPOINTS.length - 1];
}

/**
 * 每个端点一个 Trend + 一个失败计数器，summary.js 据此出分端点表。
 * 状态码单独计数：429/403 是"被拦"而不是"被打爆"，报告里必须能区分。
 */
export function createMetrics() {
	const metrics = {
		status429: new Counter('status_429'),
		status403: new Counter('status_403'),
		status4xxOther: new Counter('status_4xx_other'),
		status5xx: new Counter('status_5xx'),
	};
	for (const endpoint of ENDPOINTS) {
		metrics[endpoint.id] = {
			requests: new Counter(`ep_${endpoint.id}_requests`),
			duration: new Trend(`ep_${endpoint.id}_duration`, true),
			failures: new Counter(`ep_${endpoint.id}_failures`),
		};
	}
	return metrics;
}

export function hit(endpoint, ctx, metrics) {
	const isJson = endpoint.kind === 'json' || endpoint.method === 'POST';
	const params = {
		headers: isJson
			? JSON_HEADERS
			: { Accept: endpoint.kind === 'html' ? 'text/html,application/xhtml+xml' : '*/*', 'User-Agent': USER_AGENT },
		timeout: HTTP_TIMEOUT,
		tags: { name: endpoint.id },
	};
	if (endpoint.kind === 'redirect') {
		params.redirects = 0;
	} else if (isJson) {
		params.headers = { ...JSON_HEADERS, 'User-Agent': USER_AGENT };
	}

	const body = endpoint.body ? JSON.stringify(endpoint.body(ctx)) : null;
	const res = http.request(endpoint.method, BASE_URL + endpoint.path(ctx), body, params);

	metrics[endpoint.id].requests.add(1);
	metrics[endpoint.id].duration.add(res.timings.duration);
	if (res.status === 429) {
		metrics.status429.add(1);
	} else if (res.status === 403) {
		metrics.status403.add(1);
	} else if (res.status >= 500) {
		metrics.status5xx.add(1);
	} else if (res.status >= 400) {
		metrics.status4xxOther.add(1);
	}

	const ok = EXPECT[endpoint.kind](res);
	check(res, { [`${endpoint.id} ok`]: () => ok }, { name: endpoint.id });
	if (!ok) {
		metrics[endpoint.id].failures.add(1);
	}
	return res;
}

/**
 * setup 阶段取一批真实帖子 ID，详情页压测才有意义。
 * 目标不可达 / 返回形状不对时直接抛错，避免打出"全 0 请求"的假报告。
 */
export function seedPostIds() {
	const res = http.post(`${BASE_URL}/api/v1/posts/list`, JSON.stringify({ page: 1, pageSize: 50 }), {
		headers: { ...JSON_HEADERS, 'User-Agent': USER_AGENT },
		timeout: HTTP_TIMEOUT,
		tags: { name: 'setup' },
	});
	if (res.status !== 200) {
		throw new Error(`setup: ${BASE_URL}/api/v1/posts/list 返回 ${res.status}，目标不可压`);
	}
	const payload = res.json();
	if (payload?.code !== 0 || !payload.data || !Array.isArray(payload.data.items)) {
		throw new Error('setup: /api/v1/posts/list 响应形状不符（缺少 data.items）');
	}
	const ids = payload.data.items.map((item) => item.id).filter(Boolean);
	if (ids.length === 0) {
		throw new Error('setup: 目标库里没有可压的帖子，先造一条数据或换 BASE_URL');
	}
	return ids;
}
