package service

import (
	"context"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/repository"
)

type LeaderboardService interface {
	List(ctx context.Context, typ string, page, pageSize int) ([]model.User, int, error)
}

type leaderboardService struct {
	users repository.UserRepository
}

func NewLeaderboardService(users repository.UserRepository) LeaderboardService {
	return &leaderboardService{users: users}
}

func (s *leaderboardService) List(ctx context.Context, typ string, page, pageSize int) ([]model.User, int, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	offset := (page - 1) * pageSize
	var (
		users []model.User
		err   error
	)
	switch typ {
	case "checkin":
		users, err = s.users.ListByCheckIns(ctx, offset, pageSize)
	case "posts":
		users, err = s.users.ListByPostCount(ctx, offset, pageSize)
	default:
		users, err = s.users.ListByPoints(ctx, offset, pageSize)
	}
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	var total int64
	switch typ {
	case "posts":
		total, err = s.users.CountDistinctPostAuthors(ctx)
	default:
		total, err = s.users.Count(ctx)
	}
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return users, int(total), nil
}
