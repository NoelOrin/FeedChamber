package service

import (
	"context"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
)

type roleService struct {
	repo repository.RoleRepository
}

func NewRoleService(repo repository.RoleRepository) RoleService {
	return &roleService{repo: repo}
}

func (s *roleService) List(ctx context.Context) ([]model.Role, error) {
	return s.repo.List(ctx)
}

func (s *roleService) Create(ctx context.Context, name, displayName string) (*model.Role, error) {
	role := model.NewRole(name)
	if displayName != "" {
		role.DisplayName = displayName
	}
	if err := s.repo.Create(ctx, role); err != nil {
		return nil, pkg.NewAppError(pkg.ALREADY_EXISTS, "角色已存在")
	}
	// 刷新角色缓存
	s.reloadCache(ctx)
	return role, nil
}

func (s *roleService) Delete(ctx context.Context, id string) error {
	// Get role by ID to check its name
	roles, err := s.repo.List(ctx)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "无法获取角色列表")
	}

	var roleToDelete *model.Role
	for i := range roles {
		if roles[i].ID == id {
			roleToDelete = &roles[i]
			break
		}
	}
	if roleToDelete == nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "角色不存在")
	}

	// Check if role is built-in (admin, user, ban)
	for _, defaultRole := range model.DefaultRoles {
		if roleToDelete.Name == defaultRole.Name {
			return pkg.NewAppError(pkg.FORBIDDEN, "内置角色不能删除")
		}
	}

	// Check if any users are assigned to this role
	hasUsers, err := s.repo.HasUsersWithRole(ctx, roleToDelete.Name)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "检查角色使用情况失败")
	}
	if hasUsers {
		return pkg.NewAppError(pkg.FORBIDDEN, "角色下存在用户，请先转移用户再删除")
	}

	// Delete role_permissions associations
	if err := s.repo.DeleteRolePermissions(ctx, roleToDelete.Name); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "删除角色权限关联失败")
	}

	// Delete the role
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	// Reload role cache
	s.reloadCache(ctx)
	return nil
}

func (s *roleService) Update(ctx context.Context, id, name, displayName, description string) (*model.Role, error) {
	if name == "" && displayName == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "名称不能为空")
	}
	roles, err := s.repo.List(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "无法获取角色列表")
	}
	var target *model.Role
	for i := range roles {
		if roles[i].ID == id {
			target = &roles[i]
			break
		}
	}
	if target == nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "角色不存在")
	}
	if err := s.repo.Update(ctx, id, name, displayName, description); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.reloadCache(ctx)
	updatedName := target.Name
	if name != "" {
		updatedName = name
	}
	updatedDisplay := target.DisplayName
	if displayName != "" {
		updatedDisplay = displayName
	}
	return &model.Role{ID: id, Name: updatedName, DisplayName: updatedDisplay, Description: description}, nil
}

func (s *roleService) reloadCache(ctx context.Context) {
	if roles, err := s.repo.List(ctx); err == nil {
		model.LoadRoleCache(roles)
	}
}
