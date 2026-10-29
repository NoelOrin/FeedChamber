// Solid 版 `~/utils/draft.ts` 平移：localStorage 键语义（前缀 feedchamber:draft:）与旧版完全一致，
// 用户本地已有的草稿可继续被读出。
const PREFIX = 'feedchamber:draft:';

export function saveDraft(key: string, value: unknown) {
	if (typeof localStorage === 'undefined') return;
	localStorage.setItem(PREFIX + key, JSON.stringify({ v: value, t: Date.now() }));
}

export function loadDraft<T>(key: string): T | null {
	if (typeof localStorage === 'undefined') return null;
	const raw = localStorage.getItem(PREFIX + key);
	if (!raw) return null;
	try {
		return JSON.parse(raw).v as T;
	} catch {
		return null;
	}
}

export function clearDraft(key: string) {
	if (typeof localStorage === 'undefined') return;
	localStorage.removeItem(PREFIX + key);
}

/**
 * 旧版发帖页把草稿存成 { title, content } 对象；编辑器现在只认 markdown 字符串。
 * 两种形态都兼容，保证迁移窗口内旧草稿不丢正文。
 */
export function draftToMarkdown(draft: unknown): string | null {
	if (typeof draft === 'string') return draft || null;
	if (draft && typeof draft === 'object' && 'content' in draft) {
		const content = (draft as { content?: unknown }).content;
		if (typeof content === 'string' && content) return content;
	}
	return null;
}
