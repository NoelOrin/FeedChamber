package service

import (
	"context"
	"testing"

	"orinforum/internal/mock"
	"orinforum/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pstr(s string) *string { return &s }

func newTestUserService(userRepo *mock.UserRepository) UserService {
	return NewUserService(userRepo, nil, nil, nil)
}

func TestUpdateProfile_SetsDisplayName(t *testing.T) {
	repo := mock.NewUserRepository()
	_ = repo.Create(context.Background(), &model.User{
		ID: "u1", Username: "alice", DisplayName: "alice",
	})
	svc := newTestUserService(repo)

	err := svc.UpdateProfile(context.Background(), "u1", nil, pstr("爱丽丝"), nil, nil, nil, nil)
	require.Nil(t, err)

	got, _ := repo.FindByID(context.Background(), "u1")
	assert.Equal(t, "爱丽丝", got.DisplayName)
	assert.Equal(t, "alice", got.Username)
}

func TestUpdateProfile_RejectsInvalidDisplayName(t *testing.T) {
	repo := mock.NewUserRepository()
	_ = repo.Create(context.Background(), &model.User{
		ID: "u1", Username: "alice", DisplayName: "alice",
	})
	svc := newTestUserService(repo)

	err := svc.UpdateProfile(context.Background(), "u1", nil, pstr("bad name"), nil, nil, nil, nil)
	require.NotNil(t, err)
}
