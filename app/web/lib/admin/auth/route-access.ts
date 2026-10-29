'use client';

/**
 * 页面级权限门：后台每个模块「看它需要哪个权限码」。
 *
 * 权限码与后端网关 `app/server/internal/gateway/permission.go` 的 `/api/v1/**` 映射对齐，
 * 取的是该页面主数据的读接口所需的权限（例如设置页要 `setting:manage`、审核队列要
 * `report:manage`）。数组语义是 **anyOf**：命中任一即可。
 *
 * 注意：这里只是 UX 门控——隐藏菜单、拦下必然报 403 的页面。真正的安全边界仍在后端。
 */

export interface RoutePermission {
	/** 路径前缀；`/` 只匹配根路径本身。 */
	path: string;
	/** 命中任一权限码即可访问；空数组表示只要求已进后台。 */
	anyOf: string[];
}

/** 顺序即优先级：详情页必须排在列表页之前，`/admin` 只做精确匹配。 */
export const ROUTE_PERMISSIONS: RoutePermission[] = [
	{ path: '/admin/settings/dashboard', anyOf: ['setting:manage'] },
	{ path: '/admin/settings', anyOf: ['setting:manage'] },
	{ path: '/admin/comments/detail', anyOf: ['comment:delete'] },
	{ path: '/admin/comments', anyOf: ['comment:delete'] },
	{ path: '/admin/posts/detail', anyOf: ['post:read'] },
	{ path: '/admin/posts', anyOf: ['post:read'] },
	{ path: '/admin/reports/detail', anyOf: ['report:manage'] },
	{ path: '/admin/reports', anyOf: ['report:manage'] },
	{ path: '/admin/roles/detail', anyOf: ['role:manage'] },
	{ path: '/admin/roles', anyOf: ['role:read'] },
	{ path: '/admin/users/detail', anyOf: ['user:read'] },
	{ path: '/admin/users', anyOf: ['user:read'] },
	{ path: '/admin/audit-logs', anyOf: ['stats:read'] },
	{ path: '/admin/announcements', anyOf: ['announcement:manage'] },
	{ path: '/admin/api-keys', anyOf: ['api-key:self'] },
	{ path: '/admin/backups', anyOf: ['backup:manage'] },
	{ path: '/admin/banned', anyOf: ['banned:manage'] },
	{ path: '/admin/groups', anyOf: ['role:read'] },
	{ path: '/admin/levels', anyOf: ['level:manage'] },
	{ path: '/admin/xp-rules', anyOf: ['level:manage'] },
	{ path: '/admin/gates', anyOf: ['gate:manage'] },
	// 站点自定义层：读需 customize:read，写操作由后端 customize:write 门控。
	{ path: '/admin/customize/snippets', anyOf: ['customize:read'] },
	{ path: '/admin/customize/theme', anyOf: ['customize:read'] },
	{ path: '/admin/customize/nav', anyOf: ['customize:read'] },
	{ path: '/admin/customize/home', anyOf: ['customize:read'] },
	{ path: '/admin/customize/pages', anyOf: ['customize:read'] },
	// 本站 MCP 工具管理与外部 MCP 服务登记都挂在设置管理权限下。
	{ path: '/admin/mcp', anyOf: ['setting:manage'] },
	{ path: '/admin/mcp-servers', anyOf: ['setting:manage'] },
	{ path: '/admin/invites', anyOf: ['user:update'] },
	{ path: '/admin/moderation', anyOf: ['report:manage'] },
	{ path: '/admin/oauth-clients', anyOf: ['setting:manage'] },
	{ path: '/admin/plugins', anyOf: ['plugin:manage'] },
	{ path: '/admin/search', anyOf: ['admin:search'] },
	{ path: '/admin/security', anyOf: ['security:read'] },
	{ path: '/admin/sessions', anyOf: ['session:self'] },
	{ path: '/admin/spaces', anyOf: ['space:update'] },
	{ path: '/admin/tags', anyOf: ['tag:update'] },
	{ path: '/admin/trash', anyOf: ['post:delete', 'comment:delete'] },
	{ path: '/admin/webhooks', anyOf: ['role:manage'] },
	// 运营概览读 /admin/dashboard，需要 stats:read
	{ path: '/admin', anyOf: ['stats:read'] },
];

/** 解析某个路径需要的权限码；未登记的路径不设门控。 */
export function requiredPermissionsFor(pathname: string): string[] {
	const normalized = pathname.replace(/\/+$/, '') || '/';
	const match = ROUTE_PERMISSIONS.find((entry) => {
		// 根与 `/admin` 本身只做精确匹配，否则 `/admin/<未登记模块>` 会被误判成首页权限。
		if (entry.path === '/' || entry.path === '/admin') return normalized === entry.path;
		return normalized === entry.path || normalized.startsWith(`${entry.path}/`);
	});
	return match?.anyOf ?? [];
}
