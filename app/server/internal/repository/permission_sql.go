package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"

	"gorm.io/gorm"
)

type sqlPermRepo struct{ db db.SQLStore }

func (r *sqlPermRepo) List(ctx context.Context) ([]model.Permission, error) {
	var perms []model.Permission
	err := r.db.DB().WithContext(ctx).Find(&perms).Error
	return perms, err
}

func (r *sqlPermRepo) GetByCode(ctx context.Context, code string) (*model.Permission, error) {
	var perm model.Permission
	err := r.db.DB().WithContext(ctx).Where("code = ?", code).First(&perm).Error
	return &perm, err
}

func (r *sqlPermRepo) CreateIfNotExists(ctx context.Context, perm *model.Permission) error {
	return r.db.DB().WithContext(ctx).Where("code = ?", perm.Code).FirstOrCreate(perm).Error
}

func (r *sqlPermRepo) GetRolePermissions(ctx context.Context, roleName string) ([]string, error) {
	var codes []string
	err := r.db.DB().WithContext(ctx).
		Model(&model.RolePermission{}).
		Select("permissions.code").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Where("role_permissions.role_name = ?", roleName).
		Pluck("code", &codes).Error
	return codes, err
}

func (r *sqlPermRepo) GetAllRolePermissions(ctx context.Context) (map[string][]string, error) {
	var rps []model.RolePermission
	if err := r.db.DB().WithContext(ctx).Find(&rps).Error; err != nil {
		return nil, err
	}

	perms, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	idToCode := make(map[uint]string, len(perms))
	for _, p := range perms {
		idToCode[p.ID] = p.Code
	}

	result := make(map[string][]string)
	for _, rp := range rps {
		if code, ok := idToCode[rp.PermissionID]; ok {
			result[rp.RoleName] = append(result[rp.RoleName], code)
		}
	}
	return result, nil
}

func (r *sqlPermRepo) SyncRolePermissions(ctx context.Context, roleName string, permCodes []string) error {
	return r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Where("role_name = ?", roleName).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
		for _, code := range permCodes {
			var perm model.Permission
			if err := tx.WithContext(ctx).Where("code = ?", code).First(&perm).Error; err != nil {
				continue
			}
			rp := model.RolePermission{RoleName: roleName, PermissionID: perm.ID}
			if err := tx.WithContext(ctx).Create(&rp).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
