package repository

import (
	"context"
	"time"

	"orinforum/internal/db"
	"orinforum/internal/model"

	"gorm.io/gorm"
)

type sqlPostRepo struct{ db db.SQLStore }

// pinOrderPrefix is prepended to every list query ORDER BY so pinned posts
// always sort above unpinned ones, ordered by pin_order then pinned_at.
const pinOrderPrefix = "CASE WHEN p.pinned = true AND p.pinned_at IS NOT NULL THEN 0 ELSE 1 END, p.pin_order ASC, COALESCE(p.pinned_at, '1970-01-01') DESC, "

func (r *sqlPostRepo) List(ctx context.Context, offset, limit int) ([]model.Post, error) {
	var posts []model.Post
	err := r.db.DB().WithContext(ctx).Order("created_at desc").Offset(offset).Limit(limit).Find(&posts).Error
	return posts, err
}

func (r *sqlPostRepo) FindByID(ctx context.Context, id string) (*model.Post, error) {
	var p model.Post
	return &p, WrapDBErr(r.db.DB().WithContext(ctx).First(&p, "id = ?", id).Error)
}

func (r *sqlPostRepo) FindByContentID(ctx context.Context, contentID string) (*model.Post, error) {
	var p model.Post
	return &p, WrapDBErr(r.db.DB().WithContext(ctx).First(&p, "content_id = ?", contentID).Error)
}

func (r *sqlPostRepo) FindByIDs(ctx context.Context, ids []string) ([]*model.Post, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var posts []*model.Post
	err := r.db.DB().WithContext(ctx).Where("id IN ?", ids).Find(&posts).Error
	return posts, err
}

func (r *sqlPostRepo) FindByIDsDetails(ctx context.Context, ids []string) ([]model.PostListItem, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var items []model.PostListItem
	err := r.db.DB().WithContext(ctx).Raw(`
			SELECT p.id, p.title, p.slug, p.content_digest, p.author_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS author_name, u.avatar AS author_avatar,
			       p.space_id, sp.name AS space_name,
			       p.status, p.min_level, p.pinned, p.pin_order, p.pinned_at, p.featured_at, p.locked, p.last_active_at, p.view_count,
			       p.comment_count, p.like_count, p.favorite_count,
			       p.created_at, p.updated_at
			FROM posts p
			LEFT JOIN users u ON p.author_id = u.id
			LEFT JOIN spaces sp ON p.space_id = sp.id
			WHERE p.id IN ? AND `+SQLSoftDeleteCondition("p")+`
			ORDER BY `+pinOrderPrefix+`p.created_at DESC
		`, ids).Scan(&items).Error
	return items, err
}

func (r *sqlPostRepo) Create(ctx context.Context, post *model.Post) error {
	return r.db.DB().WithContext(ctx).Create(post).Error
}

func (r *sqlPostRepo) CreateWithContent(ctx context.Context, post *model.Post, content *model.Content) error {
	return r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(content).Error; err != nil {
			return err
		}
		return tx.Create(post).Error
	})
}

func (r *sqlPostRepo) Update(ctx context.Context, post *model.Post) error {
	// Updates(struct) 会跳过零值，导致 exiled/folded/locked/pinned 这类
	// "从 true 改回 false" 的治理动作静默失效（翻案就是典型场景：解除软隔离
	// 必须把 posts.exiled 写回 0）。这里用 map + Unscoped 显式写列，
	// 避开 GORM 对零值字段的裁剪。
	updates := map[string]any{
		"title":            post.Title,
		"slug":             post.Slug,
		"content_digest":   post.ContentDigest,
		"space_id":         post.SpaceID,
		"status":           post.Status,
		"min_level":        post.MinLevel,
		"pinned":           post.Pinned,
		"pin_order":        post.PinOrder,
		"pinned_at":        post.PinnedAt,
		"pinned_expire_at": post.PinnedExpireAt,
		"featured_at":      post.FeaturedAt,
		"locked":           post.Locked,
		"exiled":           post.Exiled,
		"folded":           post.Folded,
		"watched_until":    post.WatchedUntil,
		"last_active_at":   post.LastActiveAt,
		"view_count":       post.ViewCount,
		"comment_count":    post.CommentCount,
		"like_count":       post.LikeCount,
		"favorite_count":   post.FavoriteCount,
		"updated_at":       time.Now(),
	}
	res := r.db.DB().WithContext(ctx).Unscoped().Model(&model.Post{}).
		Where("id = ?", post.ID).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *sqlPostRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Post{}, "id = ?", id).Error
}

func (r *sqlPostRepo) Restore(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Unscoped().
		Model(&model.Post{}).
		Where("id = ?", id).
		Update("deleted_at", nil).Error
}

func (r *sqlPostRepo) ListTrash(ctx context.Context, offset, limit int) ([]model.PostListItem, int, error) {
	type row struct {
		model.PostListItem
		TotalCount int64
	}
	q := `
			SELECT p.id, p.title, p.slug, p.content_digest, p.author_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS author_name, u.avatar AS author_avatar,
			       p.space_id, sp.name AS space_name,
			       p.status, p.min_level, p.pinned, p.pin_order, p.pinned_at, p.featured_at, p.locked, p.last_active_at, p.view_count,
			       p.comment_count, p.like_count, p.favorite_count,
			       p.created_at, p.updated_at,
			       COUNT(*) OVER() AS total_count
			FROM posts p
			LEFT JOIN users u ON p.author_id = u.id
			LEFT JOIN spaces sp ON p.space_id = sp.id
			WHERE p.deleted_at IS NOT NULL
			ORDER BY p.deleted_at DESC, p.updated_at DESC
			LIMIT ? OFFSET ?`
	var rows []row
	if err := r.db.DB().WithContext(ctx).Raw(q, limit, offset).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.PostListItem, len(rows))
	for i, row := range rows {
		items[i] = row.PostListItem
	}
	if len(rows) == 0 {
		return items, 0, nil
	}
	return items, int(rows[0].TotalCount), nil
}

func (r *sqlPostRepo) TogglePinned(ctx context.Context, id string) (*model.Post, error) {
	now := time.Now()
	if err := r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("id = ?", id).
		Updates(map[string]any{"pinned": gorm.Expr("NOT pinned"), "updated_at": now}).Error; err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *sqlPostRepo) ToggleLocked(ctx context.Context, id string) (*model.Post, error) {
	now := time.Now()
	if err := r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("id = ?", id).
		Updates(map[string]any{"locked": gorm.Expr("NOT locked"), "updated_at": now}).Error; err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *sqlPostRepo) TouchActive(ctx context.Context, id string, at time.Time) error {
	return r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("id = ?", id).
		Updates(map[string]any{"last_active_at": at, "updated_at": at}).Error
}

func (r *sqlPostRepo) IncrementViewCount(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

func (r *sqlPostRepo) IncrementViewCountBy(ctx context.Context, id string, delta int64) error {
	return r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + ?", delta)).Error
}

func (r *sqlPostRepo) IncrementCommentCount(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("id = ?", id).
		Updates(map[string]any{"comment_count": gorm.Expr("comment_count + 1"), "updated_at": time.Now()}).Error
}

func (r *sqlPostRepo) DecrementCommentCount(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("id = ? AND comment_count > 0", id).
		Updates(map[string]any{"comment_count": gorm.Expr("comment_count - 1"), "updated_at": time.Now()}).Error
}

func (r *sqlPostRepo) IncrementLikeCount(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("id = ?", id).
		UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
}

func (r *sqlPostRepo) DecrementLikeCount(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("id = ? AND like_count > 0", id).
		UpdateColumn("like_count", gorm.Expr("like_count - 1")).Error
}

func (r *sqlPostRepo) IncrementFavoriteCount(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("id = ?", id).
		UpdateColumn("favorite_count", gorm.Expr("favorite_count + 1")).Error
}

func (r *sqlPostRepo) DecrementFavoriteCount(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("id = ? AND favorite_count > 0", id).
		UpdateColumn("favorite_count", gorm.Expr("favorite_count - 1")).Error
}

func (r *sqlPostRepo) FindByIDWithDetails(ctx context.Context, id string) (*model.PostListItem, error) {
	var item model.PostListItem
	err := r.db.DB().WithContext(ctx).Raw(`
			SELECT p.id, p.title, p.slug, p.content_digest, p.author_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS author_name, u.avatar AS author_avatar,
			       p.space_id, sp.name AS space_name,
			       p.status, p.min_level, p.pinned, p.pin_order, p.pinned_at, p.featured_at, p.locked, p.last_active_at, p.view_count,
			       p.comment_count, p.like_count, p.favorite_count,
			       p.created_at, p.updated_at
			FROM posts p
			LEFT JOIN users u ON p.author_id = u.id
			LEFT JOIN spaces sp ON p.space_id = sp.id
			WHERE p.id = ? AND `+SQLSoftDeleteCondition("p")+`
		`, id).Scan(&item).Error
	if err != nil {
		return nil, WrapDBErr(err)
	}
	return &item, nil
}

func (r *sqlPostRepo) FindDetailByID(ctx context.Context, id string) (*model.PostDetail, error) {
	var detail model.PostDetail
	err := r.db.DB().WithContext(ctx).Raw(`
			SELECT p.id, p.title, p.slug, p.content_digest, p.author_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS author_name, u.avatar AS author_avatar,
			       p.space_id, sp.name AS space_name,
			       p.status, p.min_level, p.pinned, p.pin_order, p.pinned_at, p.featured_at, p.locked, p.last_active_at, p.view_count,
			       p.comment_count, p.like_count, p.favorite_count,
			       p.created_at, p.updated_at,
			       ct.body
			FROM posts p
			LEFT JOIN users u ON p.author_id = u.id
			LEFT JOIN spaces sp ON p.space_id = sp.id
			LEFT JOIN contents ct ON p.content_id = ct.id
			WHERE p.id = ? AND `+SQLSoftDeleteCondition("p")+`
		`, id).Scan(&detail).Error
	if err != nil {
		return nil, WrapDBErr(err)
	}
	return &detail, nil
}

func (r *sqlPostRepo) ListWithTotal(ctx context.Context, filter PostListFilter, offset, limit int) ([]model.PostListItem, int, error) {
	type listWithTotalRow struct {
		model.PostListItem
		TotalCount int64
	}
	base := `
			SELECT p.id, p.title, p.slug, p.content_digest, p.author_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS author_name, u.avatar AS author_avatar,
			       p.space_id, sp.name AS space_name,
			       p.status, p.min_level, p.pinned, p.pin_order, p.pinned_at, p.featured_at, p.locked, p.last_active_at, p.view_count,
			       p.comment_count, p.like_count, p.favorite_count,
			       p.created_at, p.updated_at,
			       COUNT(*) OVER() AS total_count
			FROM posts p
			LEFT JOIN users u ON p.author_id = u.id
			LEFT JOIN spaces sp ON p.space_id = sp.id
			WHERE ` + SQLSoftDeleteCondition("p")
	where, args := buildPostListWhere(filter)
	orderBy := pinOrderPrefix + "p.last_active_at DESC, p.created_at DESC"
	switch filter.SortBy {
	case "hot":
		orderBy = pinOrderPrefix + "(p.like_count + p.comment_count * 2 + p.view_count / 10) DESC, p.created_at DESC"
	case "most_commented":
		orderBy = pinOrderPrefix + "p.comment_count DESC, p.created_at DESC"
	}
	query := base + where + " ORDER BY " + orderBy + " LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	var rows []listWithTotalRow
	err := r.db.DB().WithContext(ctx).Raw(query, args...).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	items := make([]model.PostListItem, len(rows))
	for i, row := range rows {
		items[i] = row.PostListItem
	}
	if len(rows) == 0 {
		return items, 0, nil
	}
	return items, int(rows[0].TotalCount), nil
}

func (r *sqlPostRepo) CountSearch(ctx context.Context, keyword string) (int, error) {
	like := "%" + keyword + "%"
	var count int64
	err := r.db.DB().WithContext(ctx).Raw(`
			SELECT COUNT(*) FROM posts p
			WHERE `+SQLSoftDeleteCondition("p")+` AND p.status IN ('approved','closed') AND (p.title LIKE ? OR p.content_digest LIKE ?)
		`, like, like).Scan(&count).Error
	return int(count), err
}

func (r *sqlPostRepo) Search(ctx context.Context, keyword string, offset, limit int) ([]model.PostListItem, error) {
	var items []model.PostListItem
	like := "%" + keyword + "%"
	err := r.db.DB().WithContext(ctx).Raw(`
			SELECT p.id, p.title, p.slug, p.content_digest, p.author_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS author_name, u.avatar AS author_avatar,
			       p.comment_count, p.like_count, p.favorite_count,
			       p.created_at, p.updated_at
			FROM posts p
			LEFT JOIN users u ON p.author_id = u.id
			WHERE `+SQLSoftDeleteCondition("p")+` AND p.status IN ('approved','closed') AND (p.title LIKE ? OR p.content_digest LIKE ?)
			ORDER BY `+pinOrderPrefix+`p.last_active_at DESC, p.created_at DESC
			LIMIT ? OFFSET ?
		`, like, like, limit, offset).Scan(&items).Error
	return items, err
}

func (r *sqlPostRepo) SearchWithTotal(ctx context.Context, keyword string, offset, limit int) ([]model.PostListItem, int, error) {
	type listWithTotalRow struct {
		model.PostListItem
		TotalCount int64
	}
	like := "%" + keyword + "%"
	var rows []listWithTotalRow
	err := r.db.DB().WithContext(ctx).Raw(`
			SELECT p.id, p.title, p.slug, p.content_digest, p.author_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS author_name, u.avatar AS author_avatar,
			       p.space_id, sp.name AS space_name,
			       p.status, p.min_level, p.pinned, p.pin_order, p.pinned_at, p.featured_at, p.locked, p.last_active_at, p.view_count,
			       p.comment_count, p.like_count, p.favorite_count,
			       p.created_at, p.updated_at,
			       COUNT(*) OVER() AS total_count
			FROM posts p
			LEFT JOIN users u ON p.author_id = u.id
			LEFT JOIN spaces sp ON p.space_id = sp.id
			WHERE `+SQLSoftDeleteCondition("p")+` AND p.status IN ('approved','closed') AND (p.title LIKE ? OR p.content_digest LIKE ?)
			ORDER BY `+pinOrderPrefix+`p.last_active_at DESC, p.created_at DESC
			LIMIT ? OFFSET ?
		`, like, like, limit, offset).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	items := make([]model.PostListItem, len(rows))
	for i, row := range rows {
		items[i] = row.PostListItem
	}
	if len(rows) == 0 {
		return items, 0, nil
	}
	return items, int(rows[0].TotalCount), nil
}

func (r *sqlPostRepo) SearchWithTotalFilter(ctx context.Context, keyword string, filter PostListFilter, offset, limit int) ([]model.PostListItem, int, error) {
	type listWithTotalRow struct {
		model.PostListItem
		TotalCount int64
	}
	like := "%" + keyword + "%"
	useFTS := r.db.DB().Dialector.Name() == "postgres"
	var base string
	var args []interface{}
	var scoreOrder string
	if useFTS {
		base = `
			SELECT p.id, p.title, p.slug, p.content_digest, p.author_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS author_name, u.avatar AS author_avatar,
			       p.space_id, sp.name AS space_name,
			       p.status, p.min_level, p.pinned, p.pin_order, p.pinned_at, p.featured_at, p.locked, p.last_active_at, p.view_count,
			       p.comment_count, p.like_count, p.favorite_count,
			       p.created_at, p.updated_at,
			       COUNT(*) OVER() AS total_count
			FROM posts p
			LEFT JOIN users u ON p.author_id = u.id
			LEFT JOIN spaces sp ON p.space_id = sp.id
			WHERE ` + SQLSoftDeleteCondition("p") + ` AND p.status IN ('approved','closed') AND (
				to_tsvector('simple', p.title) @@ plainto_tsquery('simple', ?)
				OR to_tsvector('simple', p.content_digest) @@ plainto_tsquery('simple', ?)
				OR EXISTS (
					SELECT 1 FROM contents c
					WHERE c.root_id = p.content_id
					  AND c.status = 'approved'
					  AND ` + SQLSoftDeleteCondition("c") + `
					  AND to_tsvector('simple', c.body) @@ plainto_tsquery('simple', ?)
				)
			)`
		args = []interface{}{keyword, keyword, keyword}
		scoreOrder = `
			(CASE WHEN to_tsvector('simple', p.title) @@ plainto_tsquery('simple', ?) THEN 3 ELSE 0 END
			+ CASE WHEN to_tsvector('simple', p.content_digest) @@ plainto_tsquery('simple', ?) THEN 1 ELSE 0 END
			+ CASE WHEN EXISTS (
				SELECT 1 FROM contents c
				WHERE c.root_id = p.content_id
				  AND c.status = 'approved'
				  AND ` + SQLSoftDeleteCondition("c") + `
				  AND to_tsvector('simple', c.body) @@ plainto_tsquery('simple', ?)
			) THEN 2 ELSE 0 END) DESC, `
	} else {
		base = `
			SELECT p.id, p.title, p.slug, p.content_digest, p.author_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS author_name, u.avatar AS author_avatar,
			       p.space_id, sp.name AS space_name,
			       p.status, p.min_level, p.pinned, p.pin_order, p.pinned_at, p.featured_at, p.locked, p.last_active_at, p.view_count,
			       p.comment_count, p.like_count, p.favorite_count,
			       p.created_at, p.updated_at,
			       COUNT(*) OVER() AS total_count
			FROM posts p
			LEFT JOIN users u ON p.author_id = u.id
			LEFT JOIN spaces sp ON p.space_id = sp.id
			WHERE ` + SQLSoftDeleteCondition("p") + ` AND p.status IN ('approved','closed') AND (
				p.title LIKE ?
				OR p.content_digest LIKE ?
				OR EXISTS (
					SELECT 1 FROM contents c
					WHERE c.root_id = p.content_id
					  AND c.status = 'approved'
					  AND ` + SQLSoftDeleteCondition("c") + `
					  AND c.body LIKE ?
				)
			)`
		args = []interface{}{like, like, like}
		scoreOrder = `
			(CASE WHEN p.title LIKE ? THEN 3 ELSE 0 END
			+ CASE WHEN p.content_digest LIKE ? THEN 1 ELSE 0 END
			+ CASE WHEN EXISTS (
				SELECT 1 FROM contents c
				WHERE c.root_id = p.content_id
				  AND c.status = 'approved'
				  AND ` + SQLSoftDeleteCondition("c") + `
				  AND c.body LIKE ?
			) THEN 2 ELSE 0 END) DESC, `
	}
	if len(filter.ExcludeAuthorIDs) > 0 {
		base += " AND p.author_id NOT IN ("
		for i, id := range filter.ExcludeAuthorIDs {
			if i > 0 {
				base += ","
			}
			base += "?"
			args = append(args, id)
		}
		base += ")"
	}
	if visWhere, visArgs := buildVisibilityWhere(filter); visWhere != "" {
		base += visWhere
		args = append(args, visArgs...)
	}
	base += " ORDER BY " + scoreOrder + pinOrderPrefix + "p.last_active_at DESC, p.created_at DESC LIMIT ? OFFSET ?"
	if useFTS {
		args = append(args, keyword, keyword, keyword, limit, offset)
	} else {
		args = append(args, like, like, like, limit, offset)
	}
	var rows []listWithTotalRow
	err := r.db.DB().WithContext(ctx).Raw(base, args...).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	items := make([]model.PostListItem, len(rows))
	for i, row := range rows {
		items[i] = row.PostListItem
	}
	if len(rows) == 0 {
		return items, 0, nil
	}
	return items, int(rows[0].TotalCount), nil
}

func (r *sqlPostRepo) CountBySpaceID(ctx context.Context, spaceID string) (int, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).Model(&model.Post{}).
		Where("space_id = ? AND deleted_at IS NULL", spaceID).
		Count(&count).Error
	return int(count), err
}

// sitemapVisibleStatuses mirrors what the public timeline exposes: approved plus
// closed (closed topics stay readable, only new replies are locked).
const sitemapVisibleStatuses = "('approved','closed')"

// sitemapVisibilityWhere 是爬虫面的匿名可见性谓词：游客 = TL0，命中
// posts.min_level 覆盖与版面策略（visibleSpaceIDs 由 service 解析）。
func sitemapVisibilityWhere(visibleSpaceIDs []string) (string, []any) {
	cond := " AND COALESCE(p.min_level, 0) <= 0 AND (p.space_id = '' OR p.space_id IS NULL"
	args := []any{}
	if len(visibleSpaceIDs) > 0 {
		cond += " OR p.space_id IN ("
		for i, id := range visibleSpaceIDs {
			if i > 0 {
				cond += ","
			}
			cond += "?"
			args = append(args, id)
		}
		cond += ")"
	}
	cond += ")"
	return cond, args
}

func (r *sqlPostRepo) ListSitemapEntries(ctx context.Context, offset, limit int, visibleSpaceIDs []string) ([]SitemapEntry, error) {
	var rows []SitemapEntry
	visWhere, visArgs := sitemapVisibilityWhere(visibleSpaceIDs)
	args := append(visArgs, limit, offset)
	err := r.db.DB().WithContext(ctx).Raw(
		`SELECT p.id, p.slug, p.updated_at
		   FROM posts p
		  WHERE `+SQLSoftDeleteCondition("p")+` AND p.status IN `+sitemapVisibleStatuses+visWhere+`
		  ORDER BY p.updated_at DESC, p.id DESC
		  LIMIT ? OFFSET ?`, args...).Scan(&rows).Error
	if err != nil {
		return nil, WrapDBErr(err)
	}
	return rows, nil
}

func (r *sqlPostRepo) CountSitemapEntries(ctx context.Context, visibleSpaceIDs []string) (int, error) {
	var count int64
	visWhere, visArgs := sitemapVisibilityWhere(visibleSpaceIDs)
	err := r.db.DB().WithContext(ctx).Raw(
		`SELECT COUNT(*) FROM posts p
		  WHERE `+SQLSoftDeleteCondition("p")+` AND p.status IN `+sitemapVisibleStatuses+visWhere,
		visArgs...).Scan(&count).Error
	if err != nil {
		return 0, WrapDBErr(err)
	}
	return int(count), nil
}

func (r *sqlPostRepo) SetPin(ctx context.Context, id string, input SetPinInput) (*model.Post, error) {
	now := time.Now()
	if err := r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("id = ?", id).
		Updates(map[string]any{
			"pinned":           input.Pinned,
			"pin_order":        input.PinOrder,
			"pinned_at":        input.PinnedAt,
			"pinned_expire_at": input.PinnedExpireAt,
			"updated_at":       now,
		}).Error; err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *sqlPostRepo) Unpin(ctx context.Context, id string) (*model.Post, error) {
	return r.SetPin(ctx, id, SetPinInput{Pinned: false, PinOrder: 0, PinnedAt: nil, PinnedExpireAt: nil})
}

func (r *sqlPostRepo) ExpirePins(ctx context.Context) (int64, error) {
	result := r.db.DB().WithContext(ctx).Model(&model.Post{}).
		Where("pinned = true AND pinned_expire_at IS NOT NULL AND pinned_expire_at <= ?", time.Now()).
		Updates(map[string]any{
			"pinned": false, "pin_order": 0,
			"pinned_at": nil, "pinned_expire_at": nil,
		})
	return result.RowsAffected, result.Error
}

func (r *sqlPostRepo) ListPinnedBySpace(ctx context.Context, spaceID string, offset, limit int, vis PostListFilter) ([]model.PostListItem, int64, error) {
	visWhere, visArgs := buildVisibilityWhere(vis)
	spaceFilter := ""
	spaceArgs := []any{}
	if spaceID != "" {
		spaceFilter = "AND p.space_id = ?"
		spaceArgs = append(spaceArgs, spaceID)
	}

	var count int64
	countArgs := append(append([]any{}, spaceArgs...), visArgs...)
	if err := r.db.DB().WithContext(ctx).Raw(
		`SELECT COUNT(*) FROM posts p
		  WHERE p.pinned = true AND p.deleted_at IS NULL AND p.status IN ('approved','closed') `+spaceFilter+visWhere,
		countArgs...).Scan(&count).Error; err != nil {
		return nil, 0, err
	}

	args := append(append([]any{}, spaceArgs...), visArgs...)
	args = append(args, limit, offset)

	var items []model.PostListItem
	err := r.db.DB().WithContext(ctx).Raw(`
			SELECT p.id, p.title, p.slug, p.content_digest, p.author_id,
			       COALESCE(NULLIF(u.display_name, ''), u.username) AS author_name,
			       u.avatar AS author_avatar,
			       p.space_id, sp.name AS space_name,
			       p.status, p.min_level, p.pinned, p.pin_order, p.pinned_at, p.featured_at, p.locked, p.last_active_at, p.view_count,
			       p.comment_count, p.like_count, p.favorite_count,
			       p.created_at, p.updated_at
			FROM posts p
			LEFT JOIN users u ON p.author_id = u.id
			LEFT JOIN spaces sp ON p.space_id = sp.id
			WHERE p.pinned = true AND p.deleted_at IS NULL AND p.status IN ('approved','closed')
			`+spaceFilter+`
			ORDER BY p.pin_order ASC, COALESCE(p.pinned_at, '1970-01-01') DESC
			LIMIT ? OFFSET ?
		`, args...).Scan(&items).Error
	return items, count, err
}
