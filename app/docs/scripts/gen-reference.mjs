#!/usr/bin/env node
/**
 * 从源码生成文档站的参考页数据。
 *
 * 生成三份"不会漂移"的参考材料，写给 `src/data/reference.json`：
 *   1. endpoints —— 读仓库根的 openapi.json（make swag 的产物），313 个 path / 314 个操作
 *   2. settings  —— 解析 internal/model/setting.go 的 DefaultSettings 映射
 *   3. metrics   —— 解析 internal/metrics/*.go 的 prometheus Name/Help 对
 *
 * 为什么不手写：手写的端点清单在第一次改路由后就开始骗人。这份 JSON 是构建产物，
 * 由 predev/prebuild 调用；`--check` 给 CI 用，内容和源码不一致就退出非零。
 */
import { mkdir, readdir, readFile, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const docsRoot = join(here, '..');
const repoRoot = join(docsRoot, '..', '..');
const outFile = join(docsRoot, 'src', 'data', 'reference.json');

/** 应用根路径：不挂在 /api/v1 下（爬虫面、健康检查、OAuth/OIDC 协议端点、MCP）。 */
const ROOT_PREFIXES = new Set([
	'mcp',
	'.well-known',
	'sitemap.xml',
	'sitemap-posts.xml',
	'sitemap-spaces.xml',
	'sitemap-tags.xml',
	'robots.txt',
	'latest.rss',
]);

/** 分组键 → 展示名。缺省回退为键本身。 */
const GROUP_TITLES = {
	admin: ['管理后台', 'Admin'],
	announcements: ['公告', 'Announcements'],
	'a api-keys': ['', ''],
	api: ['API', 'API'],
	'api-keys': ['API Key', 'API Keys'],
	auth: ['认证', 'Authentication'],
	badges: ['徽章', 'Badges'],
	blocks: ['用户拉黑', 'User Blocks'],
	checkin: ['签到', 'Check-in'],
	comments: ['评论', 'Comments'],
	contents: ['评论内容', 'Contents'],
	customization: ['站点自定义', 'Customization'],
	dashboard: ['首页聚合', 'Dashboard'],
	drafts: ['草稿', 'Drafts'],
	favorites: ['收藏', 'Favorites'],
	groups: ['用户组', 'Groups'],
	health: ['健康检查', 'Health'],
	'latest.rss': ['RSS', 'RSS'],
	leaderboard: ['排行榜', 'Leaderboard'],
	levels: ['等级', 'Levels'],
	messages: ['私信', 'Messages'],
	motions: ['社区动议', 'Motions'],
	notifications: ['通知', 'Notifications'],
	oauth: ['OAuth 2.0', 'OAuth 2.0'],
	pages: ['自定义页面', 'Custom Pages'],
	permission: ['权限', 'Permissions'],
	posts: ['帖子', 'Posts'],
	realtime: ['实时事件', 'Realtime'],
	reports: ['举报', 'Reports'],
	'robots.txt': ['robots.txt', 'robots.txt'],
	role: ['角色', 'Roles'],
	security: ['安全', 'Security'],
	sessions: ['会话', 'Sessions'],
	settings: ['设置', 'Settings'],
	'sitemap-posts.xml': ['Sitemap（帖子）', 'Sitemap (posts)'],
	'sitemap-spaces.xml': ['Sitemap（版面）', 'Sitemap (spaces)'],
	'sitemap-tags.xml': ['Sitemap（标签）', 'Sitemap (tags)'],
	'sitemap.xml': ['Sitemap 索引', 'Sitemap index'],
	spaces: ['版面', 'Spaces'],
	sso: ['OIDC SSO', 'OIDC SSO'],
	tags: ['标签', 'Tags'],
	themes: ['主题', 'Themes'],
	upload: ['上传', 'Uploads'],
	user: ['用户资料', 'User Profile'],
	users: ['用户与关系', 'Users and Relations'],
	watches: ['主题订阅', 'Watches'],
};

const METHOD_LABELS = { get: 'GET', post: 'POST', put: 'PUT', patch: 'PATCH', delete: 'DELETE' };

const METHODS = new Set(Object.keys(METHOD_LABELS));

function groupTitle(key, locale) {
	const index = locale === 'en' ? 1 : 0;
	return GROUP_TITLES[key]?.[index] ?? key;
}

async function generate() {
	const openapi = JSON.parse(await readFile(join(repoRoot, 'openapi.json'), 'utf8'));
	const groups = new Map();
	let operations = 0;

	for (const [rawPath, item] of Object.entries(openapi.paths ?? {})) {
		const segments = rawPath.split('/').filter(Boolean);
		const key = segments[0] ?? 'root';
		const isRoot = ROOT_PREFIXES.has(key);
		const path = isRoot ? rawPath : `/api/v1${rawPath}`;
		for (const [method, op] of Object.entries(item ?? {})) {
			if (!METHODS.has(method)) continue;
			operations += 1;
			if (!groups.has(key)) groups.set(key, []);
			groups.get(key).push({
				method: method.toUpperCase(),
				path,
				summary: op.summary || op.description || op.operationId || '',
			});
		}
	}

	const order = Object.keys(GROUP_TITLES);
	const sortedGroups = [...groups.entries()]
		.sort(([a], [b]) => {
			const ia = order.indexOf(a);
			const ib = order.indexOf(b);
			if (ia !== -1 || ib !== -1) return (ia === -1 ? 999 : ia) - (ib === -1 ? 999 : ib);
			return a.localeCompare(b);
		})
		.map(([key, items]) => ({
			key,
			title: groupTitle(key, 'zh'),
			titleEn: groupTitle(key, 'en'),
			items: items.sort((a, b) => a.path.localeCompare(b.path) || a.method.localeCompare(b.method)),
		}));

	const endpoints = {
		paths: Object.keys(openapi.paths ?? {}).length,
		operations,
		groups: sortedGroups,
	};

	const settings = await generateSettings();
	const metrics = await generateMetrics();

	// 不写生成时间：--check 比较的是内容，时间戳会让每次构建都判定为"过期"。
	return {
		openapiVersion: openapi.info?.version ?? 'unknown',
		endpoints,
		settings,
		metrics,
	};
}

async function generateSettings() {
	const file = join(repoRoot, 'app', 'server', 'internal', 'model', 'setting.go');
	const source = await readFile(file, 'utf8');
	const start = source.indexOf('var DefaultSettings = map[string]string{');
	const end = source.indexOf('\n}', start);
	if (start === -1 || end === -1) throw new Error('DefaultSettings 映射未找到，解析规则需要更新');
	const body = source.slice(start, end);

	const publicStart = source.indexOf('var PublicSettingKeys = []string{');
	const publicEnd = source.indexOf('\n}', publicStart);
	const publicKeys = new Set(
		publicStart === -1 || publicEnd === -1
			? []
			: [...source.slice(publicStart, publicEnd).matchAll(/"([^"]+)"/g)].map((m) => m[1]),
	);

	const entries = [];
	for (const line of body.split('\n')) {
		const trimmed = line.trim();
		if (!trimmed.startsWith('"')) continue;
		const match = trimmed.match(/^"([^"]+)"\s*:\s*"(.*)",?$/);
		if (!match) continue;
		entries.push({ key: match[1], value: match[2], public: publicKeys.has(match[1]) });
	}

	// 按前缀分组，oauth_* 单独成组（49 个键，混在 "oauth" 里会淹没其他配置）。
	const groups = new Map();
	for (const entry of entries) {
		const prefix = entry.key.split('_')[0];
		if (!groups.has(prefix)) groups.set(prefix, []);
		groups.get(prefix).push(entry);
	}

	const PREFIX_TITLES = {
		site: ['站点', 'Site'],
		seo: ['SEO', 'SEO'],
		og: ['Open Graph', 'Open Graph'],
		google: ['站点验证', 'Verification'],
		baidu: ['站点验证', 'Verification'],
		user: ['用户', 'Users'],
		sitemap: ['Sitemap', 'Sitemap'],
		register: ['注册', 'Registration'],
		post: ['帖子', 'Posts'],
		comment: ['评论', 'Comments'],
		moderation: ['审核', 'Moderation'],
		mcp: ['MCP', 'MCP'],
		trust: ['成长体系', 'Trust'],
		email: ['邮件', 'Email'],
		space: ['版面', 'Spaces'],
		tag: ['标签', 'Tags'],
		announcement: ['公告', 'Announcements'],
		article: ['文章', 'Articles'],
		oauth: ['OAuth / OIDC', 'OAuth / OIDC'],
		sso: ['OIDC SSO', 'OIDC SSO'],
		api: ['API Key', 'API Keys'],
		mfa: ['多因素认证', 'MFA'],
		rate: ['限流', 'Rate Limits'],
		security: ['安全', 'Security'],
		backup: ['备份', 'Backups'],
		page: ['分页', 'Pagination'],
		upload: ['上传', 'Uploads'],
		sse: ['实时推送', 'SSE'],
	};

	return {
		total: entries.length,
		groups: [...groups.entries()].map(([prefix, items]) => ({
			key: prefix,
			title: PREFIX_TITLES[prefix]?.[0] ?? prefix,
			titleEn: PREFIX_TITLES[prefix]?.[1] ?? prefix,
			items,
		})),
	};
}

async function generateMetrics() {
	const dir = join(repoRoot, 'app', 'server', 'internal', 'metrics');
	const files = (await readdir(dir)).filter((name) => name.endsWith('.go'));
	const found = new Map();
	for (const name of files) {
		const source = await readFile(join(dir, name), 'utf8');
		const pattern = /Name:\s*"(feedchamber_[a-z0-9_]+)"[\s\S]{0,400}?Help:\s*"([^"]*)"/g;
		for (const match of source.matchAll(pattern)) {
			// 构造函数出现在 Name 之前；取最近一次匹配，避免把上一个变量的
			// NewCounterVec 误判成本条的 Histogram。
			const before = source.slice(Math.max(0, match.index - 240), match.index);
			const types = [...before.matchAll(/New(HistogramVec|CounterVec|GaugeVec|Histogram|Counter|Gauge|Summary)/g)];
			const type = types.at(-1)?.[1] ?? 'Metric';
			found.set(match[1], {
				name: match[1],
				help: match[2],
				type: type.replace('Vec', ''),
			});
		}
	}
	return [...found.values()].sort((a, b) => a.name.localeCompare(b.name));
}

const data = await generate();
const json = `${JSON.stringify(data, null, '\t')}\n`;

if (process.argv.includes('--check')) {
	let current = '';
	try {
		current = await readFile(outFile, 'utf8');
	} catch {
		console.error('✗ src/data/reference.json 缺失，运行 node scripts/gen-reference.mjs');
		process.exit(1);
	}
	if (current !== json) {
		console.error('✗ src/data/reference.json 已过期，运行 node scripts/gen-reference.mjs 后提交');
		process.exit(1);
	}
	console.log(`✓ reference.json 与源码一致（${data.endpoints.paths} path / ${data.endpoints.operations} 操作）`);
} else {
	await mkdir(dirname(outFile), { recursive: true });
	await writeFile(outFile, json);
	console.log(
		`✓ reference.json: ${data.endpoints.paths} path / ${data.endpoints.operations} 操作, ` +
			`${data.settings.total} 个设置键, ${data.metrics.length} 个指标`,
	);
}
