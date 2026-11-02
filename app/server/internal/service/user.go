package service

import (
	"context"
	"errors"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/repository"
)

// systemGroupSyncer syncs a user into the system group matching their role.
type systemGroupSyncer interface {
	SyncUserSystemGroup(ctx context.Context, userID, roleName string) error
}

type userService struct {
	userRepo  repository.UserRepository
	sessions  SessionStore
	permSvc   PermissionChecker
	txMgr     db.TransactionManager
	groupSync systemGroupSyncer
}

func NewUserService(userRepo repository.UserRepository, sessions SessionStore, permSvc PermissionChecker, txMgr db.TransactionManager) UserService {
	return &userService{userRepo: userRepo, sessions: sessions, permSvc: permSvc, txMgr: txMgr}
}

// SetGroupSync injects system-group membership sync (best effort).
func (s *userService) SetGroupSync(syncer systemGroupSyncer) {
	s.groupSync = syncer
}

func (s *userService) GetByID(ctx context.Context, id string) (*model.User, error) {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "user not found")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return user, nil
}

func (s *userService) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.USER_NOT_FOUND)
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return user, nil
}

func (s *userService) List(ctx context.Context, page, pageSize int) ([]model.User, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	users, err := s.userRepo.List(ctx, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return users, nil
}

func (s *userService) Search(ctx context.Context, keyword, role string, page, pageSize int) ([]model.User, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	users, err := s.userRepo.Search(ctx, keyword, role, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return users, nil
}

func (s *userService) UpdateProfile(ctx context.Context, id string, username, displayName, avatar, bio, coverImage, profileLayout *string) error {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "user not found")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if username != nil && *username != user.Username {
		existing, _ := s.userRepo.FindByUsername(ctx, *username)
		if existing != nil {
			return pkg.NewAppError(pkg.USERNAME_EXISTS)
		}
		user.Username = *username
	}
	if displayName != nil {
		if appErr := validateDisplayName(*displayName); appErr != nil {
			return appErr
		}
		user.DisplayName = *displayName
	}
	if avatar != nil {
		user.Avatar = *avatar
	}
	if bio != nil {
		user.Bio = *bio
	}
	if coverImage != nil {
		user.CoverImage = *coverImage
	}
	if profileLayout != nil {
		user.ProfileLayout = *profileLayout
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *userService) UpdateRole(ctx context.Context, id, role string) error {
	if !model.IsValidRole(role) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "无效的角色")
	}

	var user *model.User
	err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		u, err := s.userRepo.FindByID(txCtx, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return pkg.NewAppError(pkg.NOT_FOUND, "user not found")
			}
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		user = u

		// 如果当前角色有角色管理权限而新角色没有，确保不是最后一个有该权限的用户
		if s.permSvc != nil && s.permSvc.HasPermission(user.Role, model.PermRoleManage) && !s.permSvc.HasPermission(role, model.PermRoleManage) {
			count, err := s.userRepo.CountUsersByPermission(txCtx, model.PermRoleManage)
			if err != nil {
				return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
			if count <= 1 {
				return pkg.NewAppError(pkg.FORBIDDEN, "无法移除最后一个管理员的角色管理权限")
			}
		}

		user.Role = role
		return s.userRepo.Update(txCtx, user)
	})
	if err != nil {
		return err
	}

	// 吊销该用户所有会话，使角色变更（包括封号 role="ban"）立即生效
	if s.sessions != nil {
		_ = s.sessions.RevokeAllForUser(ctx, user.ID)
	}

	// best-effort: 同步系统组成员身份
	if s.groupSync != nil {
		_ = s.groupSync.SyncUserSystemGroup(ctx, user.ID, role)
	}

	return nil
}

func (s *userService) Delete(ctx context.Context, id string) error {
	return s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		user, err := s.userRepo.FindByID(txCtx, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return pkg.NewAppError(pkg.NOT_FOUND, "user not found")
			}
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}

		// 如果当前角色有角色管理权限，确保不是最后一个有该权限的用户
		if s.permSvc != nil && s.permSvc.HasPermission(user.Role, model.PermRoleManage) {
			count, err := s.userRepo.CountUsersByPermission(txCtx, model.PermRoleManage)
			if err != nil {
				return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
			if count <= 1 {
				return pkg.NewAppError(pkg.FORBIDDEN, "无法删除最后一个管理员")
			}
		}

		return s.userRepo.Delete(txCtx, user.ID)
	})
}

func (s *userService) GetPublicProfile(ctx context.Context, id string) (*model.PublicProfileResponse, error) {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "user not found")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	postCount, _ := s.userRepo.CountPosts(ctx, id)
	commentCount, _ := s.userRepo.CountCommentsByUser(ctx, id)
	likeCount, _ := s.userRepo.CountLikesReceived(ctx, id)
	return &model.PublicProfileResponse{
		User:         user.ToPublic(),
		PostCount:    int(postCount),
		CommentCount: int(commentCount),
		LikeCount:    int(likeCount),
	}, nil
}

func (s *userService) GetActivity(ctx context.Context, userID string, year int) (*model.ActivityResponse, error) {
	entries, err := s.userRepo.GetActivityStats(ctx, userID, year)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return &model.ActivityResponse{Year: year, Entries: entries}, nil
}
