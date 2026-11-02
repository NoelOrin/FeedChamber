package service

import (
	"context"

	"feedchamber/internal/model"
)

type UserService interface {
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	List(ctx context.Context, page, pageSize int) ([]model.User, error)
	Search(ctx context.Context, keyword, role string, page, pageSize int) ([]model.User, error)
	UpdateProfile(ctx context.Context, id string, username, displayName, avatar, bio, coverImage, profileLayout *string) error
	UpdateRole(ctx context.Context, id, role string) error
	Delete(ctx context.Context, id string) error
	GetPublicProfile(ctx context.Context, id string) (*model.PublicProfileResponse, error)
	GetActivity(ctx context.Context, userID string, year int) (*model.ActivityResponse, error)
}
