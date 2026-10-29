package model

import "time"

// Group 用户组。系统组（IsSystem=true）与角色同名：admin/moderator/user/ban/pending_user。
type Group struct {
	ID          string    `gorm:"primaryKey;size:64" json:"id"`
	Name        string    `gorm:"uniqueIndex;size:64" json:"name"`
	DisplayName string    `gorm:"size:128" json:"display_name"`
	Visibility  string    `gorm:"size:16;default:public" json:"visibility"` // public|closed|secret
	IsSystem    bool      `json:"is_system"`
	CreatedAt   time.Time `json:"created_at"`
}

func (Group) TableName() string { return "groups" }

// Group visibility constants.
const (
	GroupVisibilityPublic = "public"
	GroupVisibilityClosed = "closed"
	GroupVisibilitySecret = "secret"
)

// GroupMember 组成员关系。复合主键 (GroupID, UserID)。
type GroupMember struct {
	GroupID  string    `gorm:"primaryKey;size:64" json:"group_id"`
	UserID   string    `gorm:"primaryKey;size:64;index" json:"user_id"`
	Role     string    `gorm:"size:16;default:member" json:"role"` // owner|moderator|member
	JoinedAt time.Time `json:"joined_at"`
}

func (GroupMember) TableName() string { return "group_members" }

// Group member role constants.
const (
	GroupMemberRoleOwner     = "owner"
	GroupMemberRoleModerator = "moderator"
	GroupMemberRoleMember    = "member"
)

// GroupPermission 组-权限映射，模式对齐 RolePermission。
type GroupPermission struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	GroupID      string `gorm:"index;size:64" json:"group_id"`
	PermissionID uint   `gorm:"index" json:"permission_id"`
}

func (GroupPermission) TableName() string { return "group_permissions" }

// DefaultSystemGroups 与 DefaultRoles 同名的自动系统组。
var DefaultSystemGroups = []Group{
	{Name: "admin", DisplayName: "管理员", Visibility: GroupVisibilityPublic, IsSystem: true},
	{Name: "moderator", DisplayName: "版主", Visibility: GroupVisibilityPublic, IsSystem: true},
	{Name: "user", DisplayName: "用户", Visibility: GroupVisibilityPublic, IsSystem: true},
	{Name: "pending_user", DisplayName: "待审核用户", Visibility: GroupVisibilityPublic, IsSystem: true},
	{Name: "ban", DisplayName: "封禁", Visibility: GroupVisibilityClosed, IsSystem: true},
}

// IsValidGroupVisibility 校验可见性取值。
func IsValidGroupVisibility(v string) bool {
	switch v {
	case GroupVisibilityPublic, GroupVisibilityClosed, GroupVisibilitySecret:
		return true
	default:
		return false
	}
}

// IsValidGroupMemberRole 校验成员角色取值。
func IsValidGroupMemberRole(r string) bool {
	switch r {
	case GroupMemberRoleOwner, GroupMemberRoleModerator, GroupMemberRoleMember:
		return true
	default:
		return false
	}
}
