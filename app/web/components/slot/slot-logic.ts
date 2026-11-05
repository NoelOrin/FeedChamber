import type { SlotEntry } from '@feedchamber/api-client';

/**
 * 插槽的纯逻辑层。
 *
 * 抽出来是因为 `app/web` 没有测试栈（无 vitest / jsdom），而这些逻辑恰恰是
 * 唯一会出 bug 的地方：排序、可见性、未知插槽与未知 kind 的处理。
 * 组件只做 kind → JSX 的直接映射，靠人工验收。
 */

/** 合法插槽名。与后端 `model.AllSlots` 严格一致。 */
const SLOTS = new Set([
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
]);

/** 合法条目类型。未知 kind 被跳过，不影响同插槽其他条目。 */
const KINDS = new Set(['text', 'link', 'image', 'divider', 'widget']);

/** 已知插槽名？ */
export function isKnownSlot(name: string): boolean {
	return SLOTS.has(name);
}

/** 已知条目类型？ */
export function isKnownKind(kind: string): boolean {
	return KINDS.has(kind);
}

/** 解析后端以字符串存储的 props。解析失败返回空对象而不是抛错。 */
export function parseProps(raw: string | undefined): Record<string, unknown> {
	if (!raw) return {};
	try {
		const parsed: unknown = JSON.parse(raw);
		if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
			return parsed as Record<string, unknown>;
		}
		return {};
	} catch {
		// 单条坏数据不该让整个插槽消失。
		return {};
	}
}

/**
 * 取某插槽下要渲染的条目。
 *
 * 三条规则：
 * 1. 未知插槽名 → 空数组（组件据此返回 null，不渲染任何东西）；
 * 2. 未知 kind → 跳过该条，**不影响**同插槽其他条目；
 * 3. 按 sort 升序；sort 相同时按 id 稳定排序，避免每次渲染顺序抖动。
 */
export function entriesForSlot(entries: SlotEntry[] | undefined, name: string): SlotEntry[] {
	if (!entries || entries.length === 0) return [];
	if (!isKnownSlot(name)) return [];
	return entries
		.filter((e) => e.slot === name && isKnownKind(e.kind))
		.sort((a, b) => a.sort - b.sort || a.id.localeCompare(b.id));
}
