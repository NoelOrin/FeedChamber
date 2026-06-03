package service

import (
	"context"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
)

type SettingService interface {
	Get(ctx context.Context, key string) (string, error)
	GetAll(ctx context.Context) (map[string]string, error)
	Set(ctx context.Context, key, value string) error
	BatchSet(ctx context.Context, settings map[string]string) error
	Delete(ctx context.Context, key string) error
}

type settingService struct {
	repo repository.SettingRepository
}

func NewSettingService(repo repository.SettingRepository) SettingService {
	return &settingService{repo: repo}
}

func (s *settingService) Get(ctx context.Context, key string) (string, error) {
	setting, err := s.repo.Get(ctx, key)
	if err != nil {
		return "", pkg.NewAppError(pkg.NOT_FOUND, "设置项不存在")
	}
	return setting.Value, nil
}

func (s *settingService) GetAll(ctx context.Context) (map[string]string, error) {
	return s.repo.GetAll(ctx)
}

func (s *settingService) Set(ctx context.Context, key, value string) error {
	return s.repo.Set(ctx, key, value)
}

func (s *settingService) BatchSet(ctx context.Context, settings map[string]string) error {
	return s.repo.BatchSet(ctx, settings)
}

func (s *settingService) Delete(ctx context.Context, key string) error {
	return s.repo.Delete(ctx, key)
}
