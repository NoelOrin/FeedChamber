package router

import (
	"time"

	"feedchamber/internal/middleware"
	"feedchamber/internal/model"

	"github.com/kataras/iris/v12"
)

// tagManageRoutes 标签的写操作（标签列表在 publicOptionalRoutes 里）。
func (r *reg) tagManageRoutes() {
	protected, h := r.protected, r.h

	// ── Tag ───────────────────────────────────────────────────
	protected.PartyFunc("/tags", func(t iris.Party) {
		t.Post("/create", h.Tag.Create)
		t.Post("/{id}/update", h.Tag.Update)
		t.Post("/{id}/delete", h.Tag.Delete)
	})
}

// postRoutes 帖子与评论：列表/搜索/详情/评论列表走 v1 + 可选登录门控，写操作与置顶、加锁、版本、收藏、
// 管理端评论列表走 protected。
func (r *reg) postRoutes() {
	v1, protected, h, deps := r.v1, r.protected, r.h, r.deps

	// ── Post ──────────────────────────────────────────────────
	// list/search/get are public by default; gates enforce auth when setting is enabled
	v1.PartyFunc("/posts", func(p iris.Party) {
		p.Post("/list", middleware.AuthGate(deps, model.SettingPostAuthRequired), h.Post.List)
		p.Post("/search", middleware.AuthGate(deps, model.SettingPostAuthRequired), h.Post.Search)
		p.Get("/{id}", middleware.AuthGate(deps, model.SettingPostAuthRequired), h.Post.GetByID)
		p.Get("/{id}/detail", middleware.AuthGate(deps, model.SettingPostAuthRequired), h.Post.GetDetail)
		p.Post("/{id}/suggestions", h.Post.Suggestions)

		p.Post("/{postId}/contents/list", middleware.AuthGate(deps, model.SettingCommentAuthRequired), h.Content.ListByPost)
		p.Post("/{postId}/tags", h.Tag.GetPostTags)
	})

	protected.PartyFunc("/posts", func(p iris.Party) {
		p.Post("/batch", h.Post.ListByIDs)
		p.Post("/create", middleware.RateLimit(deps.Limiter, "post_create", 10, time.Minute), h.Post.Create)
		p.Post("/{id}/update", h.Post.Update)
		p.Post("/{id}/delete", h.Post.Delete)
		p.Post("/{id}/pin", h.Post.TogglePin)
		p.Post("/{id}/lock", h.Post.ToggleLock)
		p.Post("/{id}/close", h.Post.SetClosed)
		// 状态直改（审核 / 救回无审核记录的存量待审帖）；网关映射到 report:manage
		p.Post("/{id}/status", h.Post.SetStatus)
		// 设为/取消精华（后台帖子管理）；网关映射到 report:manage
		p.Post("/{id}/feature", h.Post.SetFeatured)
		p.Post("/{id}/read", h.Post.MarkRead)
		p.Post("/{id}/pin/set", h.Post.SetPin)
		p.Post("/{id}/pin/remove", h.Post.Unpin)
		p.Post("/{id}/revisions/list", h.Revision.ListPost)
		p.Post("/{id}/revisions/{revisionId}/restore", h.Revision.RestorePost)

		p.Post("/{postId}/tags/set", h.Tag.SetPostTags)

		p.Post("/{postId}/contents/create", middleware.RateLimit(deps.Limiter, "comment_create", 30, time.Minute), h.Content.Create)
		p.Post("/{postId}/contents/{id}/update", h.Content.Update)
		p.Post("/{postId}/contents/{id}/delete", h.Content.Delete)

		p.Post("/{postId}/favorite", h.Favorite.Toggle)
		p.Post("/{postId}/favorite/status", h.Favorite.GetStatus)

		// admin: global comment list
		p.Post("/comments/list", h.Content.ListAll)
	})

	// ── Pinned posts list ────────────────────────────────────
	protected.Post("/posts/pinned/list", h.Post.ListPinned)
}
