export const ErrorCode = {
	SUCCESS: 0,
	TOKEN_NOT_EXIST: 1001,
	TOKEN_WRONG: 1002,
	TOKEN_EXPIRED: 1003,
	TOKEN_REVOKED: 1014,
	INVALID_PASSWORD: 1010,
	USER_NOT_FOUND: 1011,
	USERNAME_EXISTS: 1012,
	EMAIL_EXISTS: 1016,
	FORBIDDEN: 1013,
	USER_BANNED: 1015,
	INVALID_PARAMS: 2001,
	NOT_FOUND: 3001,
	ALREADY_EXISTS: 3002,
	RATE_LIMITED: 4001,
	BODY_TOO_LARGE: 4002,
	INTERNAL_ERROR: 5001,
	REQUEST_TIMEOUT: 5004,
	MAINTENANCE: 5005,
} as const;

export type ErrorCode = (typeof ErrorCode)[keyof typeof ErrorCode];

export const PermissionCode = {
	POST_CREATE: 'post:create',
	POST_READ: 'post:read',
	POST_UPDATE: 'post:update',
	POST_DELETE: 'post:delete',
	POST_PIN: 'post:pin',
	POST_LOCK: 'post:lock',
	COMMENT_CREATE: 'comment:create',
	COMMENT_DELETE: 'comment:delete',
	COMMENT_UPDATE: 'comment:update',
	TAG_CREATE: 'tag:create',
	TAG_UPDATE: 'tag:update',
	TAG_DELETE: 'tag:delete',
	SPACE_CREATE: 'space:create',
	SPACE_UPDATE: 'space:update',
	SPACE_DELETE: 'space:delete',
	USER_READ: 'user:read',
	USER_UPDATE: 'user:update',
	USER_DELETE: 'user:delete',
	USER_ACTIVITY_READ: 'user:activity:read',
	ROLE_READ: 'role:read',
	ROLE_MANAGE: 'role:manage',
	REPORT_CREATE: 'report:create',
	REPORT_MANAGE: 'report:manage',
	MOTION_CREATE: 'motion:create',
	MOTION_MANAGE: 'motion:manage',
	ANNOUNCEMENT_MANAGE: 'announcement:manage',
	SETTING_MANAGE: 'setting:manage',
	STATS_READ: 'stats:read',
	BANNED_MANAGE: 'banned:manage',
	BATCH_EDIT: 'batch:edit',
	ADMIN_SEARCH: 'admin:search',
	RECONCILE_COUNTERS: 'reconcile:counters',
	LEVEL_MANAGE: 'level:manage',
	GATE_MANAGE: 'gate:manage',
	// 站点自定义层：独立权限域，MUST NOT 并入任何既有 scope（见 mcp-server 规范）。
	CUSTOMIZE_READ: 'customize:read',
	CUSTOMIZE_WRITE: 'customize:write',
	CUSTOMIZE_PUBLISH: 'customize:publish',
} as const;

export type PermissionCode = (typeof PermissionCode)[keyof typeof PermissionCode];

export const SettingKey = {
	POST_AUTH_REQUIRED: 'post_auth_required',
	COMMENT_AUTH_REQUIRED: 'comment_auth_required',
	SPACE_VIEW_AUTH_REQUIRED: 'space_view_auth_required',
	TAG_LIST_AUTH_REQUIRED: 'tag_list_auth_required',
	ANNOUNCEMENT_AUTH_REQUIRED: 'announcement_auth_required',
	EMAIL_ENABLED: 'email_enabled',
	REGISTER_MODE: 'register_mode',
	SITE_MAINTENANCE: 'site_maintenance',
	EMAIL_VERIFICATION_REQUIRED: 'email_verification_required',
	API_KEY_ENABLED: 'api_key_enabled',
	MCP_ENABLED: 'mcp_enabled',
	MCP_DISABLED_TOOLS: 'mcp_disabled_tools',
	MCP_MAX_RESULT_BYTES: 'mcp_max_result_bytes',
	MFA_TOTP_ENABLED: 'mfa_totp_enabled',
	// 文章审核模式：off / async / external / manual（默认 async）
	MODERATION_MODE: 'moderation_mode',
} as const;

export type SettingKey = (typeof SettingKey)[keyof typeof SettingKey];

/** 文章审核模式取值，与后端 model.ModerationMode* 常量一一对应。 */
export const ModerationMode = {
	OFF: 'off',
	ASYNC: 'async',
	EXTERNAL: 'external',
	MANUAL: 'manual',
} as const;

export type ModerationMode = (typeof ModerationMode)[keyof typeof ModerationMode];

export const RoleName = {
	ADMIN: 'admin',
	USER: 'user',
	BAN: 'ban',
	PENDING_USER: 'pending_user',
} as const;

export type RoleName = (typeof RoleName)[keyof typeof RoleName];

export const PostStatus = {
	PENDING: 'pending',
	APPROVED: 'approved',
	REJECTED: 'rejected',
	CLOSED: 'closed',
} as const;

export type PostStatus = (typeof PostStatus)[keyof typeof PostStatus];

/**
 * 列表状态筛选的「不过滤」哨兵。
 *
 * 后端对**持有 report:manage 的调用者**把该值识别为「不按状态过滤」（无权限者只看得到
 * approved/closed）。后台帖子列表默认就发它，因此它是待审帖的唯一入口；举报列表的
 * status 筛选共用同一个哨兵。
 */
export const STATUS_FILTER_ALL = '__all__' as const;

export const API_BASE_URL = '/api/v1' as const;
