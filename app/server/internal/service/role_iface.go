package service

import (
	"context"

	"orinforum/internal/model"
)

type RoleService interface {
	List(ctx context.Context) ([]model.Role, error)
	Create(ctx context.Context, name string, displayName string) (*model.Role, error)
	Delete(ctx context.Context, id string) error
	Update(ctx context.Context, id string, name string, displayName string, description string) (*model.Role, error)
}
