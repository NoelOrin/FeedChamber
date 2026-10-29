package service

import (
	"context"
	"testing"

	"orinforum/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memModRepo struct {
	rules   []model.ModerationRule
	configs map[string]string
	// created 记录所有建单，供异步审核/外部审核的用例断言「是否留痕」。
	created []*model.ContentModeration
}

func (m *memModRepo) CreateModeration(_ context.Context, cm *model.ContentModeration) error {
	m.created = append(m.created, cm)
	return nil
}
func (m *memModRepo) GetModeration(context.Context, string) (*model.ContentModeration, error) {
	return nil, nil
}
func (m *memModRepo) ListModerations(context.Context, model.ModerationStatus, int, int) ([]model.ContentModeration, int64, error) {
	return nil, 0, nil
}
func (m *memModRepo) UpdateModerationStatus(context.Context, string, model.ModerationStatus, string) error {
	return nil
}
func (m *memModRepo) CreateRule(context.Context, *model.ModerationRule) error        { return nil }
func (m *memModRepo) GetRule(context.Context, string) (*model.ModerationRule, error) { return nil, nil }
func (m *memModRepo) ListRules(context.Context, int, int) ([]model.ModerationRule, int64, error) {
	return m.rules, int64(len(m.rules)), nil
}
func (m *memModRepo) ListEnabledRules(_ context.Context, scene model.ModerationScene) ([]model.ModerationRule, error) {
	var out []model.ModerationRule
	for _, r := range m.rules {
		if r.Enabled && r.Scene == scene {
			out = append(out, r)
		}
	}
	return out, nil
}
func (m *memModRepo) UpdateRule(context.Context, *model.ModerationRule) error { return nil }
func (m *memModRepo) DeleteRule(context.Context, string) error                { return nil }
func (m *memModRepo) GetConfig(_ context.Context, key string) (*model.ModerationConfig, error) {
	if v, ok := m.configs[key]; ok {
		return &model.ModerationConfig{Key: key, Value: v}, nil
	}
	return nil, nil
}
func (m *memModRepo) SetConfig(context.Context, string, string) error { return nil }
func (m *memModRepo) ListConfigs(context.Context) ([]model.ModerationConfig, error) {
	var out []model.ModerationConfig
	for k, v := range m.configs {
		out = append(out, model.ModerationConfig{Key: k, Value: v})
	}
	return out, nil
}

func TestCheckContent_KeywordBlockAndWhitelist(t *testing.T) {
	repo := &memModRepo{
		configs: map[string]string{
			model.ModerationConfigEnabled:        "true",
			model.ModerationConfigPostEnabled:    "true",
			model.ModerationConfigWhitelistRoles: `["admin"]`,
			model.ModerationConfigDefaultAction:  "flag",
		},
		rules: []model.ModerationRule{{
			Name:    "badword",
			Scene:   model.ModerationScenePost,
			Type:    "keyword",
			Pattern: "spamword",
			Action:  model.ModerationActionBlock,
			Enabled: true,
		}},
	}
	svc := NewModerationService(repo, nil, nil, nil, nil)

	action, err := svc.CheckContent(context.Background(), "post", "p1", "hello spamword here", model.ModerationScenePost, "user")
	require.NoError(t, err)
	assert.Equal(t, model.ModerationActionBlock, action)

	// whitelist role bypass
	action, err = svc.CheckContent(context.Background(), "post", "p1", "hello spamword here", model.ModerationScenePost, "admin")
	require.NoError(t, err)
	assert.Equal(t, model.ModerationActionAllow, action)
}
