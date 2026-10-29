package service

import (
	"context"

	"orinforum/internal/model"
)

type RegisterRequest struct {
	Username   string `json:"username"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	InviteCode string `json:"invite_code,omitempty"`

	IP        string `json:"-"` // 由 handler 填充，用于创建服务端会话
	UserAgent string `json:"-"` // 由 handler 填充，用于记录设备名
}

type LoginRequest struct {
	Login    string `json:"login"` // email or username
	Password string `json:"password"`
	IP       string `json:"-"` // 由 handler 填充，不序列化

	UserAgent string `json:"-"` // 由 handler 填充，用于记录设备名
}

// AuthResponse is the login/register payload handed to the client.
// Token is the opaque session id (the client keeps calling it
// "access_token"); there is no refresh token anymore — the session itself
// slides and rotates.
type AuthResponse struct {
	Token              string             `json:"access_token,omitempty"`
	User               model.UserResponse `json:"user,omitempty"`
	MustChangePassword bool               `json:"must_change_password,omitempty"`
	MFARequired        bool               `json:"mfa_required,omitempty"`
	MFAToken           string             `json:"mfa_token,omitempty"`
}

type AuthService interface {
	Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error)
	// ApproveRegistration 将 pending_user 审批为 user。
	ApproveRegistration(ctx context.Context, userID string) error
	Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error)
	ChangePassword(ctx context.Context, username, oldPassword, newPassword string) error
	FirstChangePassword(ctx context.Context, username, newPassword string, name *string) error
	ResetPassword(ctx context.Context, username, newPassword string) error
	ResetPasswordByEmail(ctx context.Context, email, newPassword string) error
	SendVerifyEmail(ctx context.Context, userID string) error
	VerifyEmail(ctx context.Context, userID, code string) error
}

// SecManager 定义暴力破解防御接口，由 gateway.SecurityManager 实现。
type SecManager interface {
	RecordFailedLogin(ctx context.Context, ip, username string)
	ClearFailedLogin(ctx context.Context, ip, username string)
}
