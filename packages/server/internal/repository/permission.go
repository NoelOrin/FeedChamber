package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"gorm.io/gorm"
)

type PermissionRepository interface {
	List(ctx context.Context) ([]model.Permission, error)
	GetByCode(ctx context.Context, code string) (*model.Permission, error)
	CreateIfNotExists(ctx context.Context, perm *model.Permission) error
	GetRolePermissions(ctx context.Context, roleName string) ([]string, error)
	GetAllRolePermissions(ctx context.Context) (map[string][]string, error)
	SyncRolePermissions(ctx context.Context, roleName string, permCodes []string) error
}

func NewPermissionRepository(store db.Store) PermissionRepository {
	if store.SQL() != nil {
		return &sqlPermRepo{db: store.SQL()}
	}
	return &mongoPermRepo{store: store.Mongo()}
}

// ── SQL ─────────────────────────────────────────────────────────────────────

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

// ── Mongo ───────────────────────────────────────────────────────────────────

type mongoPermRepo struct{ store db.MongoStore }

func (r *mongoPermRepo) List(ctx context.Context) ([]model.Permission, error) {
	cursor, err := r.store.Collection("permissions").Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var perms []model.Permission
	err = cursor.All(ctx, &perms)
	return perms, err
}

func (r *mongoPermRepo) GetByCode(ctx context.Context, code string) (*model.Permission, error) {
	var perm model.Permission
	err := r.store.Collection("permissions").FindOne(ctx, bson.M{"code": code}).Decode(&perm)
	return &perm, err
}

func (r *mongoPermRepo) CreateIfNotExists(ctx context.Context, perm *model.Permission) error {
	existing, _ := r.GetByCode(ctx, perm.Code)
	if existing != nil {
		return nil
	}
	_, err := r.store.Collection("permissions").InsertOne(ctx, perm)
	return err
}

func (r *mongoPermRepo) GetRolePermissions(ctx context.Context, roleName string) ([]string, error) {
	cursor, err := r.store.Collection("role_permissions").Find(ctx, bson.M{"role_name": roleName})
	if err != nil {
		return nil, err
	}
	var rps []model.RolePermission
	if err = cursor.All(ctx, &rps); err != nil {
		return nil, err
	}

	var codes []string
	for _, rp := range rps {
		var perm model.Permission
		err := r.store.Collection("permissions").FindOne(ctx, bson.M{"_id": rp.PermissionID}).Decode(&perm)
		if err == nil {
			codes = append(codes, perm.Code)
		}
	}
	return codes, nil
}

func (r *mongoPermRepo) GetAllRolePermissions(ctx context.Context) (map[string][]string, error) {
	cursor, err := r.store.Collection("role_permissions").Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var rps []model.RolePermission
	if err = cursor.All(ctx, &rps); err != nil {
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

func (r *mongoPermRepo) SyncRolePermissions(ctx context.Context, roleName string, permCodes []string) error {
	// 删除旧权限
	_, err := r.store.Collection("role_permissions").DeleteMany(ctx, bson.M{"role_name": roleName})
	if err != nil {
		return err
	}
	// 插入新权限
	for _, code := range permCodes {
		perm, err := r.GetByCode(ctx, code)
		if err != nil {
			continue
		}
		rp := model.RolePermission{RoleName: roleName, PermissionID: perm.ID}
		_, _ = r.store.Collection("role_permissions").InsertOne(ctx, rp)
	}
	return nil
}
