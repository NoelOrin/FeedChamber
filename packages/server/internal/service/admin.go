package service

import (
	"context"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
)

// AdminStats holds aggregated forum statistics.
type AdminStats struct {
	TotalUsers       int64 `json:"totalUsers"`
	TotalPosts       int64 `json:"totalPosts"`
	TotalComments    int64 `json:"totalComments"`
	TotalCategories  int64 `json:"totalCategories"`
	TodayNewUsers    int64 `json:"todayNewUsers"`
	TodayNewPosts    int64 `json:"todayNewPosts"`
	TodayNewComments int64 `json:"todayNewComments"`
}

type AdminService interface {
	GetStats(ctx context.Context) (*AdminStats, error)
}

type adminService struct {
	repo repository.AdminRepository
}

func NewAdminService(repo repository.AdminRepository) AdminService {
	return &adminService{repo: repo}
}

func (s *adminService) GetStats(ctx context.Context) (*AdminStats, error) {
	stats := &AdminStats{}

	totalUsers, err := s.repo.CountUsers(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	stats.TotalUsers = totalUsers

	totalPosts, err := s.repo.CountPosts(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	stats.TotalPosts = totalPosts

	totalComments, err := s.repo.CountComments(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	stats.TotalComments = totalComments

	totalCategories, err := s.repo.CountCategories(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	stats.TotalCategories = totalCategories

	todayUsers, err := s.repo.CountTodayUsers(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	stats.TodayNewUsers = todayUsers

	todayPosts, err := s.repo.CountTodayPosts(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	stats.TodayNewPosts = todayPosts

	todayComments, err := s.repo.CountTodayComments(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	stats.TodayNewComments = todayComments

	return stats, nil
}
