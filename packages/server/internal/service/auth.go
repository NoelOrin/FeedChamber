package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService interface {
	Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error)
	Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error)
	RefreshToken(ctx context.Context, username, userUUID, role string) (string, error)
	ChangePassword(ctx context.Context, username, oldPassword, newPassword string) error
	FirstChangePassword(ctx context.Context, username, newPassword string, name *string) error
	ResetPassword(ctx context.Context, username, newPassword string) error
}

type authService struct {
	userRepo repository.UserRepository
}

func NewAuthService(userRepo repository.UserRepository) AuthService {
	return &authService{userRepo: userRepo}
}

type LoginRequest struct {
	Login    string `json:"login"`    // email or username
	Password string `json:"password"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token        string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token"`
	User         model.User `json:"user"`
}

func (s *authService) Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error) {
	user, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err == nil && user != nil {
		return nil, pkg.NewAppError(pkg.USERNAME_EXISTS)
	}
	user, err = s.userRepo.FindByEmail(ctx, req.Email)
	if err == nil && user != nil {
		return nil, pkg.NewAppError(pkg.EMAIL_EXISTS)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	userUUID := pkg.NewID()
	user = &model.User{
		ID:        userUUID,
		UUID:      userUUID,
		Username:  req.Username,
		Email:     req.Email,
		Password:  string(hash),
		Role:      "user",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	token, refreshToken, err := generateTokenPair(user.Username, userUUID, user.Role)
	if err != nil {
		return nil, err
	}

	user.Password = ""
	return &AuthResponse{Token: token, RefreshToken: refreshToken, User: *user}, nil
}

func (s *authService) Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error) {
	login := strings.TrimSpace(req.Login)

	var user *model.User
	var err error

	// Try email first, then username
	user, err = s.userRepo.FindByEmail(ctx, login)
	if err != nil {
		user, err = s.userRepo.FindByUsername(ctx, login)
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.USER_NOT_FOUND)
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, pkg.NewAppError(pkg.INVALID_PASSWORD)
	}

	if model.HasBanRole(user.Role) {
		return nil, pkg.NewAppError(pkg.USER_BANNED)
	}

	token, refreshToken, err := generateTokenPair(user.Username, user.UUID, user.Role)
	if err != nil {
		return nil, err
	}

	user.Password = ""
	return &AuthResponse{Token: token, RefreshToken: refreshToken, User: *user}, nil
}

func (s *authService) RefreshToken(_ context.Context, username, userUUID, role string) (string, error) {
	token, err := pkg.GenerateToken(username, userUUID, role)
	if err != nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return token, nil
}

func (s *authService) ChangePassword(ctx context.Context, username, oldPassword, newPassword string) error {
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkg.NewAppError(pkg.USER_NOT_FOUND)
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		return pkg.NewAppError(pkg.INVALID_PASSWORD)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	user.Password = string(hash)
	return s.userRepo.Update(ctx, user)
}

func (s *authService) FirstChangePassword(ctx context.Context, username, newPassword string, name *string) error {
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return pkg.NewAppError(pkg.USER_NOT_FOUND)
	}
	if user.Role != "admin" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "only admin supports first change password")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	user.Password = string(hash)

	if name != nil && *name != "" && *name != user.Username {
		existing, _ := s.userRepo.FindByUsername(ctx, *name)
		if existing != nil {
			return pkg.NewAppError(pkg.USERNAME_EXISTS)
		}
		user.Username = *name
	}

	return s.userRepo.Update(ctx, user)
}

func (s *authService) ResetPassword(ctx context.Context, username, newPassword string) error {
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return pkg.NewAppError(pkg.USER_NOT_FOUND)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	user.Password = string(hash)
	return s.userRepo.Update(ctx, user)
}

func generateTokenPair(username, userUUID, role string) (string, string, error) {
	var token, refreshToken string
	var err1, err2 error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		token, err1 = pkg.GenerateToken(username, userUUID, role)
	}()
	go func() {
		defer wg.Done()
		refreshToken, err2 = pkg.GenerateRefreshToken(username, userUUID, role)
	}()
	wg.Wait()
	if err1 != nil {
		return "", "", pkg.NewAppError(pkg.INTERNAL_ERROR, err1.Error())
	}
	if err2 != nil {
		return "", "", pkg.NewAppError(pkg.INTERNAL_ERROR, err2.Error())
	}
	return token, refreshToken, nil
}