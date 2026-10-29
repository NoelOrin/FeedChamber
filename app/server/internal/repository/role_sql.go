package repository

import (
	"context"
	"errors"
	"orinforum/internal/db"
	"orinforum/internal/model"

	"gorm.io/gorm"
)

// ── SQL ─────────────────────────────────────────────────────────────────────

type sqlRoleRepo struct{ db db.SQLStore }

func (r *sqlRoleRepo) List(ctx context.Context) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.DB().WithContext(ctx).Find(&roles).Error
	return roles, err
}

func (r *sqlRoleRepo) GetByName(ctx context.Context, name string) (*model.Role, error) {
	var role model.Role
	err := r.db.DB().WithContext(ctx).Where("name = ?", name).First(&role).Error
	return &role, err
}

func (r *sqlRoleRepo) Create(ctx context.Context, role *model.Role) error {
	return r.db.DB().WithContext(ctx).Create(role).Error
}

func (r *sqlRoleRepo) CreateIfNotExists(ctx context.Context, role *model.Role) error {
	var existing model.Role
	err := WrapDBErr(r.db.DB().WithContext(ctx).Where("name = ?", role.Name).First(&existing).Error)
	if err == nil {
		*role = existing
		return nil
	}
	if !errors.Is(err, ErrNotFound) {
		return err
	}
	return r.db.DB().WithContext(ctx).Create(role).Error
}

func (r *sqlRoleRepo) Delete(ctx context.Context, id string) error {
	result := r.db.DB().WithContext(ctx).Delete(&model.Role{}, "id = ?", id)
	return result.Error
}

func (r *sqlRoleRepo) HasUsersWithRole(ctx context.Context, roleName string) (bool, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).
		Model(&model.User{}).
		Where("role = ?", roleName).
		Count(&count).Error
	return count > 0, err
}

func (r *sqlRoleRepo) DeleteRolePermissions(ctx context.Context, roleName string) error {
	return r.db.DB().WithContext(ctx).
		Where("role_name = ?", roleName).
		Delete(&model.RolePermission{}).Error
}

func (r *sqlRoleRepo) Update(ctx context.Context, id string, newName string, displayName string, description string) error {
	var old model.Role
	if err := r.db.DB().WithContext(ctx).First(&old, "id = ?", id).Error; err != nil {
		return err
	}
	return r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{"display_name": displayName, "description": description}
		if newName != "" && newName != old.Name {
			updates["name"] = newName
		}
		if err := tx.Model(&model.Role{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		if newName != "" && newName != old.Name {
			if err := tx.Model(&model.RolePermission{}).Where("role_name = ?", old.Name).Update("role_name", newName).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.SpaceRolePermission{}).Where("role_name = ?", old.Name).Update("role_name", newName).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
