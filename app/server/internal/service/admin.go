package service

import (
	"context"
	"encoding/json"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/pkg/kvstore"
	"orinforum/internal/repository"
	"time"

	"github.com/sirupsen/logrus"
)

type adminService struct {
	repo     repository.AdminRepository
	userRepo repository.UserRepository
	postRepo repository.PostRepository
	kv       kvstore.Store
	sessions SessionStore
	permSvc  PermissionChecker
	txMgr    db.TransactionManager
}

func NewAdminService(repo repository.AdminRepository, userRepo repository.UserRepository, postRepo repository.PostRepository, kv kvstore.Store, sessions SessionStore, permSvc PermissionChecker, txMgr db.TransactionManager) AdminService {
	return &adminService{repo: repo, userRepo: userRepo, postRepo: postRepo, kv: kv, sessions: sessions, permSvc: permSvc, txMgr: txMgr}
}

const adminStatsCacheKey = "admin:stats"
const adminStatsTTL = 30 * time.Second

func (s *adminService) GetStats(ctx context.Context) (*AdminStats, error) {
	// Try cache first
	if cached, err := s.kv.Get(ctx, adminStatsCacheKey); err == nil {
		stats := &AdminStats{}
		if jsonErr := json.Unmarshal([]byte(cached), stats); jsonErr == nil {
			return stats, nil
		}
	}

	// Cache miss — execute the 7 COUNT queries
	stats := &AdminStats{}
	var err error

	if stats.TotalUsers, err = s.repo.CountUsers(ctx); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if stats.TotalPosts, err = s.repo.CountPosts(ctx); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if stats.TotalComments, err = s.repo.CountComments(ctx); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if stats.TodayNewUsers, err = s.repo.CountTodayUsers(ctx); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if stats.TodayNewPosts, err = s.repo.CountTodayPosts(ctx); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if stats.TodayNewComments, err = s.repo.CountTodayComments(ctx); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	// Populate cache (best-effort; ignore errors)
	if data, jsonErr := json.Marshal(stats); jsonErr == nil {
		_ = s.kv.Set(ctx, adminStatsCacheKey, string(data), adminStatsTTL)
	}

	return stats, nil
}

func (s *adminService) InvalidateStatsCache(ctx context.Context) error {
	return s.kv.Delete(ctx, adminStatsCacheKey)
}

func (s *adminService) GetDashboard(ctx context.Context) (*DashboardData, error) {
	stats, err := s.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	pendingReports, err := s.repo.CountPendingReports(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	recentUsers, err := s.repo.ListRecentUsers(ctx, 10)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	recentPosts, err := s.repo.ListRecentPosts(ctx, 10)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	return &DashboardData{
		Stats:          *stats,
		PendingReports: pendingReports,
		RecentUsers:    recentUsers,
		RecentPosts:    recentPosts,
	}, nil
}

func (s *adminService) SearchUsers(ctx context.Context, query string, page, pageSize int) ([]model.User, int64, error) {
	page, pageSize = pkg.NormalizePaginationCustom(page, pageSize, 50, 10)
	return s.repo.SearchUsers(ctx, query, (page-1)*pageSize, pageSize)
}

func (s *adminService) SearchPosts(ctx context.Context, query string, page, pageSize int) ([]model.Post, int64, error) {
	page, pageSize = pkg.NormalizePaginationCustom(page, pageSize, 50, 10)
	return s.repo.SearchPosts(ctx, query, (page-1)*pageSize, pageSize)
}

func (s *adminService) SearchComments(ctx context.Context, query string, page, pageSize int) ([]model.Content, int64, error) {
	page, pageSize = pkg.NormalizePaginationCustom(page, pageSize, 50, 10)
	return s.repo.SearchComments(ctx, query, (page-1)*pageSize, pageSize)
}

func (s *adminService) GetUserActivities(ctx context.Context, userID string, page, pageSize int) (*UserActivities, error) {
	page, pageSize = pkg.NormalizePaginationCustom(page, pageSize, 50, 10)
	offset := (page - 1) * pageSize

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.USER_NOT_FOUND)
	}

	activities := &UserActivities{User: *user}

	activities.Posts, activities.PostsTotal, err = s.repo.GetUserPosts(ctx, userID, offset, pageSize)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	activities.Comments, activities.CommentsTotal, err = s.repo.GetUserComments(ctx, userID, offset, pageSize)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	activities.ReportsAbout, activities.ReportsTotal, err = s.repo.GetUserReportsAbout(ctx, userID, offset, pageSize)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	activities.ReportsBy, activities.ReportsByTotal, err = s.repo.GetUserReportsBy(ctx, userID, offset, pageSize)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	return activities, nil
}

func (s *adminService) BatchUserAction(ctx context.Context, action string, ids []string, params map[string]string) error {
	if len(ids) == 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "请选择用户")
	}
	switch action {
	case "delete":
		err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
			adminCount, err := s.userRepo.CountUsersByPermission(txCtx, model.PermRoleManage)
			if err != nil {
				return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
			adminsInBatch := 0
			for _, id := range ids {
				u, err := s.userRepo.FindByID(txCtx, id)
				if err != nil {
					continue
				}
				if s.permSvc.HasPermission(u.Role, model.PermRoleManage) {
					adminsInBatch++
				}
			}
			if int(adminCount)-adminsInBatch <= 0 {
				return pkg.NewAppError(pkg.FORBIDDEN, "批量操作会删除所有管理员，至少保留一位管理员")
			}
			return s.repo.BatchDeleteUsers(txCtx, ids)
		})
		return err
	case "set_role":
		role := params["role"]
		if role == "" {
			return pkg.NewAppError(pkg.INVALID_PARAMS, "请指定角色")
		}

		// 在事务内执行计数检查 + 角色更新，防止 admin guard TOCTOU
		err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
			// 如果目标角色没有角色管理权限，检查是否会清空所有有角色管理权限的用户
			if s.permSvc != nil && !s.permSvc.HasPermission(role, model.PermRoleManage) {
				adminCount, err := s.userRepo.CountUsersByPermission(txCtx, model.PermRoleManage)
				if err != nil {
					return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
				}
				adminsInBatch := 0
				for _, id := range ids {
					u, err := s.userRepo.FindByID(txCtx, id)
					if err != nil {
						continue
					}
					if s.permSvc.HasPermission(u.Role, model.PermRoleManage) {
						adminsInBatch++
					}
				}
				if int(adminCount)-adminsInBatch <= 0 {
					return pkg.NewAppError(pkg.FORBIDDEN, "批量操作会删除所有管理员的角色管理权限")
				}
			}

			return s.repo.BatchUpdateUserRole(txCtx, ids, role)
		})
		if err != nil {
			return err
		}
		// 吊销受影响用户的全部会话，使角色变更（包括封号 role="ban"）立即生效
		if s.sessions != nil {
			for _, id := range ids {
				user, err := s.userRepo.FindByID(ctx, id)
				if err != nil {
					continue
				}
				if err := s.sessions.RevokeAllForUser(ctx, user.ID); err != nil {
					logrus.Warnf("admin: revoke sessions for user %s failed: %v", user.ID, err)
				}
			}
		}
		return nil
	default:
		return pkg.NewAppError(pkg.INVALID_PARAMS, "不支持的操作: "+action)
	}
}

func (s *adminService) BatchPostAction(ctx context.Context, action string, ids []string, actorRole string) error {
	if len(ids) == 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "请选择帖子")
	}
	switch action {
	case "delete":
		if s.permSvc != nil && s.postRepo != nil {
			if err := s.checkSpacePerms(ctx, ids, actorRole, model.PermPostDelete); err != nil {
				return err
			}
		}
		return s.repo.BatchDeletePosts(ctx, ids)
	case "pin":
		if s.permSvc != nil && s.postRepo != nil {
			if err := s.checkSpacePerms(ctx, ids, actorRole, model.PermPostPin); err != nil {
				return err
			}
		}
		return s.repo.BatchTogglePinPosts(ctx, ids, true)
	case "unpin":
		if s.permSvc != nil && s.postRepo != nil {
			if err := s.checkSpacePerms(ctx, ids, actorRole, model.PermPostPin); err != nil {
				return err
			}
		}
		return s.repo.BatchTogglePinPosts(ctx, ids, false)
	case "lock":
		if s.permSvc != nil && s.postRepo != nil {
			if err := s.checkSpacePerms(ctx, ids, actorRole, model.PermPostLock); err != nil {
				return err
			}
		}
		return s.repo.BatchToggleLockPosts(ctx, ids, true)
	case "unlock":
		if s.permSvc != nil && s.postRepo != nil {
			if err := s.checkSpacePerms(ctx, ids, actorRole, model.PermPostLock); err != nil {
				return err
			}
		}
		return s.repo.BatchToggleLockPosts(ctx, ids, false)
	default:
		return pkg.NewAppError(pkg.INVALID_PARAMS, "不支持的操作: "+action)
	}
}

// checkSpacePerms 批量检查：加载所有帖子、提取唯一版块、逐版核实权限。
// 任一版块拒绝则整体拒绝。
func (s *adminService) checkSpacePerms(ctx context.Context, ids []string, actorRole, permCode string) error {
	spaceSet := make(map[string]struct{})
	for _, id := range ids {
		post, err := s.postRepo.FindByID(ctx, id)
		if err != nil {
			continue // 已不存在的帖子跳过（由后续 repo 层处理）
		}
		if post.SpaceID != "" {
			spaceSet[post.SpaceID] = struct{}{}
		}
	}
	for spaceID := range spaceSet {
		if !s.permSvc.HasPermissionInSpace(actorRole, spaceID, permCode) {
			return pkg.NewAppError(pkg.FORBIDDEN, "无权对此版块的帖子执行批量操作")
		}
	}
	return nil
}

func (s *adminService) BatchDeleteComments(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "请选择评论")
	}
	return s.repo.BatchDeleteComments(ctx, ids)
}

func (s *adminService) ReconcileCounters(ctx context.Context) (int, error) {
	return s.repo.ReconcileCounters(ctx)
}
