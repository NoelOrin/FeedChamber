import type { PublicCustomization } from '@feedchamber/api-client';
import { customizeService } from '@feedchamber/api-client';
import { cache } from 'react';

import '~/lib/api/adapter';

const EMPTY: PublicCustomization = {
	nav: [],
	home_modules: [],
	head_snippets: [],
	body_snippets: [],
	theme_tokens: [],
};

/**
 * RSC 侧读取站点自定义配置（导航 / 首页模块 / 生效片段 / 主题覆盖）。
 *
 * 与 `loadPublicSettings` 同样用 `cache()` 包一层：同一次请求内
 * layout 与 page 会各取一次，包住后只发一次后端调用。
 * 失败一律降级成空值——自定义配置是装饰层，读不到不能拖垮整站。
 */
export const loadPublicCustomization = cache(async (): Promise<PublicCustomization> => {
	try {
		const data = await customizeService.getPublicCustomization();
		return {
			nav: data?.nav ?? [],
			home_modules: data?.home_modules ?? [],
			head_snippets: data?.head_snippets ?? [],
			body_snippets: data?.body_snippets ?? [],
			theme_tokens: data?.theme_tokens ?? [],
		};
	} catch {
		return EMPTY;
	}
});

/**
 * 把主题 token 覆盖生成为一段 CSS 变量声明。
 *
 * `all` 模式同时写进 `:root` 与 `.dark`；`light` / `dark` 只写对应选择器。
 * 输出刻意只允许 CSS 变量名与值，不接受任意选择器——任意 CSS 是 snippets
 * 的职责，token 覆盖只负责「改色改圆角」，不该成为第二个注入面。
 */
export function buildThemeOverrideCss(tokens: PublicCustomization['theme_tokens']): string {
	if (!tokens || tokens.length === 0) return '';

	// 只允许合法 CSS 自定义属性名，防止把 `}` 之类的注入进声明块。
	const safeName = (name: string) => /^--[a-zA-Z0-9-_]+$/.test(name);
	// 值里禁掉能提前闭合声明块或选择器的字符。
	const safeValue = (value: string) => !/[;{}]/.test(value);

	const root: string[] = [];
	const dark: string[] = [];

	for (const t of tokens) {
		if (!safeName(t.token) || !safeValue(t.value)) continue;
		const decl = `${t.token}:${t.value};`;
		if (t.mode === 'light') root.push(decl);
		else if (t.mode === 'dark') dark.push(decl);
		else {
			root.push(decl);
			dark.push(decl);
		}
	}

	const parts: string[] = [];
	if (root.length) parts.push(`:root{${root.join('')}}`);
	if (dark.length) parts.push(`.dark{${dark.join('')}}`);
	return parts.join('\n');
}

/** 把一组已启用片段拼成可注入的文本。 */
export function joinSnippets(snippets: { content: string }[] | undefined): string {
	if (!snippets || snippets.length === 0) return '';
	return snippets.map((s) => s.content).join('\n');
}
