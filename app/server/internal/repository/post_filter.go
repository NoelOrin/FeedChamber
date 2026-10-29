package repository

import (
	"time"

	"feedchamber/internal/model"
)

// StatusFilterAll is the list-filter sentinel meaning "do not filter by status".
// Only viewers allowed to see non-public rows may use it (the service gates it
// behind canViewNonPublic); buildPostListWhere ignores it defensively so the
// sentinel can never be spliced into SQL as a literal status value
// (WHERE p.status = '__all__' used to match zero rows).
const StatusFilterAll = "__all__"

type PostListFilter struct {
	TagName          string
	AuthorID         string
	Status           string
	StatusIn         []string
	StartTime        *time.Time
	EndTime          *time.Time
	SpaceName        string
	SpaceSlug        string
	SpaceID          string
	SpaceIDs         []string
	SortBy           string
	ExcludeAuthorIDs []string
	ListView         string
	// ExcludeExiled 主推荐流排除暗区软隔离内容；按版面精确浏览（如进入暗区）时不置位。
	ExcludeExiled bool
	ViewerID      string
	ReadStates    []model.TopicReadState

	// 可见性谓词（P2）。EnforceVisibility 为 true 时，以下三个条件与状态过滤
	// 一起构成「这个访问者能看见哪些帖子」的唯一定义：
	//   - 帖子覆盖：p.min_level ≤ ViewerLevel（0 = 继承版面）
	//   - 版面策略：p.space_id 落在 VisibleSpaceIDs 内（空 space_id 视为公开）；
	//     VisibleSpaceIDs 由 service 按祖先继承解析后传入，SQL 不做递归。
	//   - 作者例外：p.author_id = ViewerID（作者始终可读自己的帖子）
	// 游客 = ViewerLevel 0、LoggedIn=false，由 VisibleSpaceIDs 的解析方保证。
	EnforceVisibility bool
	ViewerLevel       int
	VisibleSpaceIDs   []string
}

func buildPostListWhere(f PostListFilter) (string, []interface{}) {
	query := ""
	args := []interface{}{}
	if len(f.SpaceIDs) > 0 {
		query += " AND p.space_id IN ("
		for i, id := range f.SpaceIDs {
			if i > 0 {
				query += ","
			}
			query += "?"
			args = append(args, id)
		}
		query += ")"
	} else if f.SpaceID != "" {
		query += " AND p.space_id = ?"
		args = append(args, f.SpaceID)
	} else {
		if f.SpaceName != "" {
			query += " AND sp.name = ?"
			args = append(args, f.SpaceName)
		}
		if f.SpaceSlug != "" {
			query += " AND sp.slug = ?"
			args = append(args, f.SpaceSlug)
		}
	}
	if f.ExcludeExiled {
		query += " AND p.exiled = ?"
		args = append(args, false)
	}
	if f.TagName != "" {
		query += " AND EXISTS (SELECT 1 FROM post_tags pt INNER JOIN tags t ON pt.tag_id = t.id WHERE pt.post_id = p.id AND t.name = ?)"
		args = append(args, f.TagName)
	}
	if f.AuthorID != "" {
		query += " AND p.author_id = ?"
		args = append(args, f.AuthorID)
	}
	switch f.ListView {
	case "new":
		query += " AND NOT EXISTS (SELECT 1 FROM topic_read_states trs WHERE trs.post_id = p.id AND trs.user_id = ?)"
		args = append(args, f.ViewerID)
	case "read":
		query += " AND EXISTS (SELECT 1 FROM topic_read_states trs WHERE trs.post_id = p.id AND trs.user_id = ? AND p.last_active_at <= trs.last_read_at)"
		args = append(args, f.ViewerID)
	case "unread":
		query += " AND EXISTS (SELECT 1 FROM topic_read_states trs WHERE trs.post_id = p.id AND trs.user_id = ? AND p.last_active_at > trs.last_read_at)"
		args = append(args, f.ViewerID)
	case "posted":
		query += " AND p.author_id = ?"
		args = append(args, f.ViewerID)
	}
	if len(f.ExcludeAuthorIDs) > 0 {
		query += " AND p.author_id NOT IN ("
		for i, id := range f.ExcludeAuthorIDs {
			if i > 0 {
				query += ","
			}
			query += "?"
			args = append(args, id)
		}
		query += ")"
	}
	if len(f.StatusIn) > 0 {
		query += " AND p.status IN ("
		for i, st := range f.StatusIn {
			if i > 0 {
				query += ","
			}
			query += "?"
			args = append(args, st)
		}
		query += ")"
	} else if f.Status != "" && f.Status != StatusFilterAll {
		query += " AND p.status = ?"
		args = append(args, f.Status)
	}
	if visWhere, visArgs := buildVisibilityWhere(f); visWhere != "" {
		query += visWhere
		args = append(args, visArgs...)
	}
	if f.StartTime != nil {
		query += " AND p.created_at >= ?"
		args = append(args, *f.StartTime)
	}
	if f.EndTime != nil {
		query += " AND p.created_at <= ?"
		args = append(args, *f.EndTime)
	}
	return query, args
}

// buildVisibilityWhere 是可见性谓词的单一 SQL 实现（别名固定为 p）。
func buildVisibilityWhere(f PostListFilter) (string, []any) {
	if !f.EnforceVisibility {
		return "", nil
	}
	spaceCond := "(p.space_id = '' OR p.space_id IS NULL)"
	if len(f.VisibleSpaceIDs) > 0 {
		spaceCond = "(p.space_id = '' OR p.space_id IS NULL OR p.space_id IN ("
		for i := range f.VisibleSpaceIDs {
			if i > 0 {
				spaceCond += ","
			}
			spaceCond += "?"
		}
		spaceCond += "))"
	}
	args := []any{}
	cond := " AND ("
	if f.ViewerID != "" {
		cond += "p.author_id = ? OR "
		args = append(args, f.ViewerID)
	}
	cond += "COALESCE(p.min_level, 0) <= ? AND " + spaceCond + ")"
	args = append(args, f.ViewerLevel)
	for _, id := range f.VisibleSpaceIDs {
		args = append(args, id)
	}
	return cond, args
}
