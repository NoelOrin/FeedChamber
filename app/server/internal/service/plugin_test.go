package service

import (
	"context"
	"testing"

	"orinforum/internal/model"
	"orinforum/internal/repository"

	"github.com/stretchr/testify/require"
)

type stubPluginRepo struct {
	items map[string]*model.Plugin
}

func (s *stubPluginRepo) Create(_ context.Context, p *model.Plugin) error {
	s.items[p.ID] = p
	return nil
}
func (s *stubPluginRepo) Update(_ context.Context, p *model.Plugin) error {
	s.items[p.ID] = p
	return nil
}
func (s *stubPluginRepo) Get(_ context.Context, id string) (*model.Plugin, error) {
	if p, ok := s.items[id]; ok {
		return p, nil
	}
	return nil, repository.ErrNotFound
}
func (s *stubPluginRepo) List(_ context.Context) ([]model.Plugin, error) {
	items := make([]model.Plugin, 0, len(s.items))
	for _, p := range s.items {
		items = append(items, *p)
	}
	return items, nil
}
func (s *stubPluginRepo) Delete(_ context.Context, id string) error {
	delete(s.items, id)
	return nil
}

func TestPluginService_InstallToggleDelete(t *testing.T) {
	ctx := context.Background()
	svc := NewPluginService(&stubPluginRepo{items: map[string]*model.Plugin{}})

	installed, err := svc.Install(ctx, PluginInput{
		ID:       "analytics",
		Name:     "Analytics",
		Version:  "1.0.0",
		Settings: `{"enabled":true}`,
		Enabled:  true,
	})
	require.NoError(t, err)
	require.Equal(t, "analytics", installed.ID)

	require.NoError(t, svc.SetEnabled(ctx, "analytics", false))
	items, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.False(t, items[0].Enabled)

	require.NoError(t, svc.Delete(ctx, "analytics"))
	items, err = svc.List(ctx)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestPluginService_RejectsInvalidSettings(t *testing.T) {
	ctx := context.Background()
	svc := NewPluginService(&stubPluginRepo{items: map[string]*model.Plugin{}})
	_, err := svc.Install(ctx, PluginInput{ID: "bad", Name: "Bad", Settings: "not-json"})
	require.Error(t, err)
}
