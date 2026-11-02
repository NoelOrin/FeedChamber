package model

import "time"

// Permission 权限定义表
type Permission struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Code        string    `gorm:"uniqueIndex;size:64" json:"code"`
	Name        string    `gorm:"size:64" json:"name"`
	Description string    `gorm:"size:255" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

func (Permission) TableName() string { return "permissions" }

// 论坛权限码
const (
	PermPostCreate = "post:create"
	PermPostRead   = "post:read"
	PermPostUpdate = "post:update"
	PermPostDelete = "post:delete"

	PermCommentCreate = "comment:create"
	PermCommentRead   = "comment:read"
	PermCommentDelete = "comment:delete"
	PermCommentUpdate = "comment:update"

	PermTagCreate = "tag:create"
	PermTagUpdate = "tag:update"
	PermTagDelete = "tag:delete"

	PermSpaceCreate = "space:create"
	PermSpaceUpdate = "space:update"
	PermSpaceDelete = "space:delete"

	PermUserRead         = "user:read"
	PermUserUpdate       = "user:update"
	PermUserDelete       = "user:delete"
	PermUserActivityRead = "user:activity:read"

	PermPostPin  = "post:pin"
	PermPostLock = "post:lock"

	PermReportCreate = "report:create"
	PermReportManage = "report:manage"

	PermMotionCreate = "motion:create"
	PermMotionManage = "motion:manage"

	PermRoleRead   = "role:read"
	PermRoleManage = "role:manage"

	PermAnnouncementManage = "announcement:manage"
	PermSettingManage      = "setting:manage"
	PermStatsRead          = "stats:read"
	PermBannedManage       = "banned:manage"
	PermBatchEdit          = "batch:edit"
	PermAdminSearch        = "admin:search"
	PermAdminMetrics       = "admin:metrics"
	PermReconcileCounters  = "reconcile:counters"

	// Self-service permissions (any authenticated user)
	PermAuthSelf         = "auth:self"
	PermOAuthSelf        = "oauth:self"
	PermSSOSelf          = "sso:self"
	PermUserSelf         = "user:self"
	PermFavoriteSelf     = "favorite:self"
	PermContentLike      = "content:like"
	PermFollowSelf       = "follow:self"
	PermFollowRead       = "follow:read"
	PermUploadSelf       = "upload:self"
	PermNotificationSelf = "notification:self"
	PermPMSelf           = "pm:self"
	PermCheckinSelf      = "checkin:self"
	PermAPIKeySelf       = "api-key:self"
	PermSessionSelf      = "session:self"
	PermSecurityRead     = "security:read"
	PermSecurityReport   = "security:report"
	PermDashboardRead    = "dashboard:read"
	PermDraftSelf        = "draft:self"
	PermRevisionSelf     = "revision:self"
	PermRealtimeSelf     = "realtime:self"
	PermBackupManage     = "backup:manage"
	PermThemeSelf        = "theme:self"
	PermPluginManage     = "plugin:manage"
	PermLevelManage      = "level:manage"
	PermGateManage       = "gate:manage"

	// 站点自定义层。这三个权限码构成独立权限域，MUST NOT 被并入
	// gateway/apiKeyScopeRegistry 的 read / write / admin 任一项：
	// 一旦并入，所有存量持 admin scope 的 API Key 会在部署瞬间获得
	// 向全站注入 JS 的能力，属于权限提升。新 scope 必须在建 Key 时显式勾选。
	PermCustomizeRead    = "customize:read"
	PermCustomizeWrite   = "customize:write"
	PermCustomizePublish = "customize:publish"
)

// DefaultPermissions 种子权限列表
var DefaultPermissions = []Permission{
	{Code: PermPostCreate, Name: "创建帖子", Description: "创建新的论坛帖子"},
	{Code: PermPostRead, Name: "查看帖子", Description: "查看帖子列表和详情"},
	{Code: PermPostUpdate, Name: "编辑帖子", Description: "修改帖子标题和内容"},
	{Code: PermPostDelete, Name: "删除帖子", Description: "删除帖子"},

	{Code: PermCommentCreate, Name: "创建评论", Description: "在帖子下发表评论"},
	{Code: PermCommentRead, Name: "查看评论", Description: "查看评论和回复列表"},
	{Code: PermCommentDelete, Name: "删除评论", Description: "删除评论"},
	{Code: PermCommentUpdate, Name: "编辑评论", Description: "修改评论内容"},

	{Code: PermTagCreate, Name: "创建标签", Description: "创建新的标签"},
	{Code: PermTagUpdate, Name: "编辑标签", Description: "修改标签信息"},
	{Code: PermTagDelete, Name: "删除标签", Description: "删除标签"},

	{Code: PermSpaceCreate, Name: "创建版块", Description: "创建新的版块"},
	{Code: PermSpaceUpdate, Name: "编辑版块", Description: "修改版块信息和状态"},
	{Code: PermSpaceDelete, Name: "删除版块", Description: "删除版块"},

	{Code: PermUserRead, Name: "查看用户", Description: "查看用户列表和详情"},
	{Code: PermUserUpdate, Name: "编辑用户", Description: "修改用户信息"},
	{Code: PermUserDelete, Name: "删除用户", Description: "删除用户账号"},
	{Code: PermUserActivityRead, Name: "查看用户活动", Description: "查看用户操作历史"},

	{Code: PermRoleRead, Name: "查看角色", Description: "查看角色列表"},
	{Code: PermRoleManage, Name: "管理角色", Description: "创建、删除角色和分配权限"},

	{Code: PermPostPin, Name: "置顶帖子", Description: "置顶或取消置顶帖子"},
	{Code: PermPostLock, Name: "锁定帖子", Description: "锁定或取消锁定帖子"},

	{Code: PermReportCreate, Name: "举报内容", Description: "举报帖子或评论"},
	{Code: PermReportManage, Name: "管理举报", Description: "查看、处理和删除举报"},

	{Code: PermMotionCreate, Name: "发起自治动议", Description: "发起社区自治动议并参与信誉加权表决"},
	{Code: PermMotionManage, Name: "管理自治动议", Description: "翻案、标记滥诉、将软隔离升级为硬封禁/封号"},

	{Code: PermAnnouncementManage, Name: "管理公告", Description: "创建、编辑、发布和删除公告"},
	{Code: PermSettingManage, Name: "管理系统设置", Description: "查看和修改系统配置"},
	{Code: PermStatsRead, Name: "查看统计", Description: "查看后台统计数据和仪表盘"},
	{Code: PermBannedManage, Name: "管理禁言", Description: "查看禁言列表"},
	{Code: PermBatchEdit, Name: "批量操作", Description: "批量处理用户、帖子、评论"},
	{Code: PermAdminSearch, Name: "后台搜索", Description: "使用后台全局搜索"},
	{Code: PermReconcileCounters, Name: "修复计数器", Description: "重新计算和修复统计计数"},

	// Self-service permissions
	{Code: PermAuthSelf, Name: "基本认证操作", Description: "登出、刷新令牌、修改密码"},
	{Code: PermOAuthSelf, Name: "OAuth账号管理", Description: "关联和解绑OAuth账号"},
	{Code: PermSSOSelf, Name: "SSO授权", Description: "通过SSO授权登录"},
	{Code: PermUserSelf, Name: "个人资料管理", Description: "查看和修改个人资料"},
	{Code: PermFavoriteSelf, Name: "收藏管理", Description: "收藏和查看收藏"},
	{Code: PermContentLike, Name: "点赞", Description: "对内容点赞"},
	{Code: PermFollowSelf, Name: "关注操作", Description: "关注和取消关注用户"},
	{Code: PermFollowRead, Name: "查看关注", Description: "查看关注列表和状态"},
	{Code: PermUploadSelf, Name: "文件上传", Description: "上传和管理文件"},
	{Code: PermNotificationSelf, Name: "通知管理", Description: "查看和操作通知"},
	{Code: PermPMSelf, Name: "私信", Description: "查看和发送私信"},
	{Code: PermCheckinSelf, Name: "签到", Description: "签到和查看签到记录"},
	{Code: PermAPIKeySelf, Name: "API密钥管理", Description: "创建和管理API密钥"},
	{Code: PermSessionSelf, Name: "会话管理", Description: "查看和管理会话"},
	{Code: PermSecurityRead, Name: "查看安全信息", Description: "查看安全事件和登录历史"},
	{Code: PermSecurityReport, Name: "报告安全问题", Description: "报告可疑活动"},
	{Code: PermDashboardRead, Name: "查看仪表盘", Description: "查看用户首页仪表盘"},
	{Code: PermDraftSelf, Name: "草稿管理", Description: "保存和删除个人草稿"},
	{Code: PermRevisionSelf, Name: "版本管理", Description: "查看和恢复内容版本"},
	{Code: PermRealtimeSelf, Name: "实时通道", Description: "订阅个人实时事件"},
	{Code: PermBackupManage, Name: "备份管理", Description: "创建、下载、恢复和删除系统备份"},
	{Code: PermThemeSelf, Name: "主题偏好", Description: "查看和保存个人主题偏好"},
	{Code: PermPluginManage, Name: "插件管理", Description: "注册、启停和删除声明式插件"},
	{Code: PermLevelManage, Name: "等级体系管理", Description: "配置 XP 规则、等级定义与人工调整 XP"},
	{Code: PermGateManage, Name: "门控规则管理", Description: "配置门控规则、能力域默认动作与试算"},

	// 站点自定义层。仅 admin 角色默认持有；customize:publish 单独授予，
	// 因为它决定 AI/机器凭证能否让 JS 类改动直接生效（见 site-customization 规范）。
	{Code: PermCustomizeRead, Name: "查看站点自定义", Description: "查看片段、主题 token、导航、首页模块与自定义页"},
	{Code: PermCustomizeWrite, Name: "编辑站点自定义", Description: "创建和修改片段、主题 token、导航、首页模块与自定义页"},
	{Code: PermCustomizePublish, Name: "发布站点自定义", Description: "让机器凭证写入的 JS/HTML 类自定义内容直接生效，跳过草稿"},
}

// RolePermission 角色-权限关联表
type RolePermission struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	RoleName     string `gorm:"index;size:32" json:"role_name"`
	PermissionID uint   `gorm:"index" json:"permission_id"`
}

func (RolePermission) TableName() string { return "role_permissions" }

// DefaultRolePermissions 默认角色权限映射
var DefaultRolePermissions = map[string][]string{
	"admin": {
		PermPostCreate, PermPostRead, PermPostUpdate, PermPostDelete,
		PermPostPin, PermPostLock,
		PermCommentCreate, PermCommentRead, PermCommentDelete, PermCommentUpdate,
		PermTagCreate, PermTagUpdate, PermTagDelete,
		PermSpaceCreate, PermSpaceUpdate, PermSpaceDelete,
		PermUserRead, PermUserUpdate, PermUserDelete, PermUserActivityRead,
		PermRoleRead, PermRoleManage,
		PermReportCreate, PermReportManage,
		PermMotionCreate, PermMotionManage,
		PermAnnouncementManage,
		PermSettingManage,
		PermStatsRead,
		PermBannedManage,
		PermBatchEdit,
		PermAdminSearch,
		PermAdminMetrics,
		PermReconcileCounters,
		PermAuthSelf, PermOAuthSelf, PermSSOSelf, PermUserSelf,
		PermFavoriteSelf, PermContentLike,
		PermFollowSelf, PermFollowRead,
		PermUploadSelf, PermNotificationSelf, PermPMSelf,
		PermCheckinSelf, PermAPIKeySelf, PermSessionSelf,
		PermSecurityRead, PermSecurityReport,
		PermDashboardRead,
		PermDraftSelf, PermRevisionSelf,
		PermRealtimeSelf,
		PermBackupManage,
		PermThemeSelf,
		PermPluginManage,
		PermLevelManage,
		PermGateManage,
		// 站点自定义层只给 admin：内容本身可注入任意 JS，等于站点级 XSS 权限。
		PermCustomizeRead, PermCustomizeWrite, PermCustomizePublish,
	},
	"moderator": {
		PermPostCreate, PermPostRead, PermPostUpdate, PermPostDelete,
		PermPostPin, PermPostLock,
		PermCommentCreate, PermCommentDelete,
		PermTagCreate, PermTagUpdate, PermTagDelete,
		PermSpaceCreate, PermSpaceUpdate,
		PermUserRead,
		PermRoleRead,
		PermReportCreate, PermReportManage,
		PermMotionCreate, PermMotionManage,
		PermAnnouncementManage,
		PermStatsRead,
		PermBannedManage,
		PermBatchEdit,
		PermAdminSearch,
		PermUserActivityRead,
		PermAuthSelf, PermOAuthSelf, PermSSOSelf, PermUserSelf,
		PermFavoriteSelf, PermContentLike,
		PermFollowSelf, PermFollowRead,
		PermUploadSelf, PermNotificationSelf, PermPMSelf,
		PermCheckinSelf, PermAPIKeySelf, PermSessionSelf,
		PermSecurityRead, PermSecurityReport,
		PermDashboardRead,
		PermDraftSelf, PermRevisionSelf,
		PermRealtimeSelf,
		PermThemeSelf,
	},
	"user": {
		PermPostCreate, PermPostRead,
		PermCommentCreate,
		PermUserRead,
		PermRoleRead,
		PermReportCreate,
		PermMotionCreate,
		PermAuthSelf, PermOAuthSelf, PermSSOSelf, PermUserSelf,
		PermFavoriteSelf, PermContentLike,
		PermFollowSelf, PermFollowRead,
		PermUploadSelf, PermNotificationSelf, PermPMSelf,
		PermCheckinSelf, PermAPIKeySelf, PermSessionSelf,
		PermSecurityRead, PermSecurityReport,
		PermDashboardRead,
		PermDraftSelf, PermRevisionSelf,
		PermRealtimeSelf,
		PermThemeSelf,
	},
	"ban": {},
	// pending_user：可登录与基础自助，但不可发帖/评论
	"pending_user": {
		PermPostRead,
		PermCommentRead,
		PermUserRead,
		PermAuthSelf, PermOAuthSelf, PermSSOSelf, PermUserSelf,
		PermFavoriteSelf, PermContentLike,
		PermFollowSelf, PermFollowRead,
		PermUploadSelf, PermNotificationSelf, PermPMSelf,
		PermCheckinSelf, PermAPIKeySelf, PermSessionSelf,
		PermSecurityRead, PermSecurityReport,
		PermDashboardRead,
		PermThemeSelf,
	},
}
