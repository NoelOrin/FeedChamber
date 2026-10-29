/**
 * 代码高亮分词器（`CodeBlock` 的懒加载块）。
 *
 * 旧 Solid 版用 `shiki` 的 `createHighlighter()` + `createResource`，主题 github-light / github-dark。
 * 本分支的 `package.json` 未装 shiki（也不允许我改 package.json），因此这里自带一个
 * 极小的字符扫描分词器：语言集合、`language-*` 类名、复制交互、以及「高亮未就绪时先渲染
 * 纯文本 `<pre>`」的降级行为全部与旧版一致，只是着色换成设计系统的墨色/主色阶。
 * 迁移计划 §3.3 的 `createResource → use()` 由 `code-block.tsx` 一侧承担。
 */

export type TokenKind = 'comment' | 'string' | 'number' | 'keyword' | 'literal' | 'plain';

export interface CodeToken {
	text: string;
	kind: TokenKind;
}

interface LangSpec {
	/** 行注释前缀 */
	line: string[];
	/** 块注释起止 */
	block?: [string, string];
	/** 引号字符 */
	quotes: string[];
	keywords: string[];
	literals?: string[];
}

const JS = [
	'abstract',
	'any',
	'as',
	'async',
	'await',
	'boolean',
	'break',
	'case',
	'catch',
	'class',
	'const',
	'continue',
	'declare',
	'default',
	'delete',
	'do',
	'else',
	'enum',
	'export',
	'extends',
	'finally',
	'for',
	'from',
	'function',
	'get',
	'goto',
	'if',
	'implements',
	'import',
	'in',
	'instanceof',
	'interface',
	'let',
	'module',
	'namespace',
	'new',
	'number',
	'object',
	'of',
	'package',
	'return',
	'set',
	'string',
	'super',
	'switch',
	'symbol',
	'this',
	'throw',
	'type',
	'typeof',
	'var',
	'void',
	'while',
	'with',
	'yield',
];

const SHELL = [
	'and',
	'bash',
	'break',
	'case',
	'do',
	'done',
	'elif',
	'else',
	'esac',
	'exit',
	'fi',
	'for',
	'function',
	'if',
	'in',
	'local',
	'return',
	'then',
	'until',
	'while',
];

const PYTHON = [
	'and',
	'as',
	'assert',
	'async',
	'await',
	'break',
	'class',
	'continue',
	'def',
	'del',
	'elif',
	'else',
	'except',
	'finally',
	'for',
	'from',
	'global',
	'if',
	'import',
	'in',
	'is',
	'lambda',
	'nonlocal',
	'not',
	'or',
	'pass',
	'raise',
	'return',
	'try',
	'while',
	'with',
	'yield',
];

const SQL = [
	'ALTER',
	'AND',
	'AS',
	'BEGIN',
	'BETWEEN',
	'BY',
	'CASE',
	'COMMIT',
	'CREATE',
	'DELETE',
	'DISTINCT',
	'DROP',
	'ELSE',
	'END',
	'EXISTS',
	'FROM',
	'GROUP',
	'HAVING',
	'IN',
	'INNER',
	'INSERT',
	'INTO',
	'IS',
	'JOIN',
	'LEFT',
	'LIKE',
	'LIMIT',
	'NOT',
	'NULL',
	'OFFSET',
	'ON',
	'OR',
	'ORDER',
	'OUTER',
	'REFERENCES',
	'RIGHT',
	'SELECT',
	'TABLE',
	'TRANSACTION',
	'UNION',
	'UPDATE',
	'VALUES',
	'WHERE',
	'WITH',
];

const specs: Record<string, LangSpec> = {
	javascript: {
		line: ['//'],
		block: ['/*', '*/'],
		quotes: ["'", '"', '`'],
		keywords: JS,
		literals: ['true', 'false', 'null', 'undefined', 'NaN'],
	},
	typescript: {
		line: ['//'],
		block: ['/*', '*/'],
		quotes: ["'", '"', '`'],
		keywords: JS,
		literals: ['true', 'false', 'null', 'undefined'],
	},
	jsx: {
		line: ['//'],
		block: ['/*', '*/'],
		quotes: ["'", '"', '`'],
		keywords: JS,
		literals: ['true', 'false', 'null', 'undefined'],
	},
	tsx: {
		line: ['//'],
		block: ['/*', '*/'],
		quotes: ["'", '"', '`'],
		keywords: JS,
		literals: ['true', 'false', 'null', 'undefined'],
	},
	json: { line: [], quotes: ['"'], keywords: [], literals: ['true', 'false', 'null'] },
	css: {
		line: [],
		block: ['/*', '*/'],
		quotes: ["'", '"'],
		keywords: ['and', 'important', 'not', 'or', 'from', 'to'],
		literals: [],
	},
	scss: {
		line: ['//'],
		block: ['/*', '*/'],
		quotes: ["'", '"'],
		keywords: ['and', 'important', 'not', 'or'],
		literals: [],
	},
	html: {
		line: [],
		block: ['<!--', '-->'],
		quotes: ['"', "'"],
		keywords: ['a', 'body', 'div', 'head', 'html', 'li', 'script', 'span', 'style', 'td', 'tr', 'ul'],
		literals: [],
	},
	xml: { line: [], block: ['<!--', '-->'], quotes: ['"', "'"], keywords: [], literals: [] },
	bash: { line: ['#'], quotes: ["'", '"'], keywords: SHELL, literals: [] },
	sh: { line: ['#'], quotes: ["'", '"'], keywords: SHELL, literals: [] },
	shell: { line: ['#'], quotes: ["'", '"'], keywords: SHELL, literals: [] },
	python: { line: ['#'], quotes: ["'", '"'], keywords: PYTHON, literals: ['True', 'False', 'None'] },
	rust: {
		line: ['//'],
		block: ['/*', '*/'],
		quotes: ["'", '"'],
		keywords: [
			'as',
			'async',
			'await',
			'break',
			'const',
			'else',
			'enum',
			'fn',
			'for',
			'if',
			'impl',
			'in',
			'let',
			'match',
			'mod',
			'mut',
			'pub',
			'ref',
			'return',
			'self',
			'struct',
			'trait',
			'type',
			'unsafe',
			'use',
			'where',
			'while',
		],
		literals: ['true', 'false'],
	},
	go: {
		line: ['//'],
		block: ['/*', '*/'],
		quotes: ['"', '`'],
		keywords: [
			'break',
			'case',
			'chan',
			'const',
			'continue',
			'default',
			'defer',
			'else',
			'fallthrough',
			'for',
			'func',
			'go',
			'goto',
			'if',
			'import',
			'interface',
			'map',
			'package',
			'range',
			'return',
			'select',
			'struct',
			'switch',
			'type',
			'var',
		],
		literals: ['true', 'false', 'nil'],
	},
	sql: { line: ['--'], block: ['/*', '*/'], quotes: ["'", '"'], keywords: SQL, literals: ['NULL'] },
	markdown: { line: [], quotes: ['`'], keywords: [], literals: [] },
	yaml: { line: ['#'], quotes: ["'", '"'], keywords: [], literals: ['true', 'false', 'null'] },
	toml: { line: ['#'], quotes: ["'", '"'], keywords: [], literals: ['true', 'false'] },
	dockerfile: {
		line: ['#'],
		quotes: ['"', "'"],
		keywords: ['AS', 'COPY', 'ENV', 'EXPOSE', 'FROM', 'LABEL', 'RUN', 'USER', 'VOLUME', 'WORKDIR'],
		literals: [],
	},
};

const ALIAS: Record<string, string> = {
	js: 'javascript',
	mjs: 'javascript',
	cjs: 'javascript',
	node: 'javascript',
	ts: 'typescript',
	py: 'python',
	py3: 'python',
	rs: 'rust',
	golang: 'go',
	sh: 'bash',
	zsh: 'bash',
	ps1: 'bash',
	md: 'markdown',
	yml: 'yaml',
	html: 'html',
	htm: 'html',
	vue: 'html',
	svelte: 'html',
	sqlite: 'sql',
	postgres: 'sql',
	mysql: 'sql',
	docker: 'dockerfile',
};

/** 与旧版 `getLoadedLanguages().includes(lang) ? lang : 'text'` 同语义：未知语言走纯文本降级。 */
function resolveLanguage(language?: string | null): string {
	const raw = (language ?? '').trim().toLowerCase();
	if (!raw) return 'text';
	const key = ALIAS[raw] ?? raw;
	return specs[key] ? key : 'text';
}

const WORD_START = /[A-Za-z_$]/;
const WORD_BODY = /[A-Za-z0-9_$.]/;
const DIGIT = /[0-9]/;

function specOf(lang: string): LangSpec | null {
	return specs[lang] ?? null;
}

/**
 * 单遍字符扫描：块注释状态跨行保持，字符串遇到未闭合时收尾到行尾（不把后面整段吞掉）。
 * 相邻同类 token 会合并，减少渲染出的 span 数量。
 */
export function highlight(code: string, language?: string | null): CodeToken[] {
	const lang = resolveLanguage(language);
	const spec = specOf(lang);
	const tokens: CodeToken[] = [];
	const push = (text: string, kind: TokenKind) => {
		if (!text) return;
		const last = tokens[tokens.length - 1];
		if (last && last.kind === kind) last.text += text;
		else tokens.push({ text, kind });
	};

	if (!spec) {
		push(code, 'plain');
		return tokens;
	}

	const keywords = new Set(spec.keywords.map((k) => k.toLowerCase()));
	const literals = new Set((spec.literals ?? []).map((k) => k.toLowerCase()));
	const block = spec.block;
	const line = spec.line;

	let i = 0;
	let inBlock = false;

	while (i < code.length) {
		const rest = code.slice(i);

		if (inBlock && block) {
			const close = rest.indexOf(block[1]);
			if (close < 0) {
				push(rest, 'comment');
				break;
			}
			push(rest.slice(0, close + block[1].length), 'comment');
			i += close + block[1].length;
			inBlock = false;
			continue;
		}

		if (block && rest.startsWith(block[0])) {
			const close = rest.indexOf(block[1], block[0].length);
			if (close < 0) {
				push(rest, 'comment');
				inBlock = true;
				break;
			}
			push(rest.slice(0, close + block[1].length), 'comment');
			i += close + block[1].length;
			continue;
		}

		const linePrefix = line.find((prefix) => rest.startsWith(prefix));
		if (linePrefix) {
			const end = rest.indexOf('\n');
			const chunk = end < 0 ? rest : rest.slice(0, end);
			push(chunk, 'comment');
			i += chunk.length;
			continue;
		}

		const quote = spec.quotes.find((q) => code[i] === q);
		if (quote) {
			let j = i + 1;
			let text = quote;
			while (j < code.length) {
				const ch = code[j];
				if (ch === '\\' && j + 1 < code.length) {
					text += ch + code[j + 1];
					j += 2;
					continue;
				}
				// 单引号/双引号不跨行（模板字符串与反引号除外）
				if (ch === '\n' && quote !== '`') break;
				j += 1;
				text += ch;
				if (ch === quote) break;
			}
			push(text, 'string');
			i += text.length;
			continue;
		}

		const ch = code[i];
		if (ch && DIGIT.test(ch)) {
			let j = i;
			while (j < code.length && /[0-9a-fA-FxXbBoO._eE]/.test(code[j] as string)) j += 1;
			push(code.slice(i, j), 'number');
			i = j;
			continue;
		}

		if (ch && WORD_START.test(ch)) {
			let j = i;
			while (j < code.length && WORD_BODY.test(code[j] as string)) j += 1;
			const word = code.slice(i, j);
			const lower = word.toLowerCase();
			push(word, literals.has(lower) ? 'literal' : keywords.has(lower) ? 'keyword' : 'plain');
			i = j;
			continue;
		}

		push(ch ?? ' ', 'plain');
		i += (ch ?? ' ').length;
	}

	return tokens;
}
