import { post } from '../client';

/** 壳层插槽名。与后端 `model.AllSlots` 严格一致；未知名字前端忽略。 */
export const SLOT_NAMES = [
	'sidebar.top',
	'sidebar.nav.after',
	'sidebar.spaces.before',
	'sidebar.spaces.after',
	'sidebar.tags.after',
	'sidebar.bottom',
	'header.left',
	'header.right.before',
	'header.right.after',
	'footer.left',
	'footer.right',
] as const;

export type SlotName = (typeof SLOT_NAMES)[number];

/** 条目类型。刻意不含 html：插槽是同源位置，允许任意 HTML 等于允许全站 XSS。 */
export const SLOT_KINDS = ['text', 'link', 'image', 'divider', 'widget'] as const;

export type SlotKind = (typeof SLOT_KINDS)[number];

export type SlotEntry = {
	id: string;
	slot: string;
	kind: string;
	/** 后端以 JSON 字符串存储；客户端解析后使用。 */
	props: string;
	sort: number;
	visibility: string;
	min_level: number;
	enabled: boolean;
	owner_plugin_id?: string;
	created_at: string;
	updated_at: string;
};

/** 写入插槽条目的请求体。enabled 省略表示「未指定」，后端按启用处理。 */
export type SlotEntryInput = {
	slot: string;
	kind: string;
	props: Record<string, unknown>;
	sort?: number;
	visibility?: string;
	min_level?: number;
	enabled?: boolean;
};

export const slotService = {
	list(slot?: string) {
		return post<SlotEntry[]>('/admin/slots/list', slot ? { slot } : {});
	},
	create(body: SlotEntryInput) {
		return post<SlotEntry>('/admin/slots/create', body);
	},
	update(id: string, body: SlotEntryInput) {
		return post<SlotEntry>(`/admin/slots/${id}/update`, body);
	},
	delete(id: string) {
		return post<null>(`/admin/slots/${id}/delete`, {});
	},
	/** 公开读取：当前访问者可见的条目，已按 slot + sort 排序。 */
	public() {
		return post<SlotEntry[]>('/slots/public', {});
	},
};
