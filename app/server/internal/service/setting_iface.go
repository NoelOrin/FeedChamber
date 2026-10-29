package service

import (
	"context"
)

type SettingService interface {
	Get(ctx context.Context, key string) (string, error)
	GetAll(ctx context.Context) (map[string]string, error)
	Set(ctx context.Context, key, value string) error
	BatchSet(ctx context.Context, settings map[string]string) error
	Delete(ctx context.Context, key string) error
}
