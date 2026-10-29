package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"strings"
	"time"
)

type sqlAdminRepo struct{ db db.SQLStore }

func (r *sqlAdminRepo) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.User{}).Count(&count).Error
	return count, err
}

func (r *sqlAdminRepo) CountPosts(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.Post{}).Count(&count).Error
	return count, err
}

func (r *sqlAdminRepo) CountComments(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.Content{}).Where("id != root_id").Count(&count).Error
	return count, err
}

func (r *sqlAdminRepo) CountTodayUsers(ctx context.Context) (int64, error) {
	var count int64
	today := time.Now().Truncate(24 * time.Hour)
	err := r.db.DB().WithContext(ctx).Model(&model.User{}).
		Where("created_at >= ?", today).Count(&count).Error
	return count, err
}

func (r *sqlAdminRepo) CountTodayPosts(ctx context.Context) (int64, error) {
	var count int64
	today := time.Now().Truncate(24 * time.Hour)
	err := r.db.DB().WithContext(ctx).Model(&model.Post{}).
		Where("created_at >= ?", today).Count(&count).Error
	return count, err
}

func (r *sqlAdminRepo) CountTodayComments(ctx context.Context) (int64, error) {
	var count int64
	today := time.Now().Truncate(24 * time.Hour)
	err := r.db.DB().WithContext(ctx).Model(&model.Content{}).
		Where("id != root_id AND created_at >= ?", today).Count(&count).Error
	return count, err
}

func (r *sqlAdminRepo) SearchUsers(ctx context.Context, keyword string, offset, limit int) ([]model.User, int64, error) {
	db := r.db.DB().WithContext(ctx).Model(&model.User{})
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("username LIKE ? OR email LIKE ?", like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []model.User
	err := db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&users).Error
	return users, total, err
}

func (r *sqlAdminRepo) SearchPosts(ctx context.Context, keyword string, offset, limit int) ([]model.Post, int64, error) {
	db := r.db.DB().WithContext(ctx).Model(&model.Post{})
	if keyword != "" {
		db = db.Where("title LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var posts []model.Post
	err := db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&posts).Error
	return posts, total, err
}

func (r *sqlAdminRepo) SearchComments(ctx context.Context, keyword string, offset, limit int) ([]model.Content, int64, error) {
	db := r.db.DB().WithContext(ctx).Model(&model.Content{}).Where("id != root_id")
	if keyword != "" {
		db = db.Where("body LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var comments []model.Content
	err := db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&comments).Error
	return comments, total, err
}

func (r *sqlAdminRepo) GetUserPosts(ctx context.Context, userID string, offset, limit int) ([]model.Post, int64, error) {
	var total int64
	if err := r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("author_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var posts []model.Post
	err := r.db.DB().WithContext(ctx).Where("author_id = ?", userID).Order("created_at DESC").Offset(offset).Limit(limit).Find(&posts).Error
	return posts, total, err
}

func (r *sqlAdminRepo) GetUserComments(ctx context.Context, userID string, offset, limit int) ([]model.Content, int64, error) {
	var total int64
	if err := r.db.DB().WithContext(ctx).Model(&model.Content{}).Where("author_id = ? AND id != root_id", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var comments []model.Content
	err := r.db.DB().WithContext(ctx).Where("author_id = ? AND id != root_id", userID).Order("created_at DESC").Offset(offset).Limit(limit).Find(&comments).Error
	return comments, total, err
}

func (r *sqlAdminRepo) GetUserReportsAbout(ctx context.Context, userID string, offset, limit int) ([]model.Report, int64, error) {
	var total int64
	if err := r.db.DB().WithContext(ctx).Model(&model.Report{}).Where("target_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var reports []model.Report
	err := r.db.DB().WithContext(ctx).Where("target_id = ?", userID).Order("created_at DESC").Offset(offset).Limit(limit).Find(&reports).Error
	return reports, total, err
}

func (r *sqlAdminRepo) GetUserReportsBy(ctx context.Context, userID string, offset, limit int) ([]model.Report, int64, error) {
	var total int64
	if err := r.db.DB().WithContext(ctx).Model(&model.Report{}).Where("reporter_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var reports []model.Report
	err := r.db.DB().WithContext(ctx).Where("reporter_id = ?", userID).Order("created_at DESC").Offset(offset).Limit(limit).Find(&reports).Error
	return reports, total, err
}

func (r *sqlAdminRepo) CountPendingReports(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.Report{}).Where("status = ?", "pending").Count(&count).Error
	return count, err
}

func (r *sqlAdminRepo) ListRecentUsers(ctx context.Context, limit int) ([]model.User, error) {
	var users []model.User
	err := r.db.DB().WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&users).Error
	return users, err
}

func (r *sqlAdminRepo) ListRecentPosts(ctx context.Context, limit int) ([]model.Post, error) {
	var posts []model.Post
	err := r.db.DB().WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&posts).Error
	return posts, err
}

func (r *sqlAdminRepo) BatchDeleteUsers(ctx context.Context, ids []string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.User{}, "id IN ?", ids).Error
}

func (r *sqlAdminRepo) BatchUpdateUserRole(ctx context.Context, ids []string, role string) error {
	return r.db.DB().WithContext(ctx).Model(&model.User{}).Where("id IN ?", ids).Update("role", role).Error
}

func (r *sqlAdminRepo) BatchDeletePosts(ctx context.Context, ids []string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Post{}, "id IN ?", ids).Error
}

func (r *sqlAdminRepo) BatchTogglePinPosts(ctx context.Context, ids []string, pinned bool) error {
	return r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("id IN ?", ids).Update("pinned", pinned).Error
}

func (r *sqlAdminRepo) BatchToggleLockPosts(ctx context.Context, ids []string, locked bool) error {
	return r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("id IN ?", ids).Update("locked", locked).Error
}

func (r *sqlAdminRepo) BatchDeleteComments(ctx context.Context, ids []string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Content{}, "id IN ?", ids).Error
}

// ReconcileCounters recalculates comment_count for all posts using a single
// bulk UPDATE with a CASE expression, avoiding an N+1 loop. This approach is
// portable across SQLite, PostgreSQL, and MySQL.
func (r *sqlAdminRepo) ReconcileCounters(ctx context.Context) (int, error) {
	type row struct {
		PostID      string
		ActualCount int
	}
	var rows []row
	err := r.db.DB().WithContext(ctx).Raw(`
		SELECT p.root_id AS post_id, COUNT(*) - 1 AS actual_count
		FROM contents c
		JOIN posts p ON p.content_id = c.root_id
		GROUP BY p.root_id
	`).Scan(&rows).Error
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	// Build a single bulk UPDATE using a CASE expression.
	// This works on SQLite, PostgreSQL, and MySQL alike.
	var (
		updateSQL strings.Builder
		args      []interface{}
	)
	updateSQL.WriteString("UPDATE posts SET comment_count = CASE id")
	for _, row := range rows {
		updateSQL.WriteString(" WHEN ? THEN ?")
		args = append(args, row.PostID, row.ActualCount)
	}
	updateSQL.WriteString(" END WHERE id IN (")
	for i, row := range rows {
		if i > 0 {
			updateSQL.WriteString(",")
		}
		updateSQL.WriteString("?")
		args = append(args, row.PostID)
	}
	updateSQL.WriteString(") AND comment_count != CASE id")
	for _, row := range rows {
		updateSQL.WriteString(" WHEN ? THEN ?")
		args = append(args, row.PostID, row.ActualCount)
	}
	updateSQL.WriteString(" END")

	result := r.db.DB().WithContext(ctx).Exec(updateSQL.String(), args...)
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}
