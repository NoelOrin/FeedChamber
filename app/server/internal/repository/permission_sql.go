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

// EnsureRolePermissions 只追加缺失的角色-权限映射，不删除任何已有映射。
//
// 与 SyncRolePermissions 的区别是本方法**幂等且非破坏性**：Sync 是
// 「删光再插入」，用于首次 seed；Ensure 用于后续版本新增权限码时补齐
// 内置角色，不会覆盖管理员对角色权限的自定义。
//
// 为什么需要它：R-04 的幂等 seed 在首启之后整个跳过角色映射同步，
// 于是「新版本新增了一个权限码」永远到不了已有部署的内置角色——
// 表现为功能上线后管理员看不到入口，且没有任何报错。
func (r *sqlPermRepo) EnsureRolePermissions(ctx context.Context, roleName string, permCodes []string) (int, error) {
	added := 0
	err := r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先取该角色现有的权限 id 集合。
		var existing []model.RolePermission
		if err := tx.WithContext(ctx).Where("role_name = ?", roleName).Find(&existing).Error; err != nil {
			return err
		}
		have := make(map[uint]struct{}, len(existing))
		for _, rp := range existing {
			have[rp.PermissionID] = struct{}{}
		}
		for _, code := range permCodes {
			var perm model.Permission
			if err := tx.WithContext(ctx).Where("code = ?", code).First(&perm).Error; err != nil {
				continue
			}
			if _, ok := have[perm.ID]; ok {
				continue
			}
			rp := model.RolePermission{RoleName: roleName, PermissionID: perm.ID}
			if err := tx.WithContext(ctx).Create(&rp).Error; err != nil {
				return err
			}
			added++
		}
		return nil
	})
	return added, err
}
