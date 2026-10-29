package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type sqlSpaceRepo struct{ db db.SQLStore }

func (r *sqlSpaceRepo) List(ctx context.Context) ([]model.Space, error) {
	var spaces []model.Space
	err := r.db.DB().WithContext(ctx).Order("sort_order asc, name asc").Find(&spaces).Error
	return spaces, WrapDBErr(err)
}

func (r *sqlSpaceRepo) FindByID(ctx context.Context, id string) (*model.Space, error) {
	var s model.Space
	err := r.db.DB().WithContext(ctx).First(&s, "id = ?", id).Error
	return &s, WrapDBErr(err)
}

func (r *sqlSpaceRepo) FindBySlug(ctx context.Context, slug string) (*model.Space, error) {
	var s model.Space
	err := r.db.DB().WithContext(ctx).Where("slug = ?", slug).First(&s).Error
	return &s, WrapDBErr(err)
}

func (r *sqlSpaceRepo) Create(ctx context.Context, s *model.Space) error {
	if err := r.db.DB().WithContext(ctx).Unscoped().Where("name = ? AND deleted_at IS NOT NULL", s.Name).Delete(&model.Space{}).Error; err != nil {
		logrus.Warnf("space: cleanup soft-deleted record failed (name=%s): %v", s.Name, err)
	}
	if err := r.db.DB().WithContext(ctx).Unscoped().Where("slug = ? AND deleted_at IS NOT NULL", s.Slug).Delete(&model.Space{}).Error; err != nil {
		logrus.Warnf("space: cleanup soft-deleted record failed (slug=%s): %v", s.Slug, err)
	}
	return r.db.DB().WithContext(ctx).Create(s).Error
}

func (r *sqlSpaceRepo) Update(ctx context.Context, s *model.Space) error {
	if err := r.db.DB().WithContext(ctx).Unscoped().Where("name = ? AND id != ? AND deleted_at IS NOT NULL", s.Name, s.ID).Delete(&model.Space{}).Error; err != nil {
		logrus.Warnf("space: cleanup soft-deleted record failed (name=%s, id=%s): %v", s.Name, s.ID, err)
	}
	if err := r.db.DB().WithContext(ctx).Unscoped().Where("slug = ? AND id != ? AND deleted_at IS NOT NULL", s.Slug, s.ID).Delete(&model.Space{}).Error; err != nil {
		logrus.Warnf("space: cleanup soft-deleted record failed (slug=%s, id=%s): %v", s.Slug, s.ID, err)
	}
	return r.db.DB().WithContext(ctx).Where(SQLSoftDeleteCondition()).Save(s).Error
}

func (r *sqlSpaceRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Space{}, "id = ?", id).Error
}

func (r *sqlSpaceRepo) ToggleActive(ctx context.Context, id string) (*model.Space, error) {
	if err := r.db.DB().WithContext(ctx).Model(&model.Space{}).Where("id = ?", id).UpdateColumn("is_active", gorm.Expr("NOT is_active")).Error; err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *sqlSpaceRepo) SetSystemFlags(ctx context.Context, id string, system, recommendable bool) error {
	return r.db.DB().WithContext(ctx).Model(&model.Space{}).Where("id = ?", id).Updates(map[string]any{
		"system":        system,
		"recommendable": recommendable,
	}).Error
}

// BackfillRecommendable 把"系统版面以外且 recommendable=false"的存量行置 true。
// 只在迁移时调用一次（由 meta 标记门控），之后管理员可以自由改为 false。
func (r *sqlSpaceRepo) BackfillRecommendable(ctx context.Context) error {
	return r.db.DB().WithContext(ctx).Model(&model.Space{}).
		Where("system = ? AND recommendable = ?", false, false).
		Update("recommendable", true).Error
}

func (r *sqlSpaceRepo) FindChildren(ctx context.Context, parentID string) ([]model.Space, error) {
	var spaces []model.Space
	err := r.db.DB().WithContext(ctx).Where("parent_id = ?", parentID).Order("sort_order asc, name asc").Find(&spaces).Error
	return spaces, WrapDBErr(err)
}

func (r *sqlSpaceRepo) ListRoots(ctx context.Context) ([]model.Space, error) {
	var spaces []model.Space
	err := r.db.DB().WithContext(ctx).Where("parent_id = ?", "").Order("sort_order asc, name asc").Find(&spaces).Error
	return spaces, WrapDBErr(err)
}

func (r *sqlSpaceRepo) HasChildren(ctx context.Context, id string) (bool, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.Space{}).Where("parent_id = ? AND deleted_at IS NULL", id).Count(&count).Error
	return count > 0, err
}

func (r *sqlSpaceRepo) CountChildren(ctx context.Context, parentID string) (int, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.Space{}).Where("parent_id = ? AND deleted_at IS NULL", parentID).Count(&count).Error
	return int(count), err
}

func (r *sqlSpaceRepo) Exists(ctx context.Context, id string) (bool, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.Space{}).Where("id = ? AND deleted_at IS NULL", id).Count(&count).Error
	return count > 0, err
}

func (r *sqlSpaceRepo) FindAncestors(ctx context.Context, id string) ([]model.Space, error) {
	// fast path via path column if available
	target, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if target.Path != "" {
		trimmed := strings.Trim(target.Path, "/")
		parts := strings.Split(trimmed, "/")
		if len(parts) > 1 {
			ancIDs := parts[:len(parts)-1]
			if len(ancIDs) > 0 {
				var ancestors []model.Space
				if err := r.db.DB().WithContext(ctx).Where("id IN ? AND deleted_at IS NULL", ancIDs).Order("depth asc").Find(&ancestors).Error; err != nil {
					return nil, WrapDBErr(err)
				}
				// ensure order by depth (path order)
				byID := make(map[string]model.Space, len(ancestors))
				for _, a := range ancestors {
					byID[a.ID] = a
				}
				ordered := make([]model.Space, 0, len(ancIDs))
				for _, aid := range ancIDs {
					if sp, ok := byID[aid]; ok {
						ordered = append(ordered, sp)
					}
				}
				return ordered, nil
			}
		}
		return []model.Space{}, nil
	}
	// fallback: in-memory traversal
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]model.Space, len(all))
	for _, s := range all {
		byID[s.ID] = s
	}
	ancIDs := model.CollectAncestorIDs(all, id)
	ancestors := make([]model.Space, 0, len(ancIDs))
	for _, aid := range ancIDs {
		if sp, ok := byID[aid]; ok {
			ancestors = append(ancestors, sp)
		}
	}
	return ancestors, nil
}

func (r *sqlSpaceRepo) FindDescendants(ctx context.Context, id string) ([]model.Space, error) {
	target, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if target.Path != "" {
		like := target.Path + "/%"
		var spaces []model.Space
		if err := r.db.DB().WithContext(ctx).Where("path LIKE ? AND deleted_at IS NULL", like).Order("path asc").Find(&spaces).Error; err != nil {
			return nil, WrapDBErr(err)
		}
		return spaces, nil
	}
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	descIDs := model.CollectDescendantIDs(all, id)
	if len(descIDs) == 0 {
		return []model.Space{}, nil
	}
	byID := make(map[string]model.Space, len(all))
	for _, s := range all {
		byID[s.ID] = s
	}
	descs := make([]model.Space, 0, len(descIDs))
	for _, did := range descIDs {
		if sp, ok := byID[did]; ok {
			descs = append(descs, sp)
		}
	}
	return descs, nil
}

func (r *sqlSpaceRepo) FindDescendantIDs(ctx context.Context, id string) ([]string, error) {
	target, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if target.Path != "" {
		like := target.Path + "/%"
		var ids []string
		if err := r.db.DB().WithContext(ctx).Model(&model.Space{}).Where("path LIKE ? AND deleted_at IS NULL", like).Pluck("id", &ids).Error; err != nil {
			return nil, WrapDBErr(err)
		}
		return ids, nil
	}
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	return model.CollectDescendantIDs(all, id), nil
}

func (r *sqlSpaceRepo) BatchUpdateSortOrder(ctx context.Context, orderedIDs []string) error {
	return r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range orderedIDs {
			if err := tx.Model(&model.Space{}).Where("id = ?", id).UpdateColumn("sort_order", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *sqlSpaceRepo) BatchUpdateDepthPath(ctx context.Context, updates map[string]struct {
	Depth int
	Path  string
}) error {
	if len(updates) == 0 {
		return nil
	}
	return r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for id, v := range updates {
			if err := tx.Model(&model.Space{}).Where("id = ?", id).Updates(map[string]interface{}{"depth": v.Depth, "path": v.Path}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
