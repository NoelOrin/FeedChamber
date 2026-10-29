package server

import (
	"path/filepath"
	"testing"

	"orinforum/internal/config"
	"orinforum/internal/db"
	"orinforum/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSettingTestStore(t *testing.T) db.Store {
	t.Helper()
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "setting.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(&model.Setting{}))
	return store
}

func settingValue(t *testing.T, store db.Store, key string) string {
	t.Helper()
	var s model.Setting
	require.NoError(t, store.SQL().DB().Where("key = ?", key).First(&s).Error)
	return s.Value
}

// 存量站点 post_approval=true：升级后继续「全部人工」，不会因为默认 async 静默放开。
func TestMigrateModerationMode_LegacyApprovalBecomesManual(t *testing.T) {
	store := newSettingTestStore(t)
	require.NoError(t, store.SQL().DB().Create(&model.Setting{Key: "post_approval", Value: "true"}).Error)

	migrateModerationMode(store)
	assert.Equal(t, model.ModerationModeManual, settingValue(t, store, model.SettingModerationMode))

	// 迁移之后再补齐默认设置，也不能把 manual 覆盖成 async。
	syncDefaultSettings(store)
	assert.Equal(t, model.ModerationModeManual, settingValue(t, store, model.SettingModerationMode))
}

// 没有旧开关（或为 false）的站点落到默认「发布后异步审核」。
func TestMigrateModerationMode_DefaultsToAsync(t *testing.T) {
	store := newSettingTestStore(t)
	require.NoError(t, store.SQL().DB().Create(&model.Setting{Key: "post_approval", Value: "false"}).Error)

	migrateModerationMode(store)
	assert.Equal(t, model.DefaultModerationMode, settingValue(t, store, model.SettingModerationMode))
	assert.Equal(t, model.ModerationModeAsync, settingValue(t, store, model.SettingModerationMode))
}

// 已存在的 moderation_mode 是管理员的选择，迁移必须幂等且不覆盖。
func TestMigrateModerationMode_KeepsExistingValue(t *testing.T) {
	store := newSettingTestStore(t)
	require.NoError(t, store.SQL().DB().Create(&model.Setting{Key: "post_approval", Value: "true"}).Error)
	require.NoError(t, store.SQL().DB().Create(&model.Setting{Key: model.SettingModerationMode, Value: model.ModerationModeOff}).Error)

	migrateModerationMode(store)
	assert.Equal(t, model.ModerationModeOff, settingValue(t, store, model.SettingModerationMode))
}
