package service

import (
	"context"
	"testing"

	"orinforum/internal/model"
	"orinforum/internal/pkg"

	"github.com/stretchr/testify/require"
)

type stubThemePrefRepo struct {
	values map[string]string
}

func (s *stubThemePrefRepo) Get(_ context.Context, userID, key string) (string, error) {
	if v, ok := s.values[userID+"\x00"+key]; ok {
		return v, nil
	}
	return "", pkg.NewAppError(pkg.NOT_FOUND, "not found")
}
func (s *stubThemePrefRepo) Set(_ context.Context, userID, key, value string) error {
	s.values[userID+"\x00"+key] = value
	return nil
}
func (s *stubThemePrefRepo) ListByKey(_ context.Context, _, _ string, _, _ int) ([]model.UserPreference, error) {
	return nil, nil
}
func (s *stubThemePrefRepo) ListUsersWithKeyDefault(_ context.Context, _, _ string, _, _ int) ([]string, error) {
	return nil, nil
}

func TestThemeService_ListAndPreference(t *testing.T) {
	ctx := context.Background()
	svc := NewThemeService(&stubThemePrefRepo{values: map[string]string{}})

	themes := svc.List()
	require.Len(t, themes, 2)
	require.Equal(t, "orinlight", themes[0].ID)
	require.Equal(t, "orindark", themes[1].ID)

	pref, err := svc.GetPreference(ctx, "user-1")
	require.NoError(t, err)
	require.Equal(t, "system", pref)

	require.NoError(t, svc.SetPreference(ctx, "user-1", "dark"))
	pref, err = svc.GetPreference(ctx, "user-1")
	require.NoError(t, err)
	require.Equal(t, "dark", pref)

	err = svc.SetPreference(ctx, "user-1", "neon")
	require.Error(t, err)
}
