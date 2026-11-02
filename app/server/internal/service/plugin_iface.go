package service

import (
	"context"

	"feedchamber/internal/model"
)

type PluginInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Description string `json:"description"`
	Homepage    string `json:"homepage"`
	Settings    string `json:"settings"`
	Enabled     bool   `json:"enabled"`
}

type PluginService interface {
	Install(ctx context.Context, input PluginInput) (*model.Plugin, error)
	List(ctx context.Context) ([]model.Plugin, error)
	Update(ctx context.Context, input PluginInput) error
	SetEnabled(ctx context.Context, id string, enabled bool) error
	Delete(ctx context.Context, id string) error
}
