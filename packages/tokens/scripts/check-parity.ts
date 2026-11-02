/**
 * @feedchamber/tokens parity check.
 *
 * The failure this guards against actually happened: `color.component.*` used to
 * hold a hand-maintained second palette, and six of its nine values had silently
 * drifted from `color.semantic.*` (mutedColor #9a9590 vs textMuted #6f6b68, …).
 * Consumers read the drifted copy, so editing the real token changed nothing on
 * screen.
 *
 * Component tokens are now derived in build.ts, so parity is structural. This
 * script proves it still holds after generation and fails the build the moment
 * a projection stops resolving to the semantic source of truth.
 *
 * Run: pnpm --filter @feedchamber/tokens check-parity
 */
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import w3cTokens from '../w3c-tokens.json' with { type: 'json' };

const __dirname = dirname(fileURLToPath(import.meta.url));
const distDir = join(__dirname, '..', 'dist');

type W3cToken = { $value: unknown; $type?: string };
type W3cGroup = { [key: string]: W3cGroup | W3cToken };

function isToken(node: unknown): node is W3cToken {
	return typeof node === 'object' && node !== null && '$value' in (node as Record<string, unknown>);
}

function groupValues<T>(path: string): Record<string, T> {
	let node: W3cGroup | W3cToken = w3cTokens as unknown as W3cGroup;
	for (const seg of path.split('.')) {
		if (isToken(node)) break;
		const next = (node as W3cGroup)[seg];
		if (next === undefined) throw new Error(`missing group: ${path} (at "${seg}")`);
		node = next;
	}
	if (isToken(node)) throw new Error(`path is a token, not a group: ${path}`);
	const out: Record<string, T> = {};
	for (const [k, v] of Object.entries(node as W3cGroup)) {
		if (k.startsWith('$')) continue;
		if (isToken(v)) out[k] = v.$value as T;
	}
	return out;
}

const failures: string[] = [];
const checks = { total: 0, passed: 0 };

function expectEqual(label: string, actual: unknown, expected: unknown): void {
	checks.total += 1;
	if (actual === expected) {
		checks.passed += 1;
		return;
	}
	failures.push(`${label}\n    expected: ${String(expected)}\n    actual:   ${String(actual)}`);
}

const light = groupValues<string>('color.semantic.light');
const dark = groupValues<string>('color.semantic.dark');
const feedback = groupValues<string>('color.feedback');
const brand = Object.values(groupValues<string>('color.primitive.brand'));
const brandDark = Object.values(groupValues<string>('color.primitive.brandDark'));
const radius = groupValues<string>('dimension.radius');

// ---------------------------------------------------------------------------
// 1. No parallel palette may reappear in the source.
// ---------------------------------------------------------------------------
for (const banned of ['component', 'component.layout', 'component.dashboard']) {
	checks.total += 1;
	let exists = true;
	try {
		groupValues<string>(`color.${banned}`);
	} catch {
		exists = false;
	}
	if (exists) {
		failures.push(`color.${banned} is back — component tokens must be derived in build.ts, not declared`);
	} else {
		checks.passed += 1;
	}
}

// ---------------------------------------------------------------------------
// 2. tokens.css (--color-*) must carry the semantic values verbatim.
// ---------------------------------------------------------------------------
const tokensCss = readFileSync(join(distDir, 'tokens.css'), 'utf-8');
function cssVar(css: string, name: string, scope: 'root' | 'dark'): string | undefined {
	// `@theme { … }` holds the light defaults, `.dark { … }` the overrides.
	const block = scope === 'root' ? css.slice(0, css.indexOf('.dark {')) : css.slice(css.indexOf('.dark {'));
	const m = block.match(new RegExp(`--${name}:\\s*([^;]+);`));
	return m?.[1].trim();
}

const semanticToCssVar: Record<string, string> = {
	primary: 'color-primary',
	primaryHover: 'color-primary-hover',
	primaryActive: 'color-primary-active',
	primaryContent: 'color-primary-content',
	bg: 'color-bg',
	bgElevated: 'color-bg-elevated',
	card: 'color-card',
	item: 'color-item',
	text: 'color-text',
	textSecondary: 'color-text-secondary',
	textMuted: 'color-text-muted',
	border: 'color-border',
	brandText: 'color-brand-text',
	inputBorder: 'color-input-border',
};

for (const [key, cssName] of Object.entries(semanticToCssVar)) {
	expectEqual(`tokens.css light --${cssName}`, cssVar(tokensCss, cssName, 'root'), light[key]);
	expectEqual(`tokens.css dark  --${cssName}`, cssVar(tokensCss, cssName, 'dark'), dark[key]);
}

// ---------------------------------------------------------------------------
// 3. shadcn.css must expose the same values under the shadcn variable names.
// ---------------------------------------------------------------------------
const shadcnCss = readFileSync(join(distDir, 'shadcn.css'), 'utf-8');
const shadcnLight: Record<string, string> = {
	background: light.bg,
	foreground: light.text,
	card: light.card,
	primary: light.primary,
	secondary: light.item,
	muted: light.item,
	'muted-foreground': light.textMuted,
	border: light.border,
	input: light.inputBorder,
	ring: light.primary,
	'brand-text': light.brandText,
	success: feedback.success,
	warning: feedback.warning,
	info: feedback.info,
	destructive: feedback.error,
};
const shadcnDark: Record<string, string> = {
	background: dark.bg,
	foreground: dark.text,
	card: dark.card,
	primary: dark.primary,
	secondary: dark.item,
	muted: dark.item,
	'muted-foreground': dark.textMuted,
	border: dark.border,
	input: dark.inputBorder,
	ring: dark.primary,
	'brand-text': dark.brandText,
	success: feedback.successDark,
	warning: feedback.warningDark,
	info: feedback.infoDark,
	destructive: feedback.errorDark,
};
const shadcnRoot = shadcnCss.slice(0, shadcnCss.indexOf('.dark {'));
const shadcnDarkBlock = shadcnCss.slice(shadcnCss.indexOf('.dark {'));
for (const [name, expected] of Object.entries(shadcnLight)) {
	const m = shadcnRoot.match(new RegExp(`--${name}:\\s*([^;]+);`));
	expectEqual(`shadcn.css light --${name}`, m?.[1].trim(), expected);
}
for (const [name, expected] of Object.entries(shadcnDark)) {
	const m = shadcnDarkBlock.match(new RegExp(`--${name}:\\s*([^;]+);`));
	expectEqual(`shadcn.css dark  --${name}`, m?.[1].trim(), expected);
}

// ---------------------------------------------------------------------------
// 4. AntD ThemeConfig must map every semantic slot to the semantic value.
// ---------------------------------------------------------------------------
// antd.js imports `antd` for darkAlgorithm, which is not resolvable under plain
// node here; strip the import and the algorithm reference before evaluating.
const antdSrc = readFileSync(join(distDir, 'antd.js'), 'utf-8')
	.replace(/^import .*$/gm, '')
	.replace(/algorithm: antdTheme\.darkAlgorithm,/, 'algorithm: "dark",');
const antdModule = await import(`data:text/javascript;base64,${Buffer.from(antdSrc).toString('base64')}`);
const { lightTheme, darkTheme } = antdModule as {
	lightTheme: { token: Record<string, unknown> };
	darkTheme: { token: Record<string, unknown> };
};

const antdLightExpect: Record<string, unknown> = {
	colorPrimary: light.primary,
	colorPrimaryHover: light.primaryHover,
	colorPrimaryActive: light.primaryActive,
	colorBgBase: light.bg,
	colorBgLayout: light.bg,
	colorBgContainer: light.card,
	colorBgElevated: light.bgElevated,
	colorText: light.text,
	colorTextSecondary: light.textSecondary,
	colorTextTertiary: light.textMuted,
	colorBorder: light.border,
	colorSuccess: feedback.success,
	colorWarning: feedback.warning,
	colorError: feedback.error,
	colorInfo: feedback.info,
	colorLink: light.primary,
	borderRadius: parseFloat(radius.md) * 16,
};
const antdDarkExpect: Record<string, unknown> = {
	colorPrimary: dark.primary,
	colorBgBase: dark.bg,
	colorBgContainer: dark.card,
	colorText: dark.text,
	colorTextSecondary: dark.textSecondary,
	colorBorder: dark.border,
	colorSuccess: feedback.successDark,
	colorWarning: feedback.warningDark,
	colorError: feedback.errorDark,
	colorInfo: feedback.infoDark,
};
for (const [slot, expected] of Object.entries(antdLightExpect)) {
	expectEqual(`antd.js light token.${slot}`, lightTheme.token[slot], expected);
}
for (const [slot, expected] of Object.entries(antdDarkExpect)) {
	expectEqual(`antd.js dark  token.${slot}`, darkTheme.token[slot], expected);
}

// ---------------------------------------------------------------------------
// 5. dashboard.js / layout.js — the inline-style surface 30 pages consume.
// ---------------------------------------------------------------------------
const dashboardSrc = readFileSync(join(distDir, 'dashboard.js'), 'utf-8');
const dashboardModule = await import(`data:text/javascript;base64,${Buffer.from(dashboardSrc).toString('base64')}`);
const layoutSrc = readFileSync(join(distDir, 'layout.js'), 'utf-8');
const layoutModule = await import(`data:text/javascript;base64,${Buffer.from(layoutSrc).toString('base64')}`);
const { lightDashboardTokens, darkDashboardTokens } = dashboardModule as {
	lightDashboardTokens: Record<string, string>;
	darkDashboardTokens: Record<string, string>;
};
const { lightLayoutTokens, darkLayoutTokens } = layoutModule as {
	lightLayoutTokens: Record<string, string>;
	darkLayoutTokens: Record<string, string>;
};

const dashboardLightExpect: Record<string, string> = {
	headingColor: light.text,
	secondaryColor: light.textSecondary,
	mutedColor: light.textMuted,
	placeholderColor: light.inputBorder,
	successColor: feedback.success,
	warningColor: feedback.warning,
	errorColor: feedback.error,
	infoColor: feedback.info,
	purpleColor: light.primary,
	avatarBg: brand[1],
};
const dashboardDarkExpect: Record<string, string> = {
	headingColor: dark.text,
	secondaryColor: dark.textSecondary,
	mutedColor: dark.textMuted,
	placeholderColor: dark.inputBorder,
	successColor: feedback.successDark,
	warningColor: feedback.warningDark,
	errorColor: feedback.errorDark,
	infoColor: feedback.infoDark,
	purpleColor: dark.primary,
	avatarBg: brandDark[1],
};
for (const [key, expected] of Object.entries(dashboardLightExpect)) {
	expectEqual(`dashboard.js light ${key}`, lightDashboardTokens[key], expected);
}
for (const [key, expected] of Object.entries(dashboardDarkExpect)) {
	expectEqual(`dashboard.js dark  ${key}`, darkDashboardTokens[key], expected);
}

const layoutLightExpect: Record<string, string> = {
	headerBg: light.bg,
	headerColor: light.text,
	headerColorSecondary: light.textSecondary,
	siderBg: light.card,
	bodyBg: light.bg,
	colorTextMenuSelected: light.primary,
	colorBgMenuItemSelected: brand[1],
};
const layoutDarkExpect: Record<string, string> = {
	headerBg: dark.bg,
	headerColor: dark.text,
	headerColorSecondary: dark.textSecondary,
	siderBg: dark.bg,
	bodyBg: dark.bg,
	colorTextMenuSelected: dark.primary,
	colorBgMenuItemSelected: brandDark[1],
};
for (const [key, expected] of Object.entries(layoutLightExpect)) {
	expectEqual(`layout.js light ${key}`, lightLayoutTokens[key], expected);
}
for (const [key, expected] of Object.entries(layoutDarkExpect)) {
	expectEqual(`layout.js dark  ${key}`, darkLayoutTokens[key], expected);
}

// ---------------------------------------------------------------------------
// Report
// ---------------------------------------------------------------------------
if (failures.length > 0) {
	console.error(`\n[tokens] parity check FAILED — ${failures.length}/${checks.total} assertions\n`);
	for (const f of failures) console.error(`  ✗ ${f}\n`);
	process.exit(1);
}
console.log(
	`[tokens] parity check passed — ${checks.passed}/${checks.total} assertions across tokens.css, shadcn.css, antd.js, layout.js, dashboard.js`,
);
