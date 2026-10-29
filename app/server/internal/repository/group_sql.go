package repository

import (
	"context"
	"errors"
	"feedchamber/internal/db"
	"feedchamber/internal/model"

	"gorm.io/gorm"
)

type sqlGroupRepo struct{ db db.SQLStore }

func (r *sqlGroupRepo) List(ctx context.Context) ([]model.Group, error) {
	var groups []model.Group
	err := r.db.DB().WithContext(ctx).Order("created_at asc").Find(&groups).Error
	return groups, err
}

func (r *sqlGroupRepo) FindByID(ctx context.Context, id string) (*model.Group, error) {
	var g model.Group
	return &g, WrapDBErr(r.db.DB().WithContext(ctx).Where("id = ?", id).First(&g).Error)
}

func (r *sqlGroupRepo) FindByName(ctx context.Context, name string) (*model.Group, error) {
	var g model.Group
	return &g, WrapDBErr(r.db.DB().WithContext(ctx).Where("name = ?", name).First(&g).Error)
}

func (r *sqlGroupRepo) Create(ctx context.Context, group *model.Group) error {
	return r.db.DB().WithContext(ctx).Create(group).Error
}

func (r *sqlGroupRepo) CreateIfNotExistsByName(ctx context.Context, group *model.Group) error {
	var existing model.Group
	err := WrapDBErr(r.db.DB().WithContext(ctx).Where("name = ?", group.Name).First(&existing).Error)
	if err == nil {
		*group = existing
		return nil
	}
	if !errors.Is(err, ErrNotFound) {
		return err
	}
	return r.db.DB().WithContext(ctx).Create(group).Error
}

func (r *sqlGroupRepo) Update(ctx context.Context, group *model.Group) error {
	return r.db.DB().WithContext(ctx).Save(group).Error
}

func (r *sqlGroupRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Where("group_id = ?", id).Delete(&model.GroupMember{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("group_id = ?", id).Delete(&model.GroupPermission{}).Error; err != nil {
			return err
		}
		return tx.WithContext(ctx).Delete(&model.Group{}, "id = ?", id).Error
	})
}

func (r *sqlGroupRepo) ListMembers(ctx context.Context, groupID string, offset, limit int) ([]model.GroupMember, error) {
	var members []model.GroupMember
	q := r.db.DB().WithContext(ctx).Where("group_id = ?", groupID).Order("joined_at asc")
	if limit > 0 {
		q = q.Offset(offset).Limit(limit)
	}
	err := q.Find(&members).Error
	return members, err
}

func (r *sqlGroupRepo) CountMembers(ctx context.Context, groupID string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.GroupMember{}).Where("group_id = ?", groupID).Count(&count).Error
	return count, err
}

func (r *sqlGroupRepo) AddMember(ctx context.Context, member *model.GroupMember) error {
	var existing model.GroupMember
	err := WrapDBErr(r.db.DB().WithContext(ctx).
		Where("group_id = ? AND user_id = ?", member.GroupID, member.UserID).
		First(&existing).Error)
	if err == nil {
		// update role if already member
		if member.Role != "" && member.Role != existing.Role {
			return r.db.DB().WithContext(ctx).
				Model(&model.GroupMember{}).
				Where("group_id = ? AND user_id = ?", member.GroupID, member.UserID).
				Update("role", member.Role).Error
		}
		return nil
	}
	if !errors.Is(err, ErrNotFound) {
		return err
	}
	return r.db.DB().WithContext(ctx).Create(member).Error
}

func (r *sqlGroupRepo) RemoveMember(ctx context.Context, groupID, userID string) error {
	return r.db.DB().WithContext(ctx).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		Delete(&model.GroupMember{}).Error
}

func (r *sqlGroupRepo) IsMember(ctx context.Context, groupID, userID string) (bool, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.GroupMember{}).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *sqlGroupRepo) ListGroupIDsByUser(ctx context.Context, userID string) ([]string, error) {
	var ids []string
	err := r.db.DB().WithContext(ctx).Model(&model.GroupMember{}).
		Where("user_id = ?", userID).
		Pluck("group_id", &ids).Error
	return ids, err
}

func (r *sqlGroupRepo) RemoveUserFromSystemGroups(ctx context.Context, userID string) error {
	return r.db.DB().WithContext(ctx).
		Where("user_id = ? AND group_id IN (?)", userID,
			r.db.DB().Model(&model.Group{}).Select("id").Where("is_system = ?", true),
		).
		Delete(&model.GroupMember{}).Error
}

func (r *sqlGroupRepo) GetGroupPermissions(ctx context.Context, groupID string) ([]string, error) {
	var codes []string
	err := r.db.DB().WithContext(ctx).
		Model(&model.GroupPermission{}).
		Select("permissions.code").
		Joins("JOIN permissions ON permissions.id = group_permissions.permission_id").
		Where("group_permissions.group_id = ?", groupID).
		Pluck("code", &codes).Error
	return codes, err
}

func (r *sqlGroupRepo) GetAllGroupPermissions(ctx context.Context) (map[string][]string, error) {
	var gps []model.GroupPermission
	if err := r.db.DB().WithContext(ctx).Find(&gps).Error; err != nil {
		return nil, err
	}
	var perms []model.Permission
	if err := r.db.DB().WithContext(ctx).Find(&perms).Error; err != nil {
		return nil, err
	}
	idToCode := make(map[uint]string, len(perms))
	for _, p := range perms {
		idToCode[p.ID] = p.Code
	}
	result := make(map[string][]string)
	for _, gp := range gps {
		if code, ok := idToCode[gp.PermissionID]; ok {
			result[gp.GroupID] = append(result[gp.GroupID], code)
		}
	}
	return result, nil
}

func (r *sqlGroupRepo) SyncGroupPermissions(ctx context.Context, groupID string, permCodes []string) error {
	return r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Where("group_id = ?", groupID).Delete(&model.GroupPermission{}).Error; err != nil {
			return err
		}
		for _, code := range permCodes {
			var perm model.Permission
			if err := tx.WithContext(ctx).Where("code = ?", code).First(&perm).Error; err != nil {
				continue
			}
			gp := model.GroupPermission{GroupID: groupID, PermissionID: perm.ID}
			if err := tx.WithContext(ctx).Create(&gp).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *sqlGroupRepo) DeleteGroupPermissions(ctx context.Context, groupID string) error {
	return r.db.DB().WithContext(ctx).Where("group_id = ?", groupID).Delete(&model.GroupPermission{}).Error
}
