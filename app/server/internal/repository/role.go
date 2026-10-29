package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type RoleRepository interface {
	List(ctx context.Context) ([]model.Role, error)
	GetByName(ctx context.Context, name string) (*model.Role, error)
	Create(ctx context.Context, role *model.Role) error
	CreateIfNotExists(ctx context.Context, role *model.Role) error
	Delete(ctx context.Context, id string) error
	HasUsersWithRole(ctx context.Context, roleName string) (bool, error)
	DeleteRolePermissions(ctx context.Context, roleName string) error
	Update(ctx context.Context, id string, newName string, displayName string, description string) error
}

func NewRoleRepository(store db.Store) RoleRepository {
	return &sqlRoleRepo{db: store.SQL()}
}
