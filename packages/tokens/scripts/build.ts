/**
 * @orinforum/tokens build script — W3C Design Tokens (DTCG) driven.
 *
 * w3c-tokens.json is the single source of truth. All outputs are derived
 * from `$value` / `$type` entries; no hand-written token values live here.
 *
 * Run: pnpm --filter @orinforum/tokens tokens:build
 */
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import w3cTokens from '../w3c-tokens.json' with { type: 'json' };

const __dirname = dirname(fileURLToPath(import.meta.url));
const distDir = join(__dirname, '..', 'dist');
mkdirSync(distDir, { recursive: true });

// ---------------------------------------------------------------------------
// W3C DTCG accessors
// ---------------------------------------------------------------------------

type W3cToken = { $value: unknown; $type?: string };
type W3cGroup = { [key: string]: W3cGroup | W3cToken };

function isToken(node: unknown): node is W3cToken {
	return typeof node === 'object' && node !== null && '$value' in (node as Record<string, unknown>);
}

/** Flatten a group into { key: $value } pairs (one level). */
function groupValues<T>(path: string): Record<string, T> {
	let node: W3cGroup | W3cToken = w3cTokens as unknown as W3cGroup;
	for (const seg of path.split('.')) {
		if (isToken(node)) break;
		const next: W3cGroup | W3cToken = (node as W3cGroup)[seg];
		if (next === undefined) {
			throw new Error(`[tokens] missing W3C group path: ${path} (at "${seg}")`);
		}
		node = next;
	}
	if (isToken(node)) {
		throw new Error(`[tokens] path resolves to a token, not a group: ${path}`);
	}
	const out: Record<string, T> = {};
	for (const [k, v] of Object.entries(node as W3cGroup)) {
		if (k.startsWith('$')) continue;
		if (isToken(v)) out[k] = v.$value as T;
	}
	return out;
}

// ---------------------------------------------------------------------------
// Value serializers
// ---------------------------------------------------------------------------

interface W3cShadowLayer {
	color: string;
	offsetX: string;
	offsetY: string;
	blur: string;
	spread: string;
}

function shadowToCss(value: W3cShadowLayer | W3cShadowLayer[]): string {
	const layers = Array.isArray(value) ? value : [value];
	return layers.map((l) => `${l.offsetX} ${l.offsetY} ${l.blur} ${l.spread} ${l.color}`).join(', ');
}

function fontFamilyToCss(value: string | string[]): string {
	const list = Array.isArray(value) ? value : [value];
	return list.map((f) => (/[\s]/.test(f) && !f.startsWith('"') ? `"${f}"` : f)).join(', ');
}

/** camelCase / mixed key -> kebab-case for CSS custom property names */
function kebab(key: string): string {
	return key.replace(/([A-Z])/g, '-$1').toLowerCase();
}

/** Convert a rem/px dimension token to a px number for AntD borderRadius. */
function dimensionToPx(value: string): number {
	if (value.endsWith('rem')) return parseFloat(value) * 16;
	if (value.endsWith('px')) return parseFloat(value);
	return parseFloat(value);
}

// ---------------------------------------------------------------------------
// Resolve token groups from the W3C document
// ---------------------------------------------------------------------------

const brandPalette = Object.values(groupValues<string>('color.primitive.brand'));
const brandDarkPalette = Object.values(groupValues<string>('color.primitive.brandDark'));

const seedColors = groupValues<string>('color.seed');
const lightSemantic = groupValues<string>('color.semantic.light');
const darkSemantic = groupValues<string>('color.semantic.dark');
const feedback = groupValues<string>('color.feedback');

const radius = groupValues<string>('dimension.radius');
const spacing = groupValues<string>('dimension.spacing');
const duration = groupValues<string>('duration');
const zIndex = groupValues<number>('number.zIndex');

const shadows = groupValues<W3cShadowLayer | W3cShadowLayer[]>('shadow');
const fontFamilies = groupValues<string | string[]>('fontFamily');

// ---------------------------------------------------------------------------
// Component-level tokens are DERIVED, never hand-written.
//
// They used to live in `color.component.*` as a parallel palette and had
// drifted from the semantic layer (e.g. mutedColor #9a9590 vs textMuted
// #6f6b68). Deriving them here means the semantic layer is the only place a
// colour is declared, and scripts/check-parity.ts can prove the CSS, AntD and
// dashboard projections all resolve to the same value.
// ---------------------------------------------------------------------------

const fontSans = fontFamilyToCss(fontFamilies.sans);

const layoutLight = {
	headerBg: lightSemantic.bg,
	headerColor: lightSemantic.text,
	headerColorSecondary: lightSemantic.textSecondary,
	siderBg: lightSemantic.card,
	bodyBg: lightSemantic.bg,
	colorTextMenuSelected: lightSemantic.primary,
	colorBgMenuItemSelected: brandPalette[1],
};

const layoutDark = {
	headerBg: darkSemantic.bg,
	headerColor: darkSemantic.text,
	headerColorSecondary: darkSemantic.textSecondary,
	siderBg: darkSemantic.bg,
	bodyBg: darkSemantic.bg,
	colorTextMenuSelected: darkSemantic.primary,
	colorBgMenuItemSelected: brandDarkPalette[1],
};

/** Dashboard page-level colours — all aliases of semantic tokens. */
const dashboardLight = {
	headingColor: lightSemantic.text,
	secondaryColor: lightSemantic.textSecondary,
	mutedColor: lightSemantic.textMuted,
	placeholderColor: lightSemantic.inputBorder,
	successColor: feedback.success,
	warningColor: feedback.warning,
	errorColor: feedback.error,
	infoColor: feedback.info,
	purpleColor: lightSemantic.primary,
	avatarBg: brandPalette[1],
	bgGrad: `linear-gradient(180deg, ${lightSemantic.bg} 0%, ${lightSemantic.card} 55%, ${lightSemantic.item} 100%)`,
};

const dashboardDark = {
	headingColor: darkSemantic.text,
	secondaryColor: darkSemantic.textSecondary,
	mutedColor: darkSemantic.textMuted,
	placeholderColor: darkSemantic.inputBorder,
	successColor: feedback.successDark,
	warningColor: feedback.warningDark,
	errorColor: feedback.errorDark,
	infoColor: feedback.infoDark,
	purpleColor: darkSemantic.primary,
	avatarBg: brandDarkPalette[1],
	bgGrad: `linear-gradient(180deg, ${darkSemantic.bg} 0%, ${darkSemantic.card} 60%, ${darkSemantic.bg} 100%)`,
};

// ---------------------------------------------------------------------------
// 1. tokens.css — Tailwind v4 @theme + .dark
// ---------------------------------------------------------------------------

function semanticToCssVars(colors: Record<string, string>): string {
	return Object.entries(colors)
		.map(([key, value]) => `  --color-${kebab(key)}: ${value};`)
		.join('\n');
}

function buildTokensCss(): string {
	const primitiveColors = brandPalette.map((c, i) => `  --color-brand-${i + 1}: ${c};`).join('\n');
	const primitiveDarkColors = brandDarkPalette.map((c, i) => `  --color-brand-dark-${i + 1}: ${c};`).join('\n');

	const seedVars = Object.entries(seedColors)
		.map(([k, v]) => `  --color-seed-${kebab(k)}: ${v};`)
		.join('\n');

	const feedbackVars = Object.entries(feedback)
		.map(([k, v]) => `  --color-${kebab(k)}: ${v};`)
		.join('\n');

	const radiusVars = Object.entries(radius)
		.map(([k, v]) => `  --radius-${kebab(k)}: ${v};`)
		.join('\n');

	const shadowVars = Object.entries(shadows)
		.map(([k, v]) => `  --shadow-${kebab(k)}: ${shadowToCss(v)};`)
		.join('\n');

	const spacingVars = Object.entries(spacing)
		.map(([k, v]) => `  --spacing-${kebab(k)}: ${v};`)
		.join('\n');

	const durationVars = Object.entries(duration)
		.map(([k, v]) => `  --duration-${kebab(k)}: ${v};`)
		.join('\n');

	const zVars = Object.entries(zIndex)
		.map(([k, v]) => `  --z-${kebab(k)}: ${v};`)
		.join('\n');

	const fontVars = Object.entries(fontFamilies)
		.map(([k, v]) => `  --font-${kebab(k)}: ${fontFamilyToCss(v)};`)
		.join('\n');

	return `/* @orinforum/tokens — AUTO-GENERATED from w3c-tokens.json, do not edit manually.
   Run: pnpm --filter @orinforum/tokens tokens:build */

@theme {
  /* Brand primitive palette (light) */
${primitiveColors}

  /* Brand primitive palette (dark background) */
${primitiveDarkColors}

  /* Seed colors */
${seedVars}

  /* Feedback colors */
${feedbackVars}

  /* Semantic colors (light defaults) */
${semanticToCssVars(lightSemantic)}

  /* Radius */
${radiusVars}

  /* Shadow */
${shadowVars}

  /* Spacing */
${spacingVars}

  /* Duration */
${durationVars}

  /* Z-index */
${zVars}

  /* Font family */
${fontVars}
}

.dark {
  /* Semantic overrides for dark mode */
${semanticToCssVars(darkSemantic)}
}
`;
}

// ---------------------------------------------------------------------------
// 3. shadcn.css — shadcn/ui CSS variable convention (Tailwind v4)
//
// Mirrors the semantic light/dark values already defined in w3c-tokens.json onto
// the variable names shadcn/ui components expect. `@theme inline` then exposes
// them as Tailwind color utilities (bg-background, text-muted-foreground, ...).
// ---------------------------------------------------------------------------

function buildShadcnCss(): string {
	const l = lightSemantic;
	const d = darkSemantic;

	return `/* @orinforum/tokens - shadcn/ui theme vars — AUTO-GENERATED from w3c-tokens.json, do not edit manually.
   Run: pnpm --filter @orinforum/tokens tokens:build */

:root {
  --background: ${l.bg};
  --foreground: ${l.text};
  --card: ${l.card};
  --card-foreground: ${l.text};
  --popover: ${l.bgElevated};
  --popover-foreground: ${l.text};
  --primary: ${l.primary};
  --primary-foreground: ${l.primaryContent};
  --secondary: ${l.item};
  --secondary-foreground: ${l.text};
  --muted: ${l.item};
  --muted-foreground: ${l.textMuted};
  --accent: ${brandPalette[1]};
  --accent-foreground: ${l.primaryActive};
  --destructive: ${feedback.error};
  --destructive-foreground: #ffffff;
  --border: ${l.border};
  /* 表单控件边界是形状的唯一指示，必须 ≥3:1 非文本对比，因此比 --border 更深 */
  --input: ${l.inputBorder};
  --ring: ${l.primary};
  /* 品牌文字色：主色在浅底上作为小字/链接时对比不足，单独给一支 */
  --brand-text: ${l.brandText};
  --success: ${feedback.success};
  --warning: ${feedback.warning};
  --info: ${feedback.info};

  --chart-1: ${brandPalette[2]};
  --chart-2: ${brandPalette[4]};
  --chart-3: ${brandPalette[6]};
  --chart-4: ${brandPalette[8]};
  --chart-5: ${feedback.info};

  --sidebar: ${layoutLight.siderBg};
  --sidebar-foreground: ${l.text};
  --sidebar-primary: ${l.primary};
  --sidebar-primary-foreground: ${l.primaryContent};
  --sidebar-accent: ${brandPalette[1]};
  --sidebar-accent-foreground: ${l.primaryActive};
  --sidebar-border: ${l.border};
  --sidebar-ring: ${l.primary};

  --radius: ${radius.md};

  /* 登录/首改密页的整屏背景：复合值，只能整段作为变量下发 */
  --dashboard-bg-gradient: ${dashboardLight.bgGrad};
}

.dark {
  --background: ${d.bg};
  --foreground: ${d.text};
  --card: ${d.card};
  --card-foreground: ${d.text};
  --popover: ${d.bgElevated};
  --popover-foreground: ${d.text};
  --primary: ${d.primary};
  --primary-foreground: ${d.primaryContent};
  --secondary: ${d.item};
  --secondary-foreground: ${d.text};
  --muted: ${d.item};
  --muted-foreground: ${d.textMuted};
  --accent: ${brandDarkPalette[1]};
  --accent-foreground: ${brandDarkPalette[8]};
  --border: ${d.border};
  --input: ${d.inputBorder};
  --ring: ${d.primary};
  --brand-text: ${d.brandText};
  --success: ${feedback.successDark};
  --warning: ${feedback.warningDark};
  --info: ${feedback.infoDark};
  --destructive: ${feedback.errorDark};

  --chart-1: ${brandDarkPalette[2]};
  --chart-2: ${brandDarkPalette[4]};
  --chart-3: ${brandDarkPalette[6]};
  --chart-4: ${brandDarkPalette[8]};
  --chart-5: ${feedback.infoDark};

  --sidebar: ${layoutDark.siderBg};
  --sidebar-foreground: ${d.text};
  --sidebar-primary: ${d.primary};
  --sidebar-primary-foreground: ${d.primaryContent};
  --sidebar-accent: ${brandDarkPalette[1]};
  --sidebar-accent-foreground: ${brandDarkPalette[8]};
  --sidebar-border: ${d.border};
  --sidebar-ring: ${d.primary};

  --dashboard-bg-gradient: ${dashboardDark.bgGrad};
}

@theme inline {
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-card: var(--card);
  --color-card-foreground: var(--card-foreground);
  --color-popover: var(--popover);
  --color-popover-foreground: var(--popover-foreground);
  --color-primary: var(--primary);
  --color-primary-foreground: var(--primary-foreground);
  --color-secondary: var(--secondary);
  --color-secondary-foreground: var(--secondary-foreground);
  --color-muted: var(--muted);
  --color-muted-foreground: var(--muted-foreground);
  --color-accent: var(--accent);
  --color-accent-foreground: var(--accent-foreground);
  --color-destructive: var(--destructive);
  --color-destructive-foreground: var(--destructive-foreground);
  --color-border: var(--border);
  --color-input: var(--input);
  --color-ring: var(--ring);
  --color-brand-text: var(--brand-text);
  --color-success: var(--success);
  --color-warning: var(--warning);
  --color-info: var(--info);
  --color-chart-1: var(--chart-1);
  --color-chart-2: var(--chart-2);
  --color-chart-3: var(--chart-3);
  --color-chart-4: var(--chart-4);
  --color-chart-5: var(--chart-5);
  --color-sidebar: var(--sidebar);
  --color-sidebar-foreground: var(--sidebar-foreground);
  --color-sidebar-primary: var(--sidebar-primary);
  --color-sidebar-primary-foreground: var(--sidebar-primary-foreground);
  --color-sidebar-accent: var(--sidebar-accent);
  --color-sidebar-accent-foreground: var(--sidebar-accent-foreground);
  --color-sidebar-border: var(--sidebar-border);
  --color-sidebar-ring: var(--sidebar-ring);
}
`;
}

// ---------------------------------------------------------------------------
// 4. antd.js / antd.d.ts — AntD ThemeConfig objects
// ---------------------------------------------------------------------------

function buildAntdJs(): string {
	const l = lightSemantic;
	const d = darkSemantic;
	const borderRadius = dimensionToPx(radius.md);
	const radiusLg = dimensionToPx(radius.lg);
	const radiusSm = dimensionToPx(radius.sm);

	/** Shared token mapping; `fb` picks the light or dark feedback tier. */
	const tokenBlock = (t: Record<string, string>, fb: Record<string, string>) => `    colorPrimary: '${t.primary}',
    colorPrimaryHover: '${t.primaryHover}',
    colorPrimaryActive: '${t.primaryActive}',
    colorBgBase: '${t.bg}',
    colorBgLayout: '${t.bg}',
    colorBgContainer: '${t.card}',
    colorBgElevated: '${t.bgElevated}',
    colorBgSpotlight: '${t.bgElevated}',
    colorTextBase: '${t.text}',
    colorText: '${t.text}',
    colorTextSecondary: '${t.textSecondary}',
    colorTextTertiary: '${t.textMuted}',
    colorTextQuaternary: '${t.textMuted}',
    colorTextPlaceholder: '${t.inputBorder}',
    colorBorder: '${t.border}',
    colorBorderSecondary: '${t.border}',
    colorFill: '${t.item}',
    colorFillSecondary: '${t.item}',
    colorFillTertiary: '${t.item}',
    colorFillQuaternary: '${t.bgElevated}',
    colorSplit: '${t.border}',
    colorSuccess: '${fb.success}',
    colorWarning: '${fb.warning}',
    colorError: '${fb.error}',
    colorInfo: '${fb.info}',
    colorLink: '${t.primary}',
    colorLinkHover: '${t.primaryHover}',
    colorLinkActive: '${t.primaryActive}',
    colorLinkDecoration: 'none',
    colorTextLightSolid: '${t.primaryContent}',
    borderRadius: ${borderRadius},
    borderRadiusLG: ${radiusLg},
    borderRadiusSM: ${radiusSm},
    fontFamily: \`${fontSans}\`,
    fontSize: 14,
    /* 控件几何对齐主站 shadcn 按钮：h-9=36 / h-11=44 / h-8=32。
       antd 默认 controlHeight 是 32，后台表格与表单会比前台矮一截。 */
    controlHeight: 36,
    controlHeightLG: 40,
    controlHeightSM: 28,
    lineHeight: 1.5715,`;

	const componentBlock = (lay: Record<string, string>, t: Record<string, string>) => `    Layout: {
      headerBg: '${lay.headerBg}',
      headerColor: '${lay.headerColor}',
      headerColorSecondary: '${lay.headerColorSecondary}',
      siderBg: '${lay.siderBg}',
      bodyBg: '${lay.bodyBg}',
      colorTextMenuSelected: '${lay.colorTextMenuSelected}',
      colorBgMenuItemSelected: '${lay.colorBgMenuItemSelected}',
    },
    Menu: {
      itemBg: '${lay.siderBg}',
      itemColor: '${t.textSecondary}',
      itemHoverBg: '${t.item}',
      itemHoverColor: '${t.text}',
      itemSelectedBg: '${lay.colorBgMenuItemSelected}',
      itemSelectedColor: '${lay.colorTextMenuSelected}',
      subMenuItemBg: '${lay.siderBg}',
      activeBarBorderWidth: 0,
    },
    Table: {
      headerBg: '${t.item}',
      headerColor: '${t.text}',
      headerSplitColor: '${t.border}',
      rowHoverBg: '${t.item}',
      borderColor: '${t.border}',
    },
    Card: {
      headerBg: 'transparent',
      colorBorderSecondary: '${t.border}',
    },
    Tabs: {
      itemColor: '${t.textSecondary}',
      itemSelectedColor: '${t.primary}',
      itemHoverColor: '${t.primaryHover}',
      inkBarColor: '${t.primary}',
    },
    Button: {
      defaultBg: '${t.card}',
      defaultColor: '${t.text}',
      defaultBorderColor: '${t.border}',
      defaultHoverBg: '${t.item}',
      defaultHoverColor: '${t.primary}',
      defaultHoverBorderColor: '${t.primaryHover}',
    },
    Input: {
      colorBgContainer: '${t.card}',
      activeBorderColor: '${t.primary}',
      hoverBorderColor: '${t.primaryHover}',
      activeShadow: 'none',
    },
    Select: {
      optionSelectedBg: '${t.item}',
      optionActiveBg: '${t.bgElevated}',
    },
    Tag: {
      defaultBg: '${t.item}',
      defaultColor: '${t.textSecondary}',
    },
    Alert: {
      defaultBg: '${t.item}',
    },
    Modal: {
      contentBg: '${t.card}',
      headerBg: '${t.card}',
    },
    Divider: {
      colorSplit: '${t.border}',
    },
    Typography: {
      colorTextHeading: '${t.text}',
      colorTextDescription: '${t.textMuted}',
    },
    Tooltip: {
      colorBgSpotlight: '${t.bgElevated}',
      colorTextLightSolid: '${t.text}',
    },
    Statistic: {
      contentFontSize: 24,
      titleFontSize: 14,
    },`;

	return `/* @orinforum/tokens - AntD ThemeConfig — AUTO-GENERATED from w3c-tokens.json, do not edit manually.
   Run: pnpm --filter @orinforum/tokens tokens:build */

import { theme as antdTheme } from 'antd';

export const lightTheme = {
  token: {
${tokenBlock(l, feedback)}
  },
  components: {
${componentBlock(layoutLight, l)}
  },
};

export const darkTheme = {
  algorithm: antdTheme.darkAlgorithm,
  token: {
${tokenBlock(d, {
	success: feedback.successDark,
	warning: feedback.warningDark,
	error: feedback.errorDark,
	info: feedback.infoDark,
})}
  },
  components: {
${componentBlock(layoutDark, d)}
  },
};

/** Select theme based on dark mode — zero runtime derivation, just object selection */
export function getAntdTheme(isDark) {
  return isDark ? darkTheme : lightTheme;
}
`;
}

function buildAntdDts(): string {
	return `import type { ThemeConfig } from 'antd';

export declare const lightTheme: ThemeConfig;
export declare const darkTheme: ThemeConfig;
export declare function getAntdTheme(isDark: boolean): ThemeConfig;
`;
}

// ---------------------------------------------------------------------------
// 5. layout.js / layout.d.ts
// ---------------------------------------------------------------------------

function buildLayoutJs(): string {
	return `/* @orinforum/tokens - Layout tokens — AUTO-GENERATED from w3c-tokens.json, do not edit manually.
   Run: pnpm --filter @orinforum/tokens tokens:build */

export const lightLayoutTokens = {
  headerBg: '${layoutLight.headerBg}',
  headerColor: '${layoutLight.headerColor}',
  headerColorSecondary: '${layoutLight.headerColorSecondary}',
  siderBg: '${layoutLight.siderBg}',
  bodyBg: '${layoutLight.bodyBg}',
  colorTextMenuSelected: '${layoutLight.colorTextMenuSelected}',
  colorBgMenuItemSelected: '${layoutLight.colorBgMenuItemSelected}',
};

export const darkLayoutTokens = {
  headerBg: '${layoutDark.headerBg}',
  headerColor: '${layoutDark.headerColor}',
  headerColorSecondary: '${layoutDark.headerColorSecondary}',
  siderBg: '${layoutDark.siderBg}',
  bodyBg: '${layoutDark.bodyBg}',
  colorTextMenuSelected: '${layoutDark.colorTextMenuSelected}',
  colorBgMenuItemSelected: '${layoutDark.colorBgMenuItemSelected}',
};

/** Select layout tokens based on dark mode */
export function getLayoutTokens(isDark) {
  return isDark ? darkLayoutTokens : lightLayoutTokens;
}
`;
}

function buildLayoutDts(): string {
	return `export declare const lightLayoutTokens: {
  headerBg: string;
  headerColor: string;
  headerColorSecondary: string;
  siderBg: string;
  bodyBg: string;
  colorTextMenuSelected: string;
  colorBgMenuItemSelected: string;
};

export declare const darkLayoutTokens: {
  headerBg: string;
  headerColor: string;
  headerColorSecondary: string;
  siderBg: string;
  bodyBg: string;
  colorTextMenuSelected: string;
  colorBgMenuItemSelected: string;
};

export declare function getLayoutTokens(isDark: boolean): typeof lightLayoutTokens;
`;
}

// ---------------------------------------------------------------------------
// 6. dashboard.js / dashboard.d.ts
// ---------------------------------------------------------------------------

const DASHBOARD_KEYS = [
	'headingColor',
	'secondaryColor',
	'mutedColor',
	'placeholderColor',
	'successColor',
	'warningColor',
	'errorColor',
	'infoColor',
	'purpleColor',
	'bgGrad',
	'avatarBg',
] as const;

function buildDashboardJs(): string {
	const lightEntries = DASHBOARD_KEYS.map((k) => `  ${k}: '${dashboardLight[k]}',`).join('\n');
	const darkEntries = DASHBOARD_KEYS.map((k) => `  ${k}: '${dashboardDark[k]}',`).join('\n');

	return `/* @orinforum/tokens - Dashboard tokens — AUTO-GENERATED from w3c-tokens.json, do not edit manually.
   Run: pnpm --filter @orinforum/tokens tokens:build */

export const lightDashboardTokens = {
${lightEntries}
};

export const darkDashboardTokens = {
${darkEntries}
};

/** Select dashboard tokens based on dark mode */
export function getDashboardTokens(isDark) {
  return isDark ? darkDashboardTokens : lightDashboardTokens;
}
`;
}

function buildDashboardDts(): string {
	const shape = DASHBOARD_KEYS.map((k) => `  ${k}: string;`).join('\n');
	return `export declare const lightDashboardTokens: {
${shape}
};

export declare const darkDashboardTokens: {
${shape}
};

export declare function getDashboardTokens(isDark: boolean): typeof lightDashboardTokens;
`;
}

// ---------------------------------------------------------------------------
// Write all outputs
// ---------------------------------------------------------------------------

const outputs: Array<[string, string]> = [
	['tokens.css', buildTokensCss()],
	['shadcn.css', buildShadcnCss()],
	['antd.js', buildAntdJs()],
	['antd.d.ts', buildAntdDts()],
	['layout.js', buildLayoutJs()],
	['layout.d.ts', buildLayoutDts()],
	['dashboard.js', buildDashboardJs()],
	['dashboard.d.ts', buildDashboardDts()],
];

for (const [name, content] of outputs) {
	writeFileSync(join(distDir, name), content, 'utf-8');
}

console.log('@orinforum/tokens built successfully (W3C DTCG driven)');
for (const [name] of outputs) {
	console.log(`  -> dist/${name}`);
}
