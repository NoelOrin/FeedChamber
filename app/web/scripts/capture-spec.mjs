// SPEC 主题验收截图：登录后逐模块取图，输出到 artifacts/ui-spec/

import { mkdirSync } from 'node:fs';
import { chromium } from 'playwright';

const BASE = process.env.CAPTURE_BASE || 'http://localhost:3001';
const OUT = process.env.CAPTURE_OUT || '../../artifacts/ui-spec';
const POST_ID = process.env.POST_ID || '01M0MJHSG5BE8HQT8NDR7NYCE8';
const USER_ID = process.env.USER_ID || '01M0MJJC7SREBKS3V9X8YS3HBC';

mkdirSync(OUT, { recursive: true });

const authed = [
	['timeline', '/timeline'],
	['timeline-create', '/timeline/create'],
	['timeline-tag', '/timeline/tag'],
	['post-detail', '/timeline/post?id=' + POST_ID],
	['spaces', '/spaces'],
	['space-chat', '/spaces/chat'],
	['space-chat-post', '/spaces/chat/post'],
	['home', '/home'],
	['leaderboard', '/leaderboard'],
	['search', '/search?q=go'],
	['user-self', '/user'],
	['user-posts', '/user/posts'],
	['user-notifications', '/user/notifications'],
	['user-follow', '/user/follow'],
	['user-favorites', '/user/favorites'],
	['user-other', '/user/' + USER_ID],
	['messages', '/messages'],
	['ui-showcase', '/ui-showcase'],
];

const anon = [
	['login', '/login'],
	['register', '/register'],
];

async function shot(page, name, path) {
	await page.goto(BASE + path, { waitUntil: 'load' });
	await page.waitForTimeout(900);
	await page.screenshot({ path: OUT + '/' + name + '.png', fullPage: true });
	console.log('captured', name);
}

const browser = await chromium.launch();

// 1. 登录 + 桌面亮色
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1 });
const page = await ctx.newPage();
await page.goto(BASE + '/login', { waitUntil: 'load' });
await page.waitForTimeout(1500);
await page.screenshot({ path: OUT + '/login.png', fullPage: true });
for (let attempt = 0; attempt < 3; attempt += 1) {
	await page.fill('#login-identity', 'admin');
	await page.fill('#login-password', 'admin123');
	await page.click('button[type=submit]');
	try {
		await page.waitForURL('**/timeline', { timeout: 15000 });
		break;
	} catch {
		await page.waitForTimeout(1500);
	}
}
await page.waitForTimeout(1500);

for (const entry of authed) {
	await shot(page, entry[0], entry[1]);
}

// 2. 暗色：用同一批 cookie，切换 html.dark
async function darkShot(name, path) {
	await page.goto(BASE + path, { waitUntil: 'load' });
	await page.evaluate(() => {
		document.documentElement.classList.add('dark');
		document.documentElement.dataset.themeMode = 'dark';
		localStorage.setItem('orin-theme', 'dark');
	});
	await page.waitForTimeout(700);
	await page.screenshot({ path: OUT + '/' + name + '-dark.png', fullPage: true });
	console.log('captured', name + '-dark');
}
await darkShot('timeline', '/timeline');
await darkShot('post-detail', '/timeline/post?id=' + POST_ID);
await ctx.close();

// 3. 匿名：登录/注册亮色 + 暗色
const anonCtx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const anonPage = await anonCtx.newPage();
for (const entry of anon) {
	await shot(anonPage, entry[0], entry[1]);
}
await anonPage.goto(BASE + '/login', { waitUntil: 'load' });
await anonPage.evaluate(() => {
	document.documentElement.classList.add('dark');
	document.documentElement.dataset.themeMode = 'dark';
});
await anonPage.waitForTimeout(600);
await anonPage.screenshot({ path: OUT + '/login-dark.png', fullPage: true });
await anonCtx.close();

// 4. 移动端 390px
const mobileCtx = await browser.newContext({
	viewport: { width: 390, height: 844 },
	isMobile: true,
	hasTouch: true,
	deviceScaleFactor: 2,
});
const mobile = await mobileCtx.newPage();
await mobile.goto(BASE + '/login', { waitUntil: 'load' });
await mobile.waitForTimeout(1500);
await mobile.screenshot({ path: OUT + '/login-mobile.png', fullPage: true });
for (let attempt = 0; attempt < 3; attempt += 1) {
	await mobile.fill('#login-identity', 'admin');
	await mobile.fill('#login-password', 'admin123');
	await mobile.click('button[type=submit]');
	try {
		await mobile.waitForURL('**/timeline', { timeout: 15000 });
		break;
	} catch {
		await mobile.waitForTimeout(1500);
	}
}
await mobile.waitForTimeout(1000);
await mobile.screenshot({ path: OUT + '/timeline-mobile.png', fullPage: true });
await mobile.goto(BASE + '/timeline/post?id=' + POST_ID, { waitUntil: 'load' });
await mobile.waitForTimeout(900);
await mobile.screenshot({ path: OUT + '/post-detail-mobile.png', fullPage: true });
await mobileCtx.close();

await browser.close();
console.log('done');
