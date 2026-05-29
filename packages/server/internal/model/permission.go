package model

import "time"

// Permission 权限定义表
type Permission struct {
	ID          uint      `gorm:"primaryKey" json:"id" bson:"_id"`
	Code        string    `gorm:"uniqueIndex;size:64" json:"code" bson:"code"`
	Name        string    `gorm:"size:64" json:"name" bson:"name"`
	Description string    `gorm:"size:255" json:"description" bson:"description"`
	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
}

func (Permission) TableName() string { return "permissions" }

// 论坛权限码
const (
	PermPostCreate = "post:create"
	PermPostRead   = "post:read"
	PermPostUpdate = "post:update"
	PermPostDelete = "post:delete"

	PermCommentCreate = "comment:create"
	PermCommentDelete = "comment:delete"

	PermCategoryCreate = "category:create"
	PermCategoryDelete = "category:delete"

	PermUserRead   = "user:read"
	PermUserUpdate = "user:update"
	PermUserDelete = "user:delete"

	PermPostPin  = "post:pin"
	PermPostLock = "post:lock"

	PermReportCreate = "report:create"
	PermReportManage = "report:manage"

	PermRoleRead   = "role:read"
	PermRoleManage = "role:manage"
)

// DefaultPermissions 种子权限列表
var DefaultPermissions = []Permission{
	{Code: PermPostCreate, Name: "创建帖子", Description: "创建新的论坛帖子"},
	{Code: PermPostRead, Name: "查看帖子", Description: "查看帖子列表和详情"},
	{Code: PermPostUpdate, Name: "编辑帖子", Description: "修改帖子标题和内容"},
	{Code: PermPostDelete, Name: "删除帖子", Description: "删除帖子"},

	{Code: PermCommentCreate, Name: "创建评论", Description: "在帖子下发表评论"},
	{Code: PermCommentDelete, Name: "删除评论", Description: "删除评论"},

	{Code: PermCategoryCreate, Name: "创建分类", Description: "创建新的论坛分类"},
	{Code: PermCategoryDelete, Name: "删除分类", Description: "删除论坛分类"},

	{Code: PermUserRead, Name: "查看用户", Description: "查看用户列表和详情"},
	{Code: PermUserUpdate, Name: "编辑用户", Description: "修改用户信息"},
	{Code: PermUserDelete, Name: "删除用户", Description: "删除用户账号"},

	{Code: PermRoleRead, Name: "查看角色", Description: "查看角色列表"},
	{Code: PermRoleManage, Name: "管理角色", Description: "创建、删除角色和分配权限"},

	{Code: PermPostPin, Name: "置顶帖子", Description: "置顶或取消置顶帖子"},
	{Code: PermPostLock, Name: "锁定帖子", Description: "锁定或取消锁定帖子"},

	{Code: PermReportCreate, Name: "举报内容", Description: "举报帖子或评论"},
	{Code: PermReportManage, Name: "管理举报", Description: "查看、处理和删除举报"},
}

// RolePermission 角色-权限关联表
type RolePermission struct {
	ID           uint   `gorm:"primaryKey" json:"id" bson:"_id"`
	RoleName     string `gorm:"index;size:32" json:"role_name" bson:"role_name"`
	PermissionID uint   `gorm:"index" json:"permission_id" bson:"permission_id"`
}

func (RolePermission) TableName() string { return "role_permissions" }

// DefaultRolePermissions 默认角色权限映射
var DefaultRolePermissions = map[string][]string{
	"admin": {
		PermPostCreate, PermPostRead, PermPostUpdate, PermPostDelete,
		PermPostPin, PermPostLock,
		PermCommentCreate, PermCommentDelete,
		PermCategoryCreate, PermCategoryDelete,
		PermUserRead, PermUserUpdate, PermUserDelete,
		PermRoleRead, PermRoleManage,
		PermReportCreate, PermReportManage,
	},
	"user": {
		PermPostCreate, PermPostRead,
		PermCommentCreate,
		PermUserRead,
		PermRoleRead,
		PermReportCreate,
	},
	"ban": {},
}
