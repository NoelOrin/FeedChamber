package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
)

type pluginService struct {
	repo repository.PluginRepository
}

func NewPluginService(repo repository.PluginRepository) PluginService {
	return &pluginService{repo: repo}
}

func (s *pluginService) Install(ctx context.Context, input PluginInput) (*model.Plugin, error) {
	if input.ID == "" || input.Name == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "插件 ID 和名称不能为空")
	}
	if err := validatePluginSettings(input.Settings); err != nil {
		return nil, err
	}
	now := time.Now()
	plugin := &model.Plugin{
		ID:          input.ID,
		Name:        input.Name,
		Version:     input.Version,
		Author:      input.Author,
		Description: input.Description,
		Homepage:    input.Homepage,
		Settings:    input.Settings,
		Enabled:     input.Enabled,
		UpdatedAt:   now,
	}
	existing, err := s.repo.Get(ctx, input.ID)
	if err == nil && existing != nil {
		plugin.CreatedAt = existing.CreatedAt
		if err := s.repo.Update(ctx, plugin); err != nil {
			return nil, err
		}
		return plugin, nil
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	plugin.CreatedAt = now
	if err := s.repo.Create(ctx, plugin); err != nil {
		return nil, err
	}
	return plugin, nil
}

func (s *pluginService) List(ctx context.Context) ([]model.Plugin, error) {
	return s.repo.List(ctx)
}

func (s *pluginService) Update(ctx context.Context, input PluginInput) error {
	if input.ID == "" || input.Name == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "插件 ID 和名称不能为空")
	}
	if err := validatePluginSettings(input.Settings); err != nil {
		return err
	}
	plugin, err := s.repo.Get(ctx, input.ID)
	if err != nil {
		return err
	}
	plugin.Name = input.Name
	plugin.Version = input.Version
	plugin.Author = input.Author
	plugin.Description = input.Description
	plugin.Homepage = input.Homepage
	plugin.Settings = input.Settings
	plugin.Enabled = input.Enabled
	plugin.UpdatedAt = time.Now()
	return s.repo.Update(ctx, plugin)
}

func (s *pluginService) SetEnabled(ctx context.Context, id string, enabled bool) error {
	plugin, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	plugin.Enabled = enabled
	plugin.UpdatedAt = time.Now()
	return s.repo.Update(ctx, plugin)
}

func (s *pluginService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func validatePluginSettings(raw string) error {
	if raw == "" {
		return nil
	}
	var target any
	if err := json.Unmarshal([]byte(raw), &target); err != nil {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "插件 settings 必须是合法 JSON")
	}
	return nil
}
