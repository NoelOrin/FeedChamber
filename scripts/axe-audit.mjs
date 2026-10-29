// 一次性可访问性审计：登录后逐路由跑 axe-core，输出 JSON + 控制台摘要。
// 用法：node scripts/axe-audit.mjs [paths...]

import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { chromium } from 'playwright';

const AXE = readFileSync('/tmp/axe2/node_modules/axe-core/axe.min.js', 'utf8');
const BASE = process.env.CAPTURE_BASE || 'http://localhost:3001';
const OUT = 'artifacts/axe';
const POST_ID = '01M0MJHSG5BE8HQT8NDR7NYCE8';
const USER_ID = '01M0MJJC7SREBKS3V9X8YS3HBC';

const DEFAULT_ROUTES = [
	['timeline', '/timeline'],
	['post-detail', `/timeline/post?id=${POST_ID}`],
	['compose', '/timeline/create'],
	['tags', '/timeline/tag'],
	['spaces', '/spaces'],
	['space-detail', '/spaces/tech'],
	['profile-self', '/user'],
	['profile-visitor', `/user/${USER_ID}`],
	['notifications', '/user/notifications'],
	['messages', '/messages'],
	['search', '/search?q=go'],
	['leaderboard', '/leaderboard'],
	['home', '/home'],
	['login', '/login'],
	['register', '/register'],
	['not-found', '/definitely-missing-page'],
];

const routes =
	process.argv.length > 2 ? process.argv.slice(2).map((p) => [p.replace(/[^a-z0-9]+/gi, '_'), p]) : DEFAULT_ROUTES;

mkdirSync(OUT, { recursive: true });

const browser = await chromium.launch();
const results = [];

async function runAxe(page, label, theme) {
	const out = await page.evaluate(async () => {
		// @ts-expect-error
		const res = await axe.run(document, {
			runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa', 'best-practice'] },
			resultTypes: ['violations'],
		});
		return res.violations.map((v) => ({
			id: v.id,
			impact: v.impact,
			help: v.help,
			nodes: v.nodes
				.slice(0, 4)
				.map((n) => ({ target: n.target.join(' '), summary: (n.failureSummary || '').slice(0, 220) })),
			count: v.nodes.length,
		}));
	});
	results.push({ label, theme, url: page.url(), violations: out });
	const total = out.reduce((sum, v) => sum + v.count, 0);
	console.log(`\n[${theme}] ${label} — ${out.length} rules / ${total} nodes`);
	for (const v of out) console.log(`  ${v.impact?.padEnd(8)} ${v.id} x${v.count} — ${v.help}`);
}

async function gotoSoft(page, path) {
	for (let attempt = 0; attempt < 3; attempt += 1) {
		try {
			await page.goto(BASE + path, { waitUntil: 'domcontentloaded', timeout: 45000 });
			await page.waitForTimeout(1400);
			return;
		} catch (e) {
			const msg = String(e);
			if (!/ERR_ABORTED|frame was detached|Timeout/i.test(msg)) throw e;
			console.log('  [retry nav]', path, msg.slice(0, 80));
			await page.waitForTimeout(800);
		}
	}
	console.log('  [nav gave up]', path);
}

async function login(page) {
	await page.goto(`${BASE}/login`, { waitUntil: 'load' });
	await page.waitForTimeout(700);
	try {
		await page.fill('#login-identity', 'admin');
		await page.fill('#login-password', 'admin123');
		await page.click('button[type=submit]');
		// vinext dev 下整页跳转的 load 事件可能不触发，按 URL 轮询而不是等 navigation
		await page.waitForFunction(() => location.pathname !== '/login', null, { timeout: 60000 });
		await page.waitForTimeout(1500);
		return true;
	} catch (e) {
		console.log('[login failed]', String(e).slice(0, 200), 'url=', page.url());
		return false;
	}
}

const ctx = await browser.newContext({ viewport: { width: 1440, height: 950 } });
const page = await ctx.newPage();
// 预热：vinext dev 首次编译会触发依赖优化重载，登录会被打断
await page.goto(`${BASE}/timeline`, { waitUntil: 'domcontentloaded' }).catch(() => {});
await page.waitForTimeout(6000);
await page.goto(`${BASE}/login`, { waitUntil: 'domcontentloaded' }).catch(() => {});
await page.waitForTimeout(2000);
const loggedIn = await login(page);
await page.waitForTimeout(900);

for (const [label, path] of routes) {
	if (!loggedIn && !path.startsWith('/login') && !path.startsWith('/register')) continue;
	await gotoSoft(page, path);
	await page.addScriptTag({ content: AXE });
	await runAxe(page, label, 'light');
}

if (loggedIn) {
	// 切到暗色再跑一遍关键页
	await page.goto(`${BASE}/timeline`, { waitUntil: 'load' });
	await page.waitForTimeout(600);
	for (let i = 0; i < 4; i += 1) {
		const stored = await page.evaluate(() => localStorage.getItem('theme'));
		if (stored === 'dark') break;
		await page.click('[data-id="theme-btn"]');
		await page.waitForTimeout(300);
	}
	await page.waitForTimeout(400);
	for (const path of ['/timeline', `/timeline/post?id=${POST_ID}`, '/user', '/spaces', '/home']) {
		await gotoSoft(page, path);
		await page.addScriptTag({ content: AXE });
		await runAxe(page, path.replace(/[^a-z0-9]+/gi, '_'), 'dark');
	}
}

await browser.close();
writeFileSync(`${OUT}/axe-report.json`, JSON.stringify(results, null, 2));
console.log(`\nwrote ${OUT}/axe-report.json`);
