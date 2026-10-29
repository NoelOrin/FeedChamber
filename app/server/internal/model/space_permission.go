package model

type SpaceRolePermission struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	SpaceID      string `gorm:"index:idx_space_role,priority:1;size:64" json:"space_id"`
	RoleName     string `gorm:"index:idx_space_role,priority:2;size:32" json:"role_name"`
	PermissionID uint   `gorm:"index" json:"permission_id"`
}

func (SpaceRolePermission) TableName() string { return "space_role_permissions" }
