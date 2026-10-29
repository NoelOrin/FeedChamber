// 与服务端 model.DefaultSettings 保持一致的默认分页值（仅作 fallback）
export const DEFAULT_PAGE_SIZE = 20;

export function getArticlePageSize(settings?: Record<string, string>): number {
	const n = Number.parseInt(settings?.article_page_size ?? '', 10);
	return n > 0 ? n : DEFAULT_PAGE_SIZE;
}

export function getContentPageSize(settings?: Record<string, string>): number {
	const n = Number.parseInt(settings?.page_default_size ?? '', 10);
	return n > 0 ? n : DEFAULT_PAGE_SIZE;
}
