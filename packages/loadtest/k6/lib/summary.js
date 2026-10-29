import { BASE_URL, KIND_FILTER, PROFILE } from './config.js';
import { ENDPOINTS } from './endpoints.js';

const DASH = '—';

function int(value) {
	if (value === undefined || value === null) {
		return DASH;
	}
	// 不用 toLocaleString：k6 的 goja 运行时对 Intl 支持不完整，手写千分位最稳
	return String(Math.round(value)).replace(/\B(?=(\d{3})+(?!\d))/g, ',');
}

function ms(value) {
	return value === undefined || value === null ? DASH : `${value.toFixed(0)} ms`;
}

function rate(value) {
	return value === undefined || value === null ? DASH : `${(value * 100).toFixed(2)}%`;
}

function table(headers, rows) {
	const lines = [`| ${headers.join(' | ')} |`, `| ${headers.map(() => '---').join(' | ')} |`];
	for (const row of rows) {
		lines.push(`| ${row.join(' | ')} |`);
	}
	return lines.join('\n');
}

function overallSection(data) {
	const metrics = data.metrics;
	const duration = metrics.http_req_duration ? metrics.http_req_duration.values : {};
	const runSeconds = (data.state.testRunDurationMs || 0) / 1000;
	const rows = [
		['总请求数', int(metrics.http_reqs?.values.count)],
		['吞吐', metrics.http_reqs ? `${metrics.http_reqs.values.rate.toFixed(1)} req/s` : DASH],
		['迭代次数', int(metrics.iterations?.values.count)],
		['并发峰值', int(metrics.vus_max && (metrics.vus_max.values.value ?? metrics.vus_max.values.max))],
		['平均延迟', ms(duration.avg)],
		['p90 延迟', ms(duration['p(90)'])],
		['p95 延迟', ms(duration['p(95)'])],
		['p99 延迟', ms(duration['p(99)'])],
		['最大延迟', ms(duration.max)],
		['HTTP 失败率', rate(metrics.http_req_failed?.values.rate)],
		['断言通过率', rate(metrics.checks?.values.rate)],
		['运行时长', `${runSeconds.toFixed(1)} s`],
	];
	return table(['指标', '值'], rows);
}

function endpointSection(data) {
	const runSeconds = (data.state.testRunDurationMs || 0) / 1000;
	const rows = [];
	for (const endpoint of ENDPOINTS) {
		const trend = data.metrics[`ep_${endpoint.id}_duration`];
		if (!trend) {
			continue;
		}
		const values = trend.values;
		const requests = data.metrics[`ep_${endpoint.id}_requests`];
		const failures = data.metrics[`ep_${endpoint.id}_failures`];
		const count = requests ? requests.values.count : 0;
		const failCount = failures ? failures.values.count : 0;
		rows.push([
			endpoint.id,
			int(count),
			runSeconds > 0 ? (count / runSeconds).toFixed(1) : DASH,
			ms(values.med),
			ms(values['p(95)']),
			ms(values['p(99)']),
			int(failCount),
			count > 0 ? rate(failCount / count) : DASH,
		]);
	}
	rows.sort((a, b) => Number.parseInt(b[1].replace(/,/g, ''), 10) - Number.parseInt(a[1].replace(/,/g, ''), 10));
	return table(['端点', '请求数', 'req/s', 'p50', 'p95', 'p99', '失败数', '失败率'], rows);
}

function statusSection(data) {
	const rows = [
		['429（限流）', int(data.metrics.status_429?.values.count ?? 0)],
		['403（被拦 / WAF 挑战）', int(data.metrics.status_403?.values.count ?? 0)],
		['其它 4xx', int(data.metrics.status_4xx_other?.values.count ?? 0)],
		['5xx（源站错误）', int(data.metrics.status_5xx?.values.count ?? 0)],
	];
	return table(['状态分类', '计数'], rows);
}

function thresholdSection(data) {
	const rows = [];
	for (const [metricName, metric] of Object.entries(data.metrics)) {
		if (!metric.thresholds) {
			continue;
		}
		for (const [expression, result] of Object.entries(metric.thresholds)) {
			rows.push([metricName, expression, result.ok ? 'PASS' : 'FAIL']);
		}
	}
	if (rows.length === 0) {
		return '_本次运行未配置阈值_';
	}
	return table(['指标', '阈值', '结果'], rows);
}

/**
 * k6 handleSummary 的实现：一次运行产出「人看的 md + 机器读的 json」两份报告到 reports/。
 */
export function summaryHandler(meta) {
	return (data) => {
		const stamp = new Date().toISOString().replace('T', '_').replace(/[:.]/g, '-').slice(0, 19);
		const base = `reports/${stamp}-${PROFILE}`;
		const body = [
			`# 压测报告 · ${meta.scenario || PROFILE}`,
			'',
			`- 目标：${BASE_URL}`,
			`- 场景：${meta.scenario || PROFILE}${meta.note ? `（${meta.note}）` : ''}`,
			`- 端点范围：${KIND_FILTER.length > 0 ? KIND_FILTER.join(' / ') : '全量混合（API + SSR + 重定向）'}`,
			`- 时间：${new Date().toISOString()}`,
			`- k6：${__ENV.K6_VERSION || '见运行命令'} `,
			'',
			'## 总览',
			'',
			overallSection(data),
			'',
			'## 分端点',
			'',
			endpointSection(data),
			'',
			'## 状态码分类',
			'',
			statusSection(data),
			'',
			'## 阈值',
			'',
			thresholdSection(data),
			'',
			'## 读法',
			'',
			'- `429` 是策略性限流、`403` 是 Cloudflare/WAF 拦截，二者与「源站被打爆」要分开看；',
			'- 若错误率或 p95 在某一级阶梯后陡升，那一级就是当前架构的拐点；',
			'- 本套场景全部为匿名只读请求，不写入任何数据。',
			'',
		].join('\n');

		return {
			[`${base}.md`]: body,
			[`${base}.json`]: JSON.stringify({ meta, metrics: data.metrics }, null, 2),
		};
	};
}
