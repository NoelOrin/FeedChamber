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

/**
 * searchParams 里的 `?page=` → 合法页号（缺失、非数字、<=0 一律回落 1）。
 *
 * RSC 页面与客户端组件都要这个归一，此前只有 components/user/queries.ts 里有一份
 * 本地实现；放到这里是为了让新的私有列表页（如 /user/history）不必再复制一遍。
 */
export function toPage(value: string | string[] | undefined): number {
	const raw = Array.isArray(value) ? value[0] : value;
	const n = Number.parseInt(raw ?? '', 10);
	return n > 0 ? n : 1;
}
