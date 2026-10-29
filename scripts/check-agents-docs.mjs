#!/usr/bin/env node
/**
 * 模块文档同步门（AGENTS.md）
 *
 * 三条检查，零依赖（只用 Node 内置模块，CI 里不需要 pnpm install）：
 *
 *   1. --staged        本地提示：暂存了模块代码却没动该模块的 AGENTS.md → 打印提醒，**始终 exit 0**
 *   2. --base <ref>    CI 阻断：相对 <ref> 改了模块代码却没动该模块 AGENTS.md → exit 1
 *   3. --check-index   根 AGENTS.md 的索引块与实际 AGENTS.md 集合不一致 → exit 1
 *      --write-index   重写根 AGENTS.md 的索引块
 *
 * 模块归属规则：每个被改的代码文件归属到**最深的、含 AGENTS.md 的祖先目录**。
 * 因此新增一个模块目录只要建了 AGENTS.md 就自动纳入，不需要维护映射表。
 *
 * 用法：
 *   node scripts/check-agents-docs.mjs --staged
 *   node scripts/check-agents-docs.mjs --base origin/develop
 *   node scripts/check-agents-docs.mjs --check-index
 *   node scripts/check-agents-docs.mjs --write-index
 */

import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join, sep } from 'node:path';

/**
 * 仓库根按**当前工作目录**解析，而不是脚本自身所在目录：
 * 这样脚本可以对着任意 git 仓库跑（CI、临时目录、测试夹具都成立）。
 */
const REPO_ROOT = execFileSync('git', ['rev-parse', '--show-toplevel'], { encoding: 'utf8' }).trim();
const DOC_NAME = 'AGENTS.md';
const ROOT_DOC = 'AGENTS.md';
const INDEX_BEGIN = '<!-- AGENTS-INDEX:BEGIN -->';
const INDEX_END = '<!-- AGENTS-INDEX:END -->';

/** 计入「模块代码」的扩展名。文档（.md）、配置产物（.json/.lock）不算。 */
const CODE_EXTS = new Set(['.go', '.ts', '.tsx', '.js', '.mjs', '.cjs', '.css', '.sql', '.sh', '.py', '.yml', '.yaml']);

/**
 * 生成物 / 第三方目录：这些路径上的改动不要求同步文档。
 * 每条都是"改了也不该手写文档"的东西——要么是构建产物，要么是依赖。
 */
const GENERATED_PREFIXES = [
	'node_modules/',
	'app/server/docs/', // make swag 生成
	'packages/tokens/dist/', // 提交进仓库的生成产物
	'.turbo/',
	'dist/',
	'.output/',
	'.vinxi/',
	'patches/',
];

/** 这些文件虽然是代码，但改动不需要文档（根级脚手架）。 */
const EXEMPT_FILES = new Set([
	'openapi.json',
	'pnpm-lock.yaml',
	'package.json',
	'pnpm-workspace.yaml',
	'turbo.json',
	'biome.json',
	'lefthook.yml',
	'commitlint.config.mjs',
	'tsconfig.json',
	'tsconfig.check.json',
]);

function git(args, opts = {}) {
	return execFileSync('git', args, { cwd: REPO_ROOT, encoding: 'utf8', ...opts }).trim();
}

function normalize(path) {
	return path.split(sep).join('/');
}

function isExempt(path) {
	if (EXEMPT_FILES.has(path)) return true;
	if (GENERATED_PREFIXES.some((p) => path.startsWith(p))) return true;
	// 任意层级的 node_modules / 构建产物
	if (/(^|\/)(node_modules|dist|\.turbo|\.output|\.vinxi)\//.test(path)) return true;
	return false;
}

function hasCodeExt(path) {
	const dot = path.lastIndexOf('.');
	if (dot < 0) return false;
	return CODE_EXTS.has(path.slice(dot));
}

/** 全部被版本控制的 AGENTS.md（排除根文档自身）。 */
function listDocs() {
	return git(['ls-files', `*${DOC_NAME}`])
		.split('\n')
		.map(normalize)
		.filter((p) => p && p !== ROOT_DOC);
}

/** 文档所在目录集合，按路径长度降序（用于"最深的祖先"匹配）。 */
function docDirs() {
	return listDocs()
		.map((doc) => dirname(doc))
		.map((dir) => (dir === '.' ? '' : dir))
		.sort((a, b) => b.length - a.length);
}

/**
 * 把一个文件路径归属到最深的含 AGENTS.md 的祖先目录。
 * 返回该文档的仓库相对路径；找不到（例如根文档之外的松散文件）返回 null。
 */
function ownerDocOf(filePath, dirs) {
	for (const dir of dirs) {
		if (dir === '') continue; // 根目录不参与：根 AGENTS.md 不因任意根级文件变更而强制更新
		if (filePath === dir || filePath.startsWith(`${dir}/`)) {
			return `${dir}/${DOC_NAME}`;
		}
	}
	return null;
}

/** 变更文件列表：--staged 用 diff --cached；--base 用 diff --name-only <ref>...HEAD。 */
function changedFiles(mode, ref) {
	const args =
		mode === 'staged'
			? ['diff', '--cached', '--name-only', '--diff-filter=ACMR']
			: ['diff', '--name-only', '--diff-filter=ACMR', `${ref}...HEAD`];
	const out = git(args);
	return out ? out.split('\n').map(normalize).filter(Boolean) : [];
}

/** 收集「模块代码改了但文档没改」的违规项。 */
function findViolations(files) {
	const dirs = docDirs();
	const changedDocs = new Set(files.filter((f) => f.endsWith(`/${DOC_NAME}`) || f === ROOT_DOC));

	const byModule = new Map(); // docPath -> Set(file)
	for (const file of files) {
		if (!hasCodeExt(file) || isExempt(file)) continue;
		if (file.endsWith(DOC_NAME)) continue;
		const doc = ownerDocOf(file, dirs);
		if (!doc) continue; // 不属于任何模块
		if (changedDocs.has(doc)) continue; // 该模块文档已同步更新
		if (!byModule.has(doc)) byModule.set(doc, new Set());
		byModule.get(doc).add(file);
	}
	return byModule;
}

/** 文档的标题（第一行 `# xxx`）与目录名，用于索引条目。 */
function docSummary(docPath) {
	const raw = readFileSync(join(REPO_ROOT, docPath), 'utf8');
	const first = raw.split('\n').find((line) => line.startsWith('# ')) ?? '';
	const title = first.replace(/^#\s+/, '').trim();
	const dir = dirname(docPath);
	return { title: title || dir, dir: dir === '.' ? '' : dir };
}

/** 生成索引块内容（按路径排序，目录层级用缩进表达）。 */
function buildIndexBlock() {
	const docs = listDocs().sort();
	const lines = [
		INDEX_BEGIN,
		'',
		'<!-- 由 `node scripts/check-agents-docs.mjs --write-index` 生成，请勿手改。 -->',
		'',
		`共 ${docs.length} 个模块文档。改动某模块的代码时，必须在同一次变更内更新它的 AGENTS.md（CI 会阻断）。`,
		'',
		'| 模块目录 | 文档 | 标题 |',
		'|----------|------|------|',
	];
	for (const doc of docs) {
		const { title, dir } = docSummary(doc);
		lines.push(`| \`${dir}/\` | [${DOC_NAME}](${doc}) | ${title} |`);
	}
	lines.push('', INDEX_END);
	return lines.join('\n');
}

function writeIndex() {
	const rootPath = join(REPO_ROOT, ROOT_DOC);
	const raw = readFileSync(rootPath, 'utf8');
	const block = buildIndexBlock();
	const begin = raw.indexOf(INDEX_BEGIN);
	const end = raw.indexOf(INDEX_END);

	if (begin >= 0 && end > begin) {
		const updated = raw.slice(0, begin) + block + raw.slice(end + INDEX_END.length);
		writeFileSync(rootPath, updated);
	} else {
		// 首次插入：追加到文末，前面留空行
		writeFileSync(rootPath, `${raw.replace(/\s*$/, '')}\n\n## 模块文档索引\n\n${block}\n`);
	}
	console.log(`✓ 已重写根 ${ROOT_DOC} 的模块文档索引块（${listDocs().length} 个文档）`);
}

function checkIndex() {
	const rootPath = join(REPO_ROOT, ROOT_DOC);
	const raw = readFileSync(rootPath, 'utf8');
	const begin = raw.indexOf(INDEX_BEGIN);
	const end = raw.indexOf(INDEX_END);

	if (begin < 0 || end < begin) {
		console.error(`✗ 根 ${ROOT_DOC} 缺少模块文档索引块。请运行：node scripts/check-agents-docs.mjs --write-index`);
		return 1;
	}

	const current = raw.slice(begin, end + INDEX_END.length);
	const expected = buildIndexBlock();
	if (current !== expected) {
		console.error(`✗ 根 ${ROOT_DOC} 的模块文档索引块已过期（新增/删除/改名了模块文档？）。`);
		console.error('  请运行：node scripts/check-agents-docs.mjs --write-index');
		// 打印差异摘要：缺哪些、多哪些
		const currentDocs = new Set([...current.matchAll(/\]\((.*?AGENTS\.md)\)/g)].map((m) => m[1]));
		const expectedDocs = new Set([...expected.matchAll(/\]\((.*?AGENTS\.md)\)/g)].map((m) => m[1]));
		const missing = [...expectedDocs].filter((d) => !currentDocs.has(d));
		const extra = [...currentDocs].filter((d) => !expectedDocs.has(d));
		if (missing.length) console.error(`  索引中缺少：${missing.join(', ')}`);
		if (extra.length) console.error(`  索引中多余：${extra.join(', ')}`);
		return 1;
	}

	// 额外校验：每个文档条目指向的文件真实存在（防手改出死链）
	const listed = [...current.matchAll(/\]\((.*?AGENTS\.md)\)/g)].map((m) => m[1]);
	const dead = listed.filter((doc) => !existsSync(join(REPO_ROOT, doc)));
	if (dead.length) {
		console.error(`✗ 索引中有指向不存在文件的链接：${dead.join(', ')}`);
		return 1;
	}

	console.log(`✓ 模块文档索引块有效（${listed.length} 个文档）`);
	return 0;
}

function report(violations, blocking) {
	if (violations.size === 0) {
		console.log('✓ 模块代码改动都同步了对应的 AGENTS.md');
		return 0;
	}

	const lines = [];
	for (const [doc, files] of [...violations].sort()) {
		const list = [...files].slice(0, 5).join(', ');
		const more = files.size > 5 ? ` 等 ${files.size} 个` : '';
		lines.push(`  ${doc}\n      相关改动：${list}${more}`);
	}

	if (!blocking) {
		console.log('提示：以下模块的代码有改动，但对应的 AGENTS.md 没更新：\n');
		console.log(lines.join('\n'));
		console.log('\n（pre-commit 只提示不阻断；CI 会阻断合并。若改动确实不影响文档内容，可忽略。）');
		return 0;
	}

	console.error('✗ 以下模块的代码有改动，但对应的 AGENTS.md 没更新：\n');
	console.error(lines.join('\n'));
	console.error('\n每个模块的 AGENTS.md 末尾都有「本文件何时更新」一节，按它判断是否该更新。');
	console.error('若确认改动不影响文档，在提交信息里说明即可（CI 仍会阻断，需显式放行）。');
	return 1;
}

function main(argv) {
	const has = (flag) => argv.includes(flag);
	const argValue = (flag) => {
		const i = argv.indexOf(flag);
		return i >= 0 ? argv[i + 1] : undefined;
	};

	if (has('--write-index')) {
		writeIndex();
		return 0;
	}
	if (has('--check-index')) {
		return checkIndex();
	}

	const base = argValue('--base');
	if (base) {
		const files = changedFiles('base', base);
		return report(findViolations(files), true);
	}
	if (has('--staged')) {
		const files = changedFiles('staged');
		return report(findViolations(files), false);
	}

	console.error('用法：node scripts/check-agents-docs.mjs [--staged | --base <ref> | --check-index | --write-index]');
	return 2;
}

process.exit(main(process.argv.slice(2)));
