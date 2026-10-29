package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/repository"
)

type groupService struct {
	repo    repository.GroupRepository
	permSvc PermissionService
}

func NewGroupService(repo repository.GroupRepository, permSvc PermissionService) GroupService {
	return &groupService{repo: repo, permSvc: permSvc}
}

func (s *groupService) List(ctx context.Context) ([]model.Group, error) {
	return s.repo.List(ctx)
}

func (s *groupService) Create(ctx context.Context, name, displayName, visibility string) (*model.Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "组名称不能为空")
	}
	if visibility == "" {
		visibility = model.GroupVisibilityPublic
	}
	if !model.IsValidGroupVisibility(visibility) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "无效的可见性")
	}
	if displayName == "" {
		displayName = name
	}
	// 禁止与系统组同名
	for _, sys := range model.DefaultSystemGroups {
		if sys.Name == name {
			return nil, pkg.NewAppError(pkg.FORBIDDEN, "不能创建与系统组同名的组")
		}
	}
	if existing, err := s.repo.FindByName(ctx, name); err == nil && existing != nil {
		return nil, pkg.NewAppError(pkg.ALREADY_EXISTS, "组已存在")
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	g := model.NewGroup(name, displayName, visibility)
	if err := s.repo.Create(ctx, g); err != nil {
		return nil, pkg.NewAppError(pkg.ALREADY_EXISTS, "组已存在")
	}
	return g, nil
}

func (s *groupService) Update(ctx context.Context, id, displayName, visibility string) (*model.Group, error) {
	g, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "组不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if g.IsSystem && visibility != "" && visibility != g.Visibility {
		// 系统组允许改 displayName，但禁止改 visibility 为非法值；允许保持
	}
	if displayName != "" {
		g.DisplayName = displayName
	}
	if visibility != "" {
		if !model.IsValidGroupVisibility(visibility) {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "无效的可见性")
		}
		if !g.IsSystem {
			g.Visibility = visibility
		}
	}
	if err := s.repo.Update(ctx, g); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return g, nil
}

func (s *groupService) Delete(ctx context.Context, id string) error {
	g, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "组不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if g.IsSystem {
		return pkg.NewAppError(pkg.FORBIDDEN, "系统组不能删除")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if s.permSvc != nil {
		_ = s.permSvc.ReloadGroupCache(ctx)
	}
	return nil
}

func (s *groupService) ListMembers(ctx context.Context, groupID string, page, pageSize int) ([]model.GroupMember, int, error) {
	if _, err := s.repo.FindByID(ctx, groupID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, 0, pkg.NewAppError(pkg.NOT_FOUND, "组不存在")
		}
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	members, err := s.repo.ListMembers(ctx, groupID, offset, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	total, err := s.repo.CountMembers(ctx, groupID)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return members, int(total), nil
}

func (s *groupService) AddMember(ctx context.Context, groupID, userID, role string) error {
	if userID == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "用户 ID 不能为空")
	}
	if _, err := s.repo.FindByID(ctx, groupID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "组不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if role == "" {
		role = model.GroupMemberRoleMember
	}
	if !model.IsValidGroupMemberRole(role) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "无效的成员角色")
	}
	m := model.NewGroupMember(groupID, userID, role)
	if err := s.repo.AddMember(ctx, m); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *groupService) RemoveMember(ctx context.Context, groupID, userID string) error {
	if _, err := s.repo.FindByID(ctx, groupID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "组不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.repo.RemoveMember(ctx, groupID, userID); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *groupService) SyncUserSystemGroup(ctx context.Context, userID, roleName string) error {
	if userID == "" || roleName == "" {
		return nil
	}
	// 先移出所有系统组，再加入目标系统组
	if err := s.repo.RemoveUserFromSystemGroups(ctx, userID); err != nil {
		return err
	}
	g, err := s.repo.FindByName(ctx, roleName)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil // 系统组尚未 seed 时忽略
		}
		return err
	}
	if !g.IsSystem {
		return nil
	}
	return s.repo.AddMember(ctx, &model.GroupMember{
		GroupID:  g.ID,
		UserID:   userID,
		Role:     model.GroupMemberRoleMember,
		JoinedAt: time.Now(),
	})
}

func (s *groupService) SyncGroupPermissions(ctx context.Context, groupID string, permCodes []string) error {
	if _, err := s.repo.FindByID(ctx, groupID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "组不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.repo.SyncGroupPermissions(ctx, groupID, permCodes); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if s.permSvc != nil {
		if err := s.permSvc.ReloadGroupCache(ctx); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	return nil
}

func (s *groupService) GetGroupPermissions(ctx context.Context, groupID string) ([]string, error) {
	if _, err := s.repo.FindByID(ctx, groupID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "组不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.repo.GetGroupPermissions(ctx, groupID)
}

// BindSystemGroupSync wires best-effort system group membership sync into user/auth services.
func BindSystemGroupSync(groupSvc GroupService, userSvc UserService, authSvc AuthService) {
	if groupSvc == nil {
		return
	}
	if u, ok := userSvc.(*userService); ok {
		u.SetGroupSync(groupSvc)
	}
	if a, ok := authSvc.(*authService); ok {
		a.SetGroupSync(groupSvc)
	}
}
