package service

import (
	"context"
	"errors"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/pkg/kvstore"
	"orinforum/internal/repository"
)

const (
	settingCacheTTL = 60 * time.Second
	// settingMissingTTL caches "this key has no row" so an unseeded key does not
	// cost a DB round-trip on every single request. Short on purpose: creating
	// the row through Set/Delete still invalidates it immediately, but a row
	// inserted by another process should surface within seconds.
	settingMissingTTL = 10 * time.Second
	// settingMissingMarker is stored in KV in place of a value to represent a
	// known miss. It cannot collide with a real setting value: the leading NUL
	// is not producible from the settings UI or the seed table.
	settingMissingMarker = "\x00__missing__"
)

type settingService struct {
	repo repository.SettingRepository
	kv   kvstore.Store
}

func NewSettingService(repo repository.SettingRepository, kv kvstore.Store) SettingService {
	return &settingService{repo: repo, kv: kv}
}

func (s *settingService) Get(ctx context.Context, key string) (string, error) {
	cacheKey := "setting:" + key
	if val, err := s.kv.Get(ctx, cacheKey); err == nil {
		if val == settingMissingMarker {
			return "", pkg.NewAppError(pkg.NOT_FOUND, "设置项不存在")
		}
		return val, nil
	}
	setting, err := s.repo.Get(ctx, key)
	if err != nil {
		// Distinguish "this key has no row" from "the lookup failed". Both used
		// to return NOT_FOUND, so a DB outage read as an absent setting and
		// every gate silently fell back to its default. Only a genuine miss is
		// NOT_FOUND (callers treat that as "use the default"); anything else is
		// an internal error and callers must fail closed.
		if errors.Is(err, repository.ErrNotFound) {
			// Negative-cache the miss: without this, an unseeded key re-queried
			// the settings table on every request and logged GORM's
			// "record not found" each time.
			_ = s.kv.Set(ctx, cacheKey, settingMissingMarker, settingMissingTTL)
			return "", pkg.NewAppError(pkg.NOT_FOUND, "设置项不存在")
		}
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	_ = s.kv.Set(ctx, cacheKey, setting.Value, settingCacheTTL)
	return setting.Value, nil
}

func (s *settingService) GetAll(ctx context.Context) (map[string]string, error) {
	return s.repo.GetAll(ctx)
}

// validateSetting 拒绝会让功能静默失真的取值。目前只有审核模式：
// 写进一个不认识的模式，读取侧会回落到默认值，后台看起来「保存成功」却没有生效。
func validateSetting(key, value string) error {
	if key == model.SettingModerationMode && !model.ValidModerationMode(value) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "无效的审核模式（可选 off / async / external / manual）")
	}
	return nil
}

func (s *settingService) Set(ctx context.Context, key, value string) error {
	if err := validateSetting(key, value); err != nil {
		return err
	}
	if err := s.repo.Set(ctx, key, value); err != nil {
		return err
	}
	_ = s.kv.Delete(ctx, "setting:"+key)
	return nil
}

func (s *settingService) BatchSet(ctx context.Context, settings map[string]string) error {
	for key, value := range settings {
		if err := validateSetting(key, value); err != nil {
			return err
		}
	}
	if err := s.repo.BatchSet(ctx, settings); err != nil {
		return err
	}
	for key := range settings {
		_ = s.kv.Delete(ctx, "setting:"+key)
	}
	return nil
}

func (s *settingService) Delete(ctx context.Context, key string) error {
	if err := s.repo.Delete(ctx, key); err != nil {
		return err
	}
	_ = s.kv.Delete(ctx, "setting:"+key)
	return nil
}
