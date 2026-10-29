package service

import (
	"context"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
)

type themeService struct {
	prefs repository.UserPreferenceRepository
}

func NewThemeService(prefs repository.UserPreferenceRepository) ThemeService {
	return &themeService{prefs: prefs}
}

func (s *themeService) List() []model.Theme {
	return []model.Theme{
		{ID: "orinlight", Name: "明亮", Dark: false},
		{ID: "orindark", Name: "暗色", Dark: true},
	}
}

func (s *themeService) GetPreference(ctx context.Context, userID string) (string, error) {
	if userID == "" || s.prefs == nil {
		return "system", nil
	}
	value, err := s.prefs.Get(ctx, userID, model.PrefTheme)
	if err != nil {
		return "system", nil
	}
	return value, nil
}

func (s *themeService) SetPreference(ctx context.Context, userID, preference string) error {
	if userID == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "缺少用户 ID")
	}
	if preference != "light" && preference != "dark" && preference != "system" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "主题偏好必须为 light、dark 或 system")
	}
	if s.prefs == nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "偏好存储不可用")
	}
	return s.prefs.Set(ctx, userID, model.PrefTheme, preference)
}
