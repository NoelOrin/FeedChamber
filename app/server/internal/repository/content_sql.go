package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"time"

	"gorm.io/gorm"
)

type sqlContentRepo struct{ db db.SQLStore }

func (r *sqlContentRepo) ListByRoot(ctx context.Context, rootID string, offset, limit int, startTime, endTime *time.Time) ([]model.ContentListItem, error) {
	var items []model.ContentListItem
	q := `SELECT c.id, c.author_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS author_name, u.avatar AS author_avatar,
	      c.body, c.root_id, c.parent_id, c.like_count, c.created_at, c.updated_at,
	      p.id AS post_id, p.title AS post_title
	      FROM contents c
	      LEFT JOIN users u ON c.author_id = u.id
	      LEFT JOIN posts p ON p.content_id = c.root_id AND ` + SQLSoftDeleteCondition("p") + `
	      WHERE c.root_id = ? AND c.id != c.root_id AND c.status = 'approved' AND ` + SQLSoftDeleteCondition("c")
	args := []interface{}{rootID}
	if startTime != nil {
		q += " AND c.updated_at >= ?"
		args = append(args, *startTime)
	}
	if endTime != nil {
		q += " AND c.updated_at <= ?"
		args = append(args, *endTime)
	}
	q += " ORDER BY c.created_at ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	err := r.db.DB().WithContext(ctx).Raw(q, args...).Scan(&items).Error
	return items, err
}

func (r *sqlContentRepo) ListByRootWithTotal(ctx context.Context, rootID string, offset, limit int, startTime, endTime *time.Time) ([]model.ContentListItem, int, error) {
	type row struct {
		model.ContentListItem
		TotalCount int64
	}
	q := `SELECT c.id, c.author_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS author_name, u.avatar AS author_avatar,
	      c.body, c.root_id, c.parent_id, c.like_count, c.created_at, c.updated_at,
	      p.id AS post_id, p.title AS post_title,
	      COUNT(*) OVER() AS total_count
	      FROM contents c
	      LEFT JOIN users u ON c.author_id = u.id
	      LEFT JOIN posts p ON p.content_id = c.root_id AND ` + SQLSoftDeleteCondition("p") + `
	      WHERE c.root_id = ? AND c.id != c.root_id AND c.status = 'approved' AND ` + SQLSoftDeleteCondition("c")
	args := []interface{}{rootID}
	if startTime != nil {
		q += " AND c.updated_at >= ?"
		args = append(args, *startTime)
	}
	if endTime != nil {
		q += " AND c.updated_at <= ?"
		args = append(args, *endTime)
	}
	q += " ORDER BY c.created_at ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	var rows []row
	err := r.db.DB().WithContext(ctx).Raw(q, args...).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	items := make([]model.ContentListItem, len(rows))
	for i, r := range rows {
		items[i] = r.ContentListItem
	}
	if len(rows) == 0 {
		return items, 0, nil
	}
	return items, int(rows[0].TotalCount), nil
}

func (r *sqlContentRepo) ListByParent(ctx context.Context, parentID string, offset, limit int, startTime, endTime *time.Time) ([]model.ContentListItem, error) {
	var items []model.ContentListItem
	q := `SELECT c.id, c.author_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS author_name, u.avatar AS author_avatar,
	      c.body, c.root_id, c.parent_id, c.like_count, c.created_at, c.updated_at,
	      p.id AS post_id, p.title AS post_title
	      FROM contents c
	      LEFT JOIN users u ON c.author_id = u.id
	      LEFT JOIN posts p ON p.content_id = c.root_id AND ` + SQLSoftDeleteCondition("p") + `
	      WHERE c.parent_id = ? AND c.status = 'approved' AND ` + SQLSoftDeleteCondition("c")
	args := []interface{}{parentID}
	if startTime != nil {
		q += " AND c.updated_at >= ?"
		args = append(args, *startTime)
	}
	if endTime != nil {
		q += " AND c.updated_at <= ?"
		args = append(args, *endTime)
	}
	q += " ORDER BY c.created_at ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	err := r.db.DB().WithContext(ctx).Raw(q, args...).Scan(&items).Error
	return items, err
}

func (r *sqlContentRepo) ListByParentWithTotal(ctx context.Context, parentID string, offset, limit int, startTime, endTime *time.Time) ([]model.ContentListItem, int, error) {
	type row struct {
		model.ContentListItem
		TotalCount int64
	}
	q := `SELECT c.id, c.author_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS author_name, u.avatar AS author_avatar,
	      c.body, c.root_id, c.parent_id, c.like_count, c.created_at, c.updated_at,
	      p.id AS post_id, p.title AS post_title,
	      COUNT(*) OVER() AS total_count
	      FROM contents c
	      LEFT JOIN users u ON c.author_id = u.id
	      LEFT JOIN posts p ON p.content_id = c.root_id AND ` + SQLSoftDeleteCondition("p") + `
	      WHERE c.parent_id = ? AND c.status = 'approved' AND ` + SQLSoftDeleteCondition("c")
	args := []interface{}{parentID}
	if startTime != nil {
		q += " AND c.updated_at >= ?"
		args = append(args, *startTime)
	}
	if endTime != nil {
		q += " AND c.updated_at <= ?"
		args = append(args, *endTime)
	}
	q += " ORDER BY c.created_at ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	var rows []row
	err := r.db.DB().WithContext(ctx).Raw(q, args...).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	items := make([]model.ContentListItem, len(rows))
	for i, r := range rows {
		items[i] = r.ContentListItem
	}
	if len(rows) == 0 {
		return items, 0, nil
	}
	return items, int(rows[0].TotalCount), nil
}

func (r *sqlContentRepo) CountByRoot(ctx context.Context, rootID string, startTime, endTime *time.Time) (int, error) {
	q := `SELECT COUNT(*) FROM contents c
	      WHERE c.root_id = ? AND c.id != c.root_id AND c.status = 'approved' AND ` + SQLSoftDeleteCondition("c")
	args := []interface{}{rootID}
	if startTime != nil {
		q += " AND c.updated_at >= ?"
		args = append(args, *startTime)
	}
	if endTime != nil {
		q += " AND c.updated_at <= ?"
		args = append(args, *endTime)
	}
	var count int64
	err := r.db.DB().WithContext(ctx).Raw(q, args...).Scan(&count).Error
	return int(count), err
}

func (r *sqlContentRepo) CountUnreadByRoot(ctx context.Context, rootID string, after time.Time) (int, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM contents c
		WHERE c.root_id = ? AND c.id != c.root_id AND c.status = 'approved'
		  AND c.created_at > ? AND `+SQLSoftDeleteCondition("c"),
		rootID, after).Scan(&count).Error
	return int(count), err
}

func (r *sqlContentRepo) CountByParent(ctx context.Context, parentID string, startTime, endTime *time.Time) (int, error) {
	q := `SELECT COUNT(*) FROM contents c
	      WHERE c.parent_id = ? AND c.status = 'approved' AND ` + SQLSoftDeleteCondition("c")
	args := []interface{}{parentID}
	if startTime != nil {
		q += " AND c.updated_at >= ?"
		args = append(args, *startTime)
	}
	if endTime != nil {
		q += " AND c.updated_at <= ?"
		args = append(args, *endTime)
	}
	var count int64
	err := r.db.DB().WithContext(ctx).Raw(q, args...).Scan(&count).Error
	return int(count), err
}

func (r *sqlContentRepo) ListAll(ctx context.Context, offset, limit int) ([]model.ContentListItem, error) {
	var items []model.ContentListItem
	q := `SELECT c.id, c.author_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS author_name, u.avatar AS author_avatar,
	      c.body, c.root_id, c.parent_id, c.like_count, c.created_at, c.updated_at,
	      p.id AS post_id, p.title AS post_title
	      FROM contents c
	      LEFT JOIN users u ON c.author_id = u.id
	      LEFT JOIN posts p ON p.content_id = c.root_id AND ` + SQLSoftDeleteCondition("p") + `
	      WHERE c.id != c.root_id AND ` + SQLSoftDeleteCondition("c") + `
	      ORDER BY c.created_at DESC LIMIT ? OFFSET ?`
	err := r.db.DB().WithContext(ctx).Raw(q, limit, offset).Scan(&items).Error
	return items, err
}

func (r *sqlContentRepo) ListAllWithTotal(ctx context.Context, offset, limit int) ([]model.ContentListItem, int, error) {
	type row struct {
		model.ContentListItem
		TotalCount int64
	}
	q := `SELECT c.id, c.author_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS author_name, u.avatar AS author_avatar,
	      c.body, c.root_id, c.parent_id, c.like_count, c.created_at, c.updated_at,
	      p.id AS post_id, p.title AS post_title,
	      COUNT(*) OVER() AS total_count
	      FROM contents c
	      LEFT JOIN users u ON c.author_id = u.id
	      LEFT JOIN posts p ON p.content_id = c.root_id AND ` + SQLSoftDeleteCondition("p") + `
	      WHERE c.id != c.root_id AND ` + SQLSoftDeleteCondition("c") + `
	      ORDER BY c.created_at DESC LIMIT ? OFFSET ?`
	var rows []row
	err := r.db.DB().WithContext(ctx).Raw(q, limit, offset).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	items := make([]model.ContentListItem, len(rows))
	for i, r := range rows {
		items[i] = r.ContentListItem
	}
	if len(rows) == 0 {
		return items, 0, nil
	}
	return items, int(rows[0].TotalCount), nil
}

func (r *sqlContentRepo) CountAll(ctx context.Context) (int, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Raw(`SELECT COUNT(*) FROM contents WHERE id != root_id AND ` + SQLSoftDeleteCondition()).Scan(&count).Error
	return int(count), err
}

func (r *sqlContentRepo) FindByID(ctx context.Context, id string) (*model.Content, error) {
	var c model.Content
	return &c, WrapDBErr(r.db.DB().WithContext(ctx).First(&c, "id = ?", id).Error)
}

func (r *sqlContentRepo) Create(ctx context.Context, c *model.Content) error {
	return r.db.DB().WithContext(ctx).Create(c).Error
}

func (r *sqlContentRepo) Update(ctx context.Context, c *model.Content) error {
	return r.db.DB().WithContext(ctx).Where(SQLSoftDeleteCondition()).Save(c).Error
}

func (r *sqlContentRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Content{}, "id = ?", id).Error
}

func (r *sqlContentRepo) Restore(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Unscoped().
		Model(&model.Content{}).
		Where("id = ?", id).
		Update("deleted_at", nil).Error
}

func (r *sqlContentRepo) ListTrash(ctx context.Context, offset, limit int) ([]model.ContentListItem, int, error) {
	type row struct {
		model.ContentListItem
		TotalCount int64
	}
	q := `SELECT c.id, c.author_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS author_name, u.avatar AS author_avatar,
	      c.body, c.root_id, c.parent_id, c.like_count, c.created_at, c.updated_at,
	      p.id AS post_id, p.title AS post_title,
	      COUNT(*) OVER() AS total_count
	      FROM contents c
	      LEFT JOIN users u ON c.author_id = u.id
	      LEFT JOIN posts p ON p.content_id = c.root_id
	      WHERE c.id != c.root_id AND c.deleted_at IS NOT NULL
	      ORDER BY c.deleted_at DESC, c.updated_at DESC LIMIT ? OFFSET ?`
	var rows []row
	if err := r.db.DB().WithContext(ctx).Raw(q, limit, offset).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.ContentListItem, len(rows))
	for i, row := range rows {
		items[i] = row.ContentListItem
	}
	if len(rows) == 0 {
		return items, 0, nil
	}
	return items, int(rows[0].TotalCount), nil
}

func (r *sqlContentRepo) IncrementLikeCount(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Model(&model.Content{}).Where("id = ?", id).
		UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
}

func (r *sqlContentRepo) DecrementLikeCount(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Model(&model.Content{}).Where("id = ? AND like_count > 0", id).
		UpdateColumn("like_count", gorm.Expr("like_count - 1")).Error
}
