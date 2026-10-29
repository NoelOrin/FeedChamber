package repository

import (
	"context"
	"time"

	"orinforum/internal/db"
	"orinforum/internal/model"
)

type sqlTrustRepo struct {
	db db.SQLStore
}

// ── 配置 ────────────────────────────────────────────────────────────────────

func (r *sqlTrustRepo) ListXPRules(ctx context.Context) ([]model.XPRule, error) {
	var rules []model.XPRule
	err := r.db.DB().WithContext(ctx).Order("metric ASC").Find(&rules).Error
	return rules, err
}

func (r *sqlTrustRepo) UpsertXPRule(ctx context.Context, rule *model.XPRule) error {
	rule.UpdatedAt = time.Now()
	return r.db.DB().WithContext(ctx).Save(rule).Error
}

func (r *sqlTrustRepo) DeleteXPRule(ctx context.Context, metric string) error {
	return r.db.DB().WithContext(ctx).Where("metric = ?", metric).Delete(&model.XPRule{}).Error
}

func (r *sqlTrustRepo) ListStreakBonuses(ctx context.Context) ([]model.XPStreakBonus, error) {
	var rows []model.XPStreakBonus
	err := r.db.DB().WithContext(ctx).Order("days ASC").Find(&rows).Error
	return rows, err
}

func (r *sqlTrustRepo) UpsertStreakBonus(ctx context.Context, bonus *model.XPStreakBonus) error {
	bonus.UpdatedAt = time.Now()
	return r.db.DB().WithContext(ctx).Save(bonus).Error
}

func (r *sqlTrustRepo) DeleteStreakBonus(ctx context.Context, days int) error {
	return r.db.DB().WithContext(ctx).Where("days = ?", days).Delete(&model.XPStreakBonus{}).Error
}

func (r *sqlTrustRepo) ListLevels(ctx context.Context) ([]model.TrustLevel, error) {
	var levels []model.TrustLevel
	err := r.db.DB().WithContext(ctx).Order("level ASC").Find(&levels).Error
	return levels, err
}

func (r *sqlTrustRepo) UpsertLevel(ctx context.Context, level *model.TrustLevel) error {
	level.UpdatedAt = time.Now()
	return r.db.DB().WithContext(ctx).Save(level).Error
}

func (r *sqlTrustRepo) DeleteLevel(ctx context.Context, level int) error {
	return r.db.DB().WithContext(ctx).Where("level = ?", level).Delete(&model.TrustLevel{}).Error
}

// ── 人工调整 ────────────────────────────────────────────────────────────────

func (r *sqlTrustRepo) CreateAdjustment(ctx context.Context, adj *model.XPAdjustment) error {
	return r.db.DB().WithContext(ctx).Create(adj).Error
}

func (r *sqlTrustRepo) GetAdjustment(ctx context.Context, id string) (*model.XPAdjustment, error) {
	var adj model.XPAdjustment
	if err := r.db.DB().WithContext(ctx).Where("id = ?", id).First(&adj).Error; err != nil {
		return nil, WrapDBErr(err)
	}
	return &adj, nil
}

func (r *sqlTrustRepo) DeleteAdjustment(ctx context.Context, id string) error {
	res := r.db.DB().WithContext(ctx).Where("id = ?", id).Delete(&model.XPAdjustment{})
	if res.Error != nil {
		return WrapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *sqlTrustRepo) ListAdjustments(ctx context.Context, userID string, offset, limit int) ([]model.XPAdjustment, error) {
	var rows []model.XPAdjustment
	err := r.db.DB().WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&rows).Error
	return rows, err
}

func (r *sqlTrustRepo) CountAdjustments(ctx context.Context, userID string) (int64, error) {
	var n int64
	err := r.db.DB().WithContext(ctx).Model(&model.XPAdjustment{}).Where("user_id = ?", userID).Count(&n).Error
	return n, err
}

func (r *sqlTrustRepo) SumAdjustments(ctx context.Context, userID string) (int, error) {
	var sum int
	err := r.db.DB().WithContext(ctx).
		Model(&model.XPAdjustment{}).
		Select("COALESCE(SUM(delta), 0)").
		Where("user_id = ?", userID).
		Scan(&sum).Error
	return sum, err
}

// ── 指标聚合 ────────────────────────────────────────────────────────────────

// dailyGroupByClause 返回某指标「按自然日分组」的 SQL 片段（含表连接与时间条件）。
// 只有需要每日上限的指标才会调用它。
func (r *sqlTrustRepo) dailyQuery(metric string) string {
	switch metric {
	case model.MetricPostApproved:
		return `SELECT COUNT(*) AS cnt
		        FROM posts
		        WHERE author_id = ? AND status IN ('approved','closed')
		          AND deleted_at IS NULL AND created_at >= ?
		        GROUP BY DATE(created_at)`
	case model.MetricCommentApproved:
		return `SELECT COUNT(*) AS cnt
		        FROM contents c
		        WHERE c.author_id = ? AND c.status = 'approved'
		          AND c.deleted_at IS NULL AND c.created_at >= ?
		          AND NOT EXISTS (SELECT 1 FROM posts p WHERE p.content_id = c.id)
		        GROUP BY DATE(c.created_at)`
	case model.MetricLikesReceived:
		return `SELECT COUNT(*) AS cnt
		        FROM content_likes cl
		        JOIN contents c ON c.id = cl.content_id
		        WHERE c.author_id = ? AND c.status = 'approved'
		          AND c.deleted_at IS NULL AND cl.created_at >= ?
		        GROUP BY DATE(cl.created_at)`
	case model.MetricFavoritesRecv:
		return `SELECT COUNT(*) AS cnt
		        FROM post_favorites pf
		        JOIN posts p ON p.id = pf.post_id
		        WHERE p.author_id = ? AND p.status IN ('approved','closed')
		          AND p.deleted_at IS NULL AND pf.created_at >= ?
		        GROUP BY DATE(pf.created_at)`
	default:
		return ""
	}
}

func (r *sqlTrustRepo) MetricDailyCounts(ctx context.Context, userID, metric string, since time.Time) ([]int, error) {
	query := r.dailyQuery(metric)
	if query == "" {
		return nil, nil
	}
	var counts []int
	if err := r.db.DB().WithContext(ctx).Raw(query, userID, since).Scan(&counts).Error; err != nil {
		return nil, err
	}
	return counts, nil
}

func (r *sqlTrustRepo) CountMetric(ctx context.Context, userID, metric string) (int, error) {
	var query string
	switch metric {
	case model.MetricPostApproved:
		query = `SELECT COUNT(*) FROM posts WHERE author_id = ? AND status IN ('approved','closed') AND deleted_at IS NULL`
	case model.MetricCommentApproved:
		query = `SELECT COUNT(*) FROM contents c WHERE c.author_id = ? AND c.status = 'approved' AND c.deleted_at IS NULL
		         AND NOT EXISTS (SELECT 1 FROM posts p WHERE p.content_id = c.id)`
	case model.MetricLikesReceived:
		query = `SELECT COUNT(*) FROM content_likes cl JOIN contents c ON c.id = cl.content_id
		         WHERE c.author_id = ? AND c.status = 'approved' AND c.deleted_at IS NULL`
	case model.MetricFavoritesRecv:
		query = `SELECT COUNT(*) FROM post_favorites pf JOIN posts p ON p.id = pf.post_id
		         WHERE p.author_id = ? AND p.status IN ('approved','closed') AND p.deleted_at IS NULL`
	case model.MetricFeaturedPost:
		query = `SELECT COUNT(*) FROM posts WHERE author_id = ? AND featured_at IS NOT NULL AND status IN ('approved','closed') AND deleted_at IS NULL`
	case model.MetricCheckin:
		query = `SELECT COUNT(*) FROM check_ins WHERE user_id = ?`
	default:
		return 0, nil
	}
	var n int
	if err := r.db.DB().WithContext(ctx).Raw(query, userID).Scan(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

func (r *sqlTrustRepo) StreakHits(ctx context.Context, userID string) (map[int]int, error) {
	var rows []struct {
		Streak int
		Cnt    int64
	}
	err := r.db.DB().WithContext(ctx).
		Raw(`SELECT streak, COUNT(*) AS cnt FROM check_ins WHERE user_id = ? GROUP BY streak`, userID).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[int]int, len(rows))
	for _, row := range rows {
		out[row.Streak] = int(row.Cnt)
	}
	return out, nil
}

// CountActiveDays 计算窗口内的活跃天数：当日有成功登录，或产生过帖子 / 评论 /
// 签到，该自然日即计一次。定义必须唯一，见 trust-levels 规范。
func (r *sqlTrustRepo) CountActiveDays(ctx context.Context, userID string, since time.Time) (int, error) {
	sinceStr := since.Format("2006-01-02")
	query := `SELECT COUNT(*) FROM (
		SELECT DATE(created_at) AS d FROM posts WHERE author_id = ? AND deleted_at IS NULL AND created_at >= ?
		UNION
		SELECT DATE(created_at) AS d FROM contents WHERE author_id = ? AND deleted_at IS NULL AND created_at >= ?
		UNION
		SELECT DATE(check_in_date) AS d FROM check_ins WHERE user_id = ? AND check_in_date >= ?
		UNION
		SELECT DATE(created_at) AS d FROM login_histories WHERE user_uuid = ? AND success = ? AND created_at >= ?
	) t`
	var n int
	err := r.db.DB().WithContext(ctx).Raw(query,
		userID, since,
		userID, since,
		userID, sinceStr,
		userID, true, since,
	).Scan(&n).Error
	return n, err
}

// CountUpheldReports 只统计已受理成立（resolved）且被举报内容属于该用户的举报。
// 任意举报都计入会让恶意举报成为压制他人等级的工具。
func (r *sqlTrustRepo) CountUpheldReports(ctx context.Context, userID string, since time.Time) (int, error) {
	query := `SELECT COUNT(*) FROM reports r
		WHERE r.status = 'resolved' AND COALESCE(r.handled_at, r.created_at) >= ?
		  AND (
		    (r.target_type = 'post' AND EXISTS (SELECT 1 FROM posts p WHERE p.id = r.target_id AND p.author_id = ?))
		    OR
		    (r.target_type = 'content' AND EXISTS (SELECT 1 FROM contents c WHERE c.id = r.target_id AND c.author_id = ?))
		  )`
	var n int
	err := r.db.DB().WithContext(ctx).Raw(query, since, userID, userID).Scan(&n).Error
	return n, err
}

// ── 派生缓存写入 ────────────────────────────────────────────────────────────

func (r *sqlTrustRepo) SaveLevelSnapshot(ctx context.Context, userID string, totalXP, level int, metricsJSON string, evaluatedAt time.Time) error {
	return r.db.DB().WithContext(ctx).Model(&model.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"total_xp":           totalXP,
			"trust_level":        level,
			"level_metrics":      metricsJSON,
			"level_evaluated_at": evaluatedAt,
		}).Error
}

func (r *sqlTrustRepo) SetManualLevel(ctx context.Context, userID string, level int, manual bool) error {
	return r.db.DB().WithContext(ctx).Model(&model.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"trust_level":  level,
			"level_manual": manual,
		}).Error
}
