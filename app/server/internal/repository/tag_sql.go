package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"

	"github.com/sirupsen/logrus"
)

type sqlTagRepo struct{ db db.SQLStore }

func (r *sqlTagRepo) List(ctx context.Context, offset, limit int) ([]model.Tag, error) {
	var tags []model.Tag
	err := r.db.DB().WithContext(ctx).Order("name asc").Offset(offset).Limit(limit).Find(&tags).Error
	return tags, err
}

func (r *sqlTagRepo) ListWithTotal(ctx context.Context, offset, limit int) ([]model.Tag, int, error) {
	type row struct {
		model.Tag
		TotalCount int64
	}
	var rows []row
	err := r.db.DB().WithContext(ctx).Raw(`
		SELECT t.id, t.name, t.created_at,
		       COUNT(*) OVER() AS total_count
		FROM tags t
		WHERE `+SQLSoftDeleteCondition("t")+`
		ORDER BY t.name ASC
		LIMIT ? OFFSET ?
	`, limit, offset).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	tags := make([]model.Tag, len(rows))
	for i, r := range rows {
		tags[i] = r.Tag
	}
	if len(rows) == 0 {
		return tags, 0, nil
	}
	return tags, int(rows[0].TotalCount), nil
}

func (r *sqlTagRepo) Count(ctx context.Context) (int, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.Tag{}).Count(&count).Error
	return int(count), err
}

func (r *sqlTagRepo) GetByName(ctx context.Context, name string) (*model.Tag, error) {
	var t model.Tag
	return &t, WrapDBErr(r.db.DB().WithContext(ctx).First(&t, "name = ?", name).Error)
}

func (r *sqlTagRepo) FindByID(ctx context.Context, id string) (*model.Tag, error) {
	var t model.Tag
	return &t, WrapDBErr(r.db.DB().WithContext(ctx).First(&t, "id = ?", id).Error)
}

func (r *sqlTagRepo) Create(ctx context.Context, tag *model.Tag) error {
	if err := r.db.DB().WithContext(ctx).Unscoped().Where("name = ? AND deleted_at IS NOT NULL", tag.Name).Delete(&model.Tag{}).Error; err != nil {
		logrus.Warnf("tag: cleanup soft-deleted record failed (name=%s): %v", tag.Name, err)
	}
	return r.db.DB().WithContext(ctx).Create(tag).Error
}

func (r *sqlTagRepo) CreateIfNotExists(ctx context.Context, tag *model.Tag) error {
	var existing model.Tag
	err := r.db.DB().WithContext(ctx).First(&existing, "name = ?", tag.Name).Error
	if err == nil {
		*tag = existing
		return nil
	}
	return r.db.DB().WithContext(ctx).Create(tag).Error
}

func (r *sqlTagRepo) Update(ctx context.Context, tag *model.Tag) error {
	if err := r.db.DB().WithContext(ctx).Unscoped().Where("name = ? AND id != ? AND deleted_at IS NOT NULL", tag.Name, tag.ID).Delete(&model.Tag{}).Error; err != nil {
		logrus.Warnf("tag: cleanup soft-deleted record failed (name=%s, id=%s): %v", tag.Name, tag.ID, err)
	}
	return r.db.DB().WithContext(ctx).Where(SQLSoftDeleteCondition()).Save(tag).Error
}

func (r *sqlTagRepo) Delete(ctx context.Context, id string) error {
	if err := r.db.DB().WithContext(ctx).Where("tag_id = ?", id).Delete(&model.PostTag{}).Error; err != nil {
		return err
	}
	return r.db.DB().WithContext(ctx).Delete(&model.Tag{}, "id = ?", id).Error
}

func (r *sqlTagRepo) AddPostTag(ctx context.Context, postID, tagID string) error {
	pt := &model.PostTag{PostID: postID, TagID: tagID}
	return r.db.DB().WithContext(ctx).Create(pt).Error
}

func (r *sqlTagRepo) RemovePostTags(ctx context.Context, postID string) error {
	return r.db.DB().WithContext(ctx).Where("post_id = ?", postID).Delete(&model.PostTag{}).Error
}

func (r *sqlTagRepo) GetTagsByPostID(ctx context.Context, postID string) ([]model.Tag, error) {
	var tags []model.Tag
	err := r.db.DB().WithContext(ctx).
		Joins("JOIN post_tags ON post_tags.tag_id = tags.id").
		Where("post_tags.post_id = ?", postID).
		Order("tags.name asc").
		Find(&tags).Error
	return tags, err
}

func (r *sqlTagRepo) GetPostsByTagID(ctx context.Context, tagID string, offset, limit int) ([]string, error) {
	var postIDs []string
	err := r.db.DB().WithContext(ctx).
		Model(&model.PostTag{}).
		Where("tag_id = ?", tagID).
		Offset(offset).Limit(limit).
		Pluck("post_id", &postIDs).Error
	return postIDs, err
}
