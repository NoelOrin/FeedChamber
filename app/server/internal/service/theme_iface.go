package service

import (
	"context"

	"orinforum/internal/model"
)

type ThemeService interface {
	List() []model.Theme
	GetPreference(ctx context.Context, userID string) (string, error)
	SetPreference(ctx context.Context, userID, preference string) error
}
