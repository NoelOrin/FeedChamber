#!/usr/bin/env node
/**
 * 断言构建产物里的站内绝对链接都带 base 前缀。
 *
 * 为什么需要这道门：GitHub Pages 项目页挂在 https://<owner>.github.io/<repo>/，
 * 任何写死成 `/foo/` 的链接都会指到站点根而不是本仓库的产物目录。这类错误
 * 构建期不报、CI 不报，只在线上 404 —— 所以必须自己扫。
 *
 * 用法：DOCS_BASE=/<repo>/ node scripts/check-base-links.mjs
 */
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const distDir = join(here, '..', 'dist');
const base = process.env.DOCS_BASE ?? '/';

if (base === '/') {
	console.log('✓ DOCS_BASE 为 /，无需检查');
	process.exit(0);
}

/** 递归收集所有 .html。 */
function collectHtml(dir, out = []) {
	for (const name of readdirSync(dir)) {
		const full = join(dir, name);
		if (statSync(full).isDirectory()) {
			collectHtml(full, out);
		} else if (name.endsWith('.html')) {
			out.push(full);
		}
	}
	return out;
}

// 这些前缀是「不属于本站」或「由站点根提供」的，不参与 base 校验：
//   //  协议相对外链；# 页内锚点；/ 之外的 scheme 也抓不到（href 只抓绝对路径）
// 这些路径由站点根（或后端）提供，不属于文档站产物，因此既不需要 base 前缀，
// 也不参与死链判定。
const ROOT_PROVIDED = ['/api/v1', '/swagger', '/mcp', '/health', '/sitemap', '/robots.txt', '/latest.rss'];

/** 把站内绝对链接映射成产物里的目标文件，用于死链判定。 */
function targetExists(url) {
	let clean = url.split('#')[0].split('?')[0];
	if (!clean.startsWith('/')) return true; // 非站内绝对路径不判
	// 先剥掉 base 前缀，再映射到产物目录：url 是 /<repo>/foo/，产物是 dist/foo/。
	if (clean === base) clean = '/';
	else if (clean.startsWith(base)) clean = clean.slice(base.length - 1);
	if (clean === '/' || clean === '') return statSafe(join(distDir, 'index.html'));
	const rel = clean.replace(/^\//, '');
	const candidates = [join(distDir, rel), join(distDir, rel, 'index.html'), join(distDir, `${rel}.html`)];
	return candidates.some((c) => statSafe(c));
}

function statSafe(p) {
	try {
		return statSync(p).isFile();
	} catch {
		return false;
	}
}

const problems = [];
const deadLinks = [];
for (const file of collectHtml(distDir)) {
	const html = readFileSync(file, 'utf8');
	// 只抓 href/src 里的站内绝对路径。
	for (const match of html.matchAll(/(?:href|src)="(\/[^"]*)"/g)) {
		const url = match[1];
		if (url.startsWith('//')) continue;

		const rootProvided = ROOT_PROVIDED.some((p) => url === p || url.startsWith(p + '/'));
		const hasBase = url === base || url.startsWith(base);

		// 1) 站内链接必须带 base 前缀
		if (!rootProvided && !hasBase) {
			problems.push({ file: file.replace(distDir, ''), url });
			continue;
		}

		// 2) 带前缀的链接要指向真实存在的产物（防死链）
		if (!rootProvided && hasBase && !targetExists(url)) {
			deadLinks.push({ file: file.replace(distDir, ''), url });
		}
	}
}

if (deadLinks.length > 0) {
	console.error(`✗ 发现 ${deadLinks.length} 个死链（指向不存在的产物）：\n`);
	for (const p of deadLinks.slice(0, 20)) {
		console.error(`  ${p.file}: ${p.url}`);
	}
	if (deadLinks.length > 20) console.error(`  …另有 ${deadLinks.length - 20} 个`);
	console.error('\n修法：改链接目标，或在页面上真的建出该路径。');
	process.exit(1);
}

if (problems.length > 0) {
	console.error(`✗ 发现 ${problems.length} 个未带 base 前缀的站内链接（base=${base}）：\n`);
	for (const p of problems.slice(0, 20)) {
		console.error(`  ${p.file}: ${p.url}`);
	}
	if (problems.length > 20) console.error(`  …另有 ${problems.length - 20} 个`);
	console.error('\n修法：链接用 BASE_URL 前缀（如 MDX 里的 import.meta.env.BASE_URL + 路径），');
	console.error('或确认该路径确实由站点根提供，然后加进本脚本的 ALLOWED_PREFIXES。');
	process.exit(1);
}

console.log(`✓ 站内绝对链接全部带 base 前缀（base=${base}）`);
