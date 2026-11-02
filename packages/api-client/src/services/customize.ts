import { get, post, postPaginated } from '../client';

/** 片段类型。css 低风险直接生效；js / html 在机器写入时默认落草稿。 */
export type SnippetKind = 'css' | 'js' | 'html';
/** 注入位置。html 一律落 body_end（服务端归一化）。 */
export type SnippetPosition = 'head' | 'body_end';

export type Snippet = {
	id: string;
	kind: SnippetKind;
	position: SnippetPosition;
	content: string;
	enabled: boolean;
	sort: number;
	updated_by?: string;
	created_at: string;
	updated_at: string;
};

export type ThemeTokenMode = 'all' | 'light' | 'dark';

export type ThemeToken = {
	id: string;
	token: string;
	value: string;
	mode: ThemeTokenMode;
	updated_by?: string;
	created_at: string;
	updated_at: string;
};

/** 导航项可见性。level 时需要同时给 min_level。 */
export type NavVisibility = 'public' | 'login' | 'level';

export type NavItem = {
	id: string;
	label: string;
	href: string;
	icon?: string;
	sort: number;
	visibility: NavVisibility;
	min_level: number;
	enabled: boolean;
	updated_by?: string;
	created_at: string;
	updated_at: string;
};

export type HomeModuleKind = 'latest' | 'hot' | 'spaces' | 'html';

export type HomeModule = {
	id: string;
	kind: HomeModuleKind;
	title?: string;
	config?: string;
	sort: number;
	visibility: NavVisibility;
	min_level: number;
	enabled: boolean;
	updated_by?: string;
	created_at: string;
	updated_at: string;
};

export type CustomPageStatus = 'draft' | 'published';

export type CustomPage = {
	id: string;
	slug: string;
	title: string;
	content: string;
	status: CustomPageStatus;
	updated_by?: string;
	created_at: string;
	updated_at: string;
};

/**
 * 自定义写入的统一返回。
 *
 * `applied` 是这套 API 最关键的字段：它告诉调用方（尤其是 AI）
 * 这次写入到底生效了没有。JS/HTML 类内容在机器写入且凭证未持
 * `customize:publish` 时会是 `false`，并附带 `reason`。
 */
export type CustomizeWriteResult<T> = {
	applied: boolean;
	reason?: string;
} & T;

/** 前台聚合：导航、首页模块、生效片段与主题覆盖。已按可见性过滤。 */
export type PublicCustomization = {
	nav: NavItem[];
	home_modules: HomeModule[];
	head_snippets: Snippet[];
	body_snippets: Snippet[];
	theme_tokens: ThemeToken[];
};

export const customizeService = {
	// ── 片段 ──────────────────────────────────────────────
	listSnippets() {
		return post<Snippet[]>('/admin/customization/snippets/list', {});
	},
	createSnippet(body: Partial<Snippet>) {
		return post<CustomizeWriteResult<{ snippet: Snippet }>>('/admin/customization/snippets/create', body);
	},
	updateSnippet(id: string, body: Partial<Snippet>) {
		return post<CustomizeWriteResult<{ snippet: Snippet }>>(`/admin/customization/snippets/${id}/update`, body);
	},
	deleteSnippet(id: string) {
		return post<null>(`/admin/customization/snippets/${id}/delete`, {});
	},
	toggleSnippet(id: string, enabled: boolean) {
		return post<null>(`/admin/customization/snippets/${id}/toggle`, { enabled });
	},

	// ── 主题 token ────────────────────────────────────────
	listThemeTokens() {
		return post<ThemeToken[]>('/admin/customization/tokens/list', {});
	},
	setThemeTokens(items: Array<Pick<ThemeToken, 'token' | 'value' | 'mode'>>) {
		return post<{ applied: boolean; reason?: string }>('/admin/customization/tokens/set', { items });
	},
	deleteThemeToken(token: string) {
		return post<null>(`/admin/customization/tokens/${encodeURIComponent(token)}/delete`, {});
	},
	clearThemeTokens() {
		return post<null>('/admin/customization/tokens/clear', {});
	},

	// ── 导航 ──────────────────────────────────────────────
	listNavItems() {
		return post<NavItem[]>('/admin/customization/nav/list', {});
	},
	createNavItem(body: Partial<NavItem>) {
		return post<CustomizeWriteResult<{ item: NavItem }>>('/admin/customization/nav/create', body);
	},
	updateNavItem(id: string, body: Partial<NavItem>) {
		return post<CustomizeWriteResult<{ item: NavItem }>>(`/admin/customization/nav/${id}/update`, body);
	},
	deleteNavItem(id: string) {
		return post<null>(`/admin/customization/nav/${id}/delete`, {});
	},

	// ── 首页模块 ──────────────────────────────────────────
	listHomeModules() {
		return post<HomeModule[]>('/admin/customization/modules/list', {});
	},
	createHomeModule(body: Partial<HomeModule>) {
		return post<CustomizeWriteResult<{ module: HomeModule }>>('/admin/customization/modules/create', body);
	},
	updateHomeModule(id: string, body: Partial<HomeModule>) {
		return post<CustomizeWriteResult<{ module: HomeModule }>>(`/admin/customization/modules/${id}/update`, body);
	},
	deleteHomeModule(id: string) {
		return post<null>(`/admin/customization/modules/${id}/delete`, {});
	},

	// ── 自定义页 ──────────────────────────────────────────
	listCustomPages(body: { page?: number; pageSize?: number } = {}) {
		return postPaginated<CustomPage[]>('/admin/customization/pages/list', body);
	},
	createCustomPage(body: Partial<CustomPage>) {
		return post<CustomizeWriteResult<{ page: CustomPage }>>('/admin/customization/pages/create', body);
	},
	updateCustomPage(id: string, body: Partial<CustomPage>) {
		return post<CustomizeWriteResult<{ page: CustomPage }>>(`/admin/customization/pages/${id}/update`, body);
	},
	deleteCustomPage(id: string) {
		return post<null>(`/admin/customization/pages/${id}/delete`, {});
	},
	publishCustomPage(id: string, publish: boolean) {
		return post<CustomizeWriteResult<{ page: CustomPage }>>(`/admin/customization/pages/${id}/publish`, {
			publish,
		});
	},

	// ── 前台 ──────────────────────────────────────────────
	/** 公开读取一个已发布的自定义页。草稿会返回 404。 */
	getPublicPage(slug: string) {
		return get<{ slug: string; title: string; content: string; updated_at: string }>(
			`/pages/${encodeURIComponent(slug)}`,
		);
	},
	/** 前台聚合：导航 / 首页模块 / 生效片段 / 主题覆盖。 */
	getPublicCustomization() {
		return post<PublicCustomization>('/customization/public', {});
	},
};
