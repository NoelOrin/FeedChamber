package repository

import (
	"context"
	"fmt"
	"orinforum/internal/db"
	"orinforum/internal/model"

	"gorm.io/gorm"
)

type sqlSpaceRolePermRepo struct{ db db.SQLStore }

func (r *sqlSpaceRolePermRepo) GetSpaceRolePermissions(ctx context.Context, spaceID, roleName string) ([]string, bool, error) {
	var codes []string
	err := r.db.DB().WithContext(ctx).
		Model(&model.SpaceRolePermission{}).
		Select("permissions.code").
		Joins("JOIN permissions ON permissions.id = space_role_permissions.permission_id").
		Where("space_role_permissions.space_id = ? AND space_role_permissions.role_name = ?", spaceID, roleName).
		Pluck("code", &codes).Error
	if err != nil {
		return nil, false, err
	}
	return codes, len(codes) > 0, nil
}

func (r *sqlSpaceRolePermRepo) GetAllSpaceRolePermissions(ctx context.Context) (map[string]map[string][]string, error) {
	var srps []model.SpaceRolePermission
	if err := r.db.DB().WithContext(ctx).Find(&srps).Error; err != nil {
		return nil, err
	}
	if len(srps) == 0 {
		return map[string]map[string][]string{}, nil
	}

	permIDs := make([]uint, 0, len(srps))
	for _, srp := range srps {
		permIDs = append(permIDs, srp.PermissionID)
	}

	var perms []model.Permission
	if err := r.db.DB().WithContext(ctx).Where("id IN ?", permIDs).Find(&perms).Error; err != nil {
		return nil, err
	}
	idToCode := make(map[uint]string, len(perms))
	for _, p := range perms {
		idToCode[p.ID] = p.Code
	}

	result := make(map[string]map[string][]string)
	for _, srp := range srps {
		if code, ok := idToCode[srp.PermissionID]; ok {
			if result[srp.SpaceID] == nil {
				result[srp.SpaceID] = make(map[string][]string)
			}
			result[srp.SpaceID][srp.RoleName] = append(result[srp.SpaceID][srp.RoleName], code)
		}
	}
	return result, nil
}

func (r *sqlSpaceRolePermRepo) SyncSpaceRolePermissions(ctx context.Context, spaceID, roleName string, permCodes []string) error {
	return r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Where("space_id = ? AND role_name = ?", spaceID, roleName).Delete(&model.SpaceRolePermission{}).Error; err != nil {
			return err
		}
		written := 0
		for _, code := range permCodes {
			var perm model.Permission
			if err := tx.WithContext(ctx).Where("code = ?", code).First(&perm).Error; err != nil {
				continue
			}
			srp := model.SpaceRolePermission{SpaceID: spaceID, RoleName: roleName, PermissionID: perm.ID}
			if err := tx.WithContext(ctx).Create(&srp).Error; err != nil {
				return err
			}
			written++
		}
		if written < len(permCodes) {
			return fmt.Errorf("space permission sync: %d/%d codes resolved (unknown codes present)", written, len(permCodes))
		}
		return nil
	})
}

func (r *sqlSpaceRolePermRepo) ClearSpaceRolePermissions(ctx context.Context, spaceID, roleName string) error {
	return r.db.DB().WithContext(ctx).
		Where("space_id = ? AND role_name = ?", spaceID, roleName).
		Delete(&model.SpaceRolePermission{}).Error
}
