import type { Permission } from '@orinforum/api-client';

// 后端 Permission 不返回 group 字段，这里按权限码前缀（冒号前的部分）推导分组标签。
const GROUP_LABELS: Record<string, string> = {
	post: '帖子',
	comment: '评论',
	tag: '标签',
	space: '版块',
	user: '用户',
	role: '角色',
	report: '举报',
	announcement: '公告',
	setting: '系统设置',
	stats: '统计',
	banned: '禁言',
	batch: '批量操作',
	admin: '后台',
	reconcile: '维护',
};

export function permissionGroup(code: string): string {
	const prefix = code.split(':')[0];
	return GROUP_LABELS[prefix] || '其他';
}

// 按推导出的分组对权限进行归类，返回稳定顺序的分组数组。
export function groupPermissions(permissions: Permission[]): Array<[string, Permission[]]> {
	const map = new Map<string, Permission[]>();
	for (const p of permissions) {
		const group = p.group || permissionGroup(p.code);
		const list = map.get(group);
		if (list) list.push(p);
		else map.set(group, [p]);
	}
	return Array.from(map.entries());
}

/* 权限码 → 语义色，供主站 Badge 使用。
   与下面的 permissionColor（antd 颜色名）保持同一套判据，避免两处规则漂移。 */
export type PermissionTone = 'destructive' | 'primary' | 'warning' | 'success';

export function permissionTone(code: string): PermissionTone {
	if (code.includes('delete')) return 'destructive';
	if (code.includes('write') || code.includes('create')) return 'primary';
	if (code.includes('manage') || code.includes('update')) return 'warning';
	return 'success';
}

export function permissionColor(code: string): string {
	if (code.includes('delete')) return 'red';
	if (code.includes('write') || code.includes('create')) return 'blue';
	if (code.includes('manage') || code.includes('update')) return 'orange';
	return 'green';
}

// 后端真正按版块校验（HasPermissionInSpace / RequireSpacePermission）的权限码。
// 只有这些权限设置版块级覆盖才有效；其余权限是全局判定，设了也不生效。
export const SPACE_SCOPED_PERMISSION_CODES = [
	'post:update',
	'post:delete',
	'post:pin',
	'post:lock',
	'comment:update',
	'comment:delete',
	'user:update',
	'space:update',
];

/**
 * 版面级权限值的编码格式: `<spaceId>__space__<permCode>`
 * 全局权限值就是原始 permCode 本身。
 * 用 `__space__` 作分隔符，权限码和 spaceId 都不含此串，不会冲突。
 */
const SPACE_VALUE_SEP = '__space__';

/** 将版面级权限编码为 Select value */
export function encodeSpacePermValue(spaceId: string, permCode: string): string {
	return `${spaceId}${SPACE_VALUE_SEP}${permCode}`;
}

/** 解码 Select value，返回 { spaceId, code }；全局权限 spaceId 为空 */
export function decodePermValue(value: string): { spaceId: string; code: string } {
	const idx = value.indexOf(SPACE_VALUE_SEP);
	if (idx === -1) return { spaceId: '', code: value };
	return { spaceId: value.slice(0, idx), code: value.slice(idx + SPACE_VALUE_SEP.length) };
}

/** 判断是否为版面级权限值 */
export function isSpacePermValue(value: string): boolean {
	return value.includes(SPACE_VALUE_SEP);
}
