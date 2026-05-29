package service

import (
	"context"
	"errors"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"

	"gorm.io/gorm"
)

type UserService interface {
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByUUID(ctx context.Context, uuid string) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	List(ctx context.Context, page, pageSize int) ([]model.User, error)
	UpdateProfile(ctx context.Context, uuid string, username, avatar, bio *string) error
	UpdateRole(ctx context.Context, id, role string) error
	Delete(ctx context.Context, id string) error
}

type userService struct {
	userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

func (s *userService) GetByID(ctx context.Context, id string) (*model.User, error) {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "user not found")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	user.Password = ""
	return user, nil
}

func (s *userService) GetByUUID(ctx context.Context, uuid string) (*model.User, error) {
	user, err := s.userRepo.FindByUUID(ctx, uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "user not found")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	user.Password = ""
	return user, nil
}

func (s *userService) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.USER_NOT_FOUND)
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	user.Password = ""
	return user, nil
}

func (s *userService) List(ctx context.Context, page, pageSize int) ([]model.User, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	users, err := s.userRepo.List(ctx, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	for i := range users {
		users[i].Password = ""
	}
	return users, nil
}

func (s *userService) UpdateProfile(ctx context.Context, uuid string, username, avatar, bio *string) error {
	user, err := s.userRepo.FindByUUID(ctx, uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "user not found")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if username != nil && *username != user.Username {
		existing, _ := s.userRepo.FindByUsername(ctx, *username)
		if existing != nil {
			return pkg.NewAppError(pkg.USERNAME_EXISTS)
		}
		user.Username = *username
	}
	if avatar != nil {
		user.Avatar = *avatar
	}
	if bio != nil {
		user.Bio = *bio
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *userService) UpdateRole(ctx context.Context, id, role string) error {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "user not found")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if !model.IsValidRole(role) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "invalid role")
	}

	user.Role = role
	if err := s.userRepo.Update(ctx, user); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *userService) Delete(ctx context.Context, id string) error {
	_, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "user not found")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.userRepo.Delete(ctx, id)
}