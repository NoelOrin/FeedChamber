package service

import (
	"context"
	"testing"

	"orinforum/internal/mock"
	"orinforum/internal/model"
	"orinforum/internal/pkg"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubEmailService struct {
	sent    []string
	verify  map[string]string // email -> expected code
	failGet error
}

func (s *stubEmailService) SendCode(ctx context.Context, purpose Purpose, email string) error {
	s.sent = append(s.sent, email)
	if s.verify == nil {
		s.verify = map[string]string{}
	}
	s.verify[email] = "123456"
	return nil
}
func (s *stubEmailService) VerifyCode(ctx context.Context, purpose Purpose, email, code string) error {
	if s.failGet != nil {
		return s.failGet
	}
	if s.verify[email] != code {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "验证码错误")
	}
	return nil
}
func (s *stubEmailService) SendNotification(ctx context.Context, to, subject, htmlBody, textBody string) error {
	return nil
}

func TestAuthSendAndVerifyEmail(t *testing.T) {
	users := mock.NewUserRepository()
	_ = users.Create(context.Background(), &model.User{
		ID:    "u1",
		Email: "a@example.com",
		Role:  "user",
	})
	email := &stubEmailService{}
	svc := &authService{
		userRepo: users,
		emailSvc: email,
	}

	require.NoError(t, svc.SendVerifyEmail(context.Background(), "u1"))
	assert.Equal(t, []string{"a@example.com"}, email.sent)

	require.NoError(t, svc.VerifyEmail(context.Background(), "u1", "123456"))
	u, err := users.FindByID(context.Background(), "u1")
	require.NoError(t, err)
	assert.True(t, u.EmailVerified)
}

func TestAuthSendVerifyEmail_AlreadyVerified(t *testing.T) {
	users := mock.NewUserRepository()
	_ = users.Create(context.Background(), &model.User{
		ID:            "u1",
		Email:         "a@example.com",
		EmailVerified: true,
	})
	svc := &authService{userRepo: users, emailSvc: &stubEmailService{}}
	err := svc.SendVerifyEmail(context.Background(), "u1")
	require.Error(t, err)
	var appErr *pkg.AppError
	require.True(t, assert.ErrorAs(t, err, &appErr))
	assert.Equal(t, pkg.INVALID_PARAMS, appErr.Code)
}
