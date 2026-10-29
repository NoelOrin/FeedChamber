package service

import (
	"context"

	"orinforum/internal/model"
)

// GroupService 用户组管理接口。
type GroupService interface {
	List(ctx context.Context) ([]model.Group, error)
	Create(ctx context.Context, name, displayName, visibility string) (*model.Group, error)
	Update(ctx context.Context, id, displayName, visibility string) (*model.Group, error)
	Delete(ctx context.Context, id string) error

	ListMembers(ctx context.Context, groupID string, page, pageSize int) ([]model.GroupMember, int, error)
	AddMember(ctx context.Context, groupID, userID, role string) error
	RemoveMember(ctx context.Context, groupID, userID string) error

	// SyncUserSystemGroup 将用户同步到与 roleName 同名的系统组（best effort 调用方）。
	SyncUserSystemGroup(ctx context.Context, userID, roleName string) error
	// SyncGroupPermissions 设置组权限码。
	SyncGroupPermissions(ctx context.Context, groupID string, permCodes []string) error
	GetGroupPermissions(ctx context.Context, groupID string) ([]string, error)
}
