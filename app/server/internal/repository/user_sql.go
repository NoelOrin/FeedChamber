package repository

import (
	"context"
	"fmt"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ── SQL implementation ──────────────────────────────────────────────────────

type sqlUserRepo struct{ db db.SQLStore }

func (r *sqlUserRepo) FindByID(ctx context.Context, id string) (*model.User, error) {
	var u model.User
	return &u, WrapDBErr(r.db.DB().WithContext(ctx).First(&u, "id = ?", id).Error)
}

func (r *sqlUserRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	return &u, WrapDBErr(r.db.DB().WithContext(ctx).First(&u, "email = ?", email).Error)
}

func (r *sqlUserRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	return &u, WrapDBErr(r.db.DB().WithContext(ctx).First(&u, "username = ?", username).Error)
}

func (r *sqlUserRepo) FindByIdentifier(ctx context.Context, identifier string) (*model.User, error) {
	var u model.User
	if strings.Contains(identifier, "@") {
		return &u, WrapDBErr(r.db.DB().WithContext(ctx).First(&u, "email = ?", identifier).Error)
	}
	return &u, WrapDBErr(r.db.DB().WithContext(ctx).First(&u, "username = ?", identifier).Error)
}

func (r *sqlUserRepo) List(ctx context.Context, offset, limit int) ([]model.User, error) {
	var users []model.User
	err := r.db.DB().WithContext(ctx).Order("created_at desc").Offset(offset).Limit(limit).Find(&users).Error
	return users, err
}

func (r *sqlUserRepo) Search(ctx context.Context, keyword, role string, offset, limit int) ([]model.User, error) {
	query := r.db.DB().WithContext(ctx).Model(&model.User{})
	if keyword != "" {
		query = query.Where("username LIKE ? OR email LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if role != "" {
		query = query.Where("role = ?", role)
	}
	var users []model.User
	err := query.Order("created_at desc").Offset(offset).Limit(limit).Find(&users).Error
	return users, err
}

func (r *sqlUserRepo) Create(ctx context.Context, user *model.User) error {
	if err := r.db.DB().WithContext(ctx).Unscoped().Where("username = ? AND deleted_at IS NOT NULL", user.Username).Delete(&model.User{}).Error; err != nil {
		logrus.Warnf("user: cleanup soft-deleted record failed (username=%s): %v", user.Username, err)
	}
	if err := r.db.DB().WithContext(ctx).Unscoped().Where("email = ? AND deleted_at IS NOT NULL", user.Email).Delete(&model.User{}).Error; err != nil {
		logrus.Warnf("user: cleanup soft-deleted record failed (email=%s): %v", user.Email, err)
	}
	return r.db.DB().WithContext(ctx).Create(user).Error
}

func (r *sqlUserRepo) Update(ctx context.Context, user *model.User) error {
	// XP / 等级是派生缓存，唯一写入点是 TrustRepository.SaveLevelSnapshot。
	// 这里 Omit 掉，避免任何「读旧行 → 全行 Save」的路径把刚重算的快照覆盖回
	// 旧值（例如登录更新 last_login、改资料、签到统计）。
	return r.db.TxDB(ctx).Where(SQLSoftDeleteCondition()).
		Omit("total_xp", "trust_level", "level_evaluated_at", "level_metrics", "level_manual").
		Save(user).Error
}

func (r *sqlUserRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.User{}, "id = ?", id).Error
}

func (r *sqlUserRepo) CountUsersByPermission(ctx context.Context, permCode string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).
		Model(&model.User{}).
		Joins("JOIN role_permissions ON role_permissions.role_name = users.role").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Where("permissions.code = ?", permCode).
		Where("users.deleted_at IS NULL").
		Count(&count).Error
	return count, err
}

func (r *sqlUserRepo) GetActivityStats(ctx context.Context, userID string, year int) (map[string]int, error) {
	result := make(map[string]int)
	type row struct {
		Date  string
		Count int
	}
	var rows []row
	err := r.db.DB().WithContext(ctx).
		Raw(`SELECT date(created_at) as date, COUNT(*) as count FROM posts
			WHERE author_id = ? AND strftime('%Y', created_at) = ?
			GROUP BY date(created_at)`, userID, fmt.Sprintf("%d", year)).
		Scan(&rows).Error
	if err != nil {
		return result, err
	}
	for _, r := range rows {
		result[r.Date] += r.Count
	}
	var contentRows []row
	err = r.db.DB().WithContext(ctx).
		Raw(`SELECT date(created_at) as date, COUNT(*) as count FROM contents
			WHERE author_id = ? AND strftime('%Y', created_at) = ?
			GROUP BY date(created_at)`, userID, fmt.Sprintf("%d", year)).
		Scan(&contentRows).Error
	if err != nil {
		return result, nil
	}
	for _, r := range contentRows {
		result[r.Date] += r.Count
	}
	return result, nil
}

func (r *sqlUserRepo) CountPosts(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("author_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *sqlUserRepo) CountCommentsByUser(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.Content{}).Where("author_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *sqlUserRepo) CountLikesByUser(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.ContentLike{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *sqlUserRepo) CountLikesReceived(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM content_likes cl
		INNER JOIN contents c ON c.id = cl.content_id
		WHERE c.author_id = ? AND c.deleted_at IS NULL
		  AND c.status = 'approved' AND cl.user_id <> c.author_id
	`, userID).Scan(&count).Error
	return count, err
}

func (r *sqlUserRepo) IncrPoints(ctx context.Context, userID string, delta int) error {
	return r.db.DB().WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).UpdateColumn("total_xp", gorm.Expr("total_xp + ?", delta)).Error
}

func (r *sqlUserRepo) ListByPoints(ctx context.Context, offset, limit int) ([]model.User, error) {
	var users []model.User
	err := r.db.DB().WithContext(ctx).Order("total_xp desc, created_at asc").Offset(offset).Limit(limit).Find(&users).Error
	return users, err
}

func (r *sqlUserRepo) ListByCheckIns(ctx context.Context, offset, limit int) ([]model.User, error) {
	var users []model.User
	err := r.db.DB().WithContext(ctx).Order("total_checkins desc, created_at asc").Offset(offset).Limit(limit).Find(&users).Error
	return users, err
}

func (r *sqlUserRepo) ListByPostCount(ctx context.Context, offset, limit int) ([]model.User, error) {
	var users []model.User
	err := r.db.DB().WithContext(ctx).Raw(`
		SELECT u.* FROM users u
		LEFT JOIN (
			SELECT author_id, COUNT(*) AS c FROM posts WHERE deleted_at IS NULL GROUP BY author_id
		) p ON p.author_id = u.id
		ORDER BY COALESCE(p.c,0) DESC, u.created_at ASC
		LIMIT ? OFFSET ?
	`, limit, offset).Scan(&users).Error
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return users, nil
	}
	ids := make([]string, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}
	type cntRow struct {
		AuthorID string `gorm:"column:author_id"`
		Cnt      int    `gorm:"column:cnt"`
	}
	var rows []cntRow
	if err := r.db.DB().WithContext(ctx).Raw(`SELECT author_id, COUNT(*) as cnt FROM posts WHERE author_id IN ? AND deleted_at IS NULL GROUP BY author_id`, ids).Scan(&rows).Error; err != nil {
		return users, nil
	}
	m := make(map[string]int, len(rows))
	for _, r := range rows {
		m[r.AuthorID] = r.Cnt
	}
	for i := range users {
		users[i].PostCount = m[users[i].ID]
	}
	return users, nil
}

func (r *sqlUserRepo) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.DB().WithContext(ctx).Model(&model.User{}).Where("deleted_at IS NULL").Count(&n).Error
	return n, err
}

func (r *sqlUserRepo) CountDistinctPostAuthors(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.DB().WithContext(ctx).Raw(`SELECT COUNT(DISTINCT author_id) FROM posts WHERE deleted_at IS NULL`).Scan(&n).Error
	return n, err
}
