package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg/testnats"
	"feedchamber/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// XP / 等级配置（xp_rules / trust_levels / xp_streak_bonuses）在进程内缓存，
// 写配置后必须让**其它实例**也立即失效；否则多实例下改分值最坏 60s 不一致，
// 与「改分值立即对全员生效」的契约冲突。
func TestTrust_ConfigChangeSyncsAcrossInstances(t *testing.T) {
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "trust-sync.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(
		&model.XPRule{}, &model.XPStreakBonus{}, &model.TrustLevel{}, &model.XPAdjustment{}, &model.User{},
	))
	require.NoError(t, store.SQL().DB().Create(&model.XPRule{
		Metric: model.MetricPostApproved, Delta: 10, Enabled: true,
	}).Error)

	kv := testnats.NewKVStore(t, "trust_sync_test")
	users := repository.NewUserRepository(store)
	trustRepo := repository.NewTrustRepository(store)

	writer := NewTrustService(users, trustRepo, nil, kv)
	reader := NewTrustService(users, trustRepo, nil, kv)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	require.NoError(t, reader.StartSyncSubscriber(ctx))

	// 预热 reader 的配置缓存。
	cfg, err := reader.(*trustService).config(ctx)
	require.NoError(t, err)
	require.Len(t, cfg.rules, 1)
	require.Equal(t, 10, cfg.rules[0].Delta)

	// 另一实例改分值：reader 不应等 60s TTL 才看到。
	require.NoError(t, writer.SaveXPRule(ctx, &model.XPRule{
		Metric: model.MetricPostApproved, Delta: 25, Enabled: true,
	}))

	deadline := time.Now().Add(3 * time.Second)
	for {
		cfg, err = reader.(*trustService).config(ctx)
		require.NoError(t, err)
		if len(cfg.rules) == 1 && cfg.rules[0].Delta == 25 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("reader 未在 3s 内看到新分值，仍是 %+v", cfg.rules)
		}
		time.Sleep(20 * time.Millisecond)
	}
	assert.Equal(t, 25, cfg.rules[0].Delta)
}
