package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
)

// GroupRepository 用户组、成员与组权限数据访问。
type GroupRepository interface {
	List(ctx context.Context) ([]model.Group, error)
	FindByID(ctx context.Context, id string) (*model.Group, error)
	FindByName(ctx context.Context, name string) (*model.Group, error)
	Create(ctx context.Context, group *model.Group) error
	CreateIfNotExistsByName(ctx context.Context, group *model.Group) error
	Update(ctx context.Context, group *model.Group) error
	Delete(ctx context.Context, id string) error

	ListMembers(ctx context.Context, groupID string, offset, limit int) ([]model.GroupMember, error)
	CountMembers(ctx context.Context, groupID string) (int64, error)
	AddMember(ctx context.Context, member *model.GroupMember) error
	RemoveMember(ctx context.Context, groupID, userID string) error
	IsMember(ctx context.Context, groupID, userID string) (bool, error)
	ListGroupIDsByUser(ctx context.Context, userID string) ([]string, error)
	RemoveUserFromSystemGroups(ctx context.Context, userID string) error

	GetGroupPermissions(ctx context.Context, groupID string) ([]string, error)
	GetAllGroupPermissions(ctx context.Context) (map[string][]string, error)
	SyncGroupPermissions(ctx context.Context, groupID string, permCodes []string) error
	DeleteGroupPermissions(ctx context.Context, groupID string) error
}

func NewGroupRepository(store db.Store) GroupRepository {
	return &sqlGroupRepo{db: store.SQL()}
}
