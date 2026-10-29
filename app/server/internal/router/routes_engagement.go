package router

import (
	"github.com/kataras/iris/v12"
)

// engagementRoutes 用户互动：收藏列表、评论管理端读删与版本、点赞与回复、帖子订阅、关注与拉黑。
func (r *reg) engagementRoutes() {
	protected, h := r.protected, r.h

	// ── Favorites list ────────────────────────────────────────
	protected.PartyFunc("/favorites", func(f iris.Party) {
		f.Post("/list", h.Favorite.ListByUser)
	})

	// ── Comments (admin delete only) ────────────────────────────
	protected.PartyFunc("/comments", func(c iris.Party) {
		c.Get("/{id}", h.Content.AdminGet)
		c.Post("/{id}/delete", h.Content.AdminDelete)
		c.Post("/{id}/revisions/list", h.Revision.ListComment)
		c.Post("/{id}/revisions/{revisionId}/restore", h.Revision.RestoreComment)
	})

	// ── Content standalone ────────────────────────────────────
	protected.PartyFunc("/contents/{contentId}", func(c iris.Party) {
		c.Post("/like", h.Like.ToggleContentLike)
		c.Post("/like/status", h.Like.GetContentLikeStatus)
		c.Post("/replies", h.Content.ListReplies)
	})

	// ── Watches ───────────────────────────────────────────────
	protected.PartyFunc("/watches", func(w iris.Party) {
		w.Post("/set", h.Watch.Set)
		w.Post("/list", h.Watch.List)
		w.Post("/delete", h.Watch.Delete)
		w.Post("/status", h.Watch.GetStatus)
	})

	// ── Follow ────────────────────────────────────────────────
	protected.PartyFunc("/users/{userId}", func(u iris.Party) {
		u.Post("/follow", h.Follow.Follow)
		u.Post("/unfollow", h.Follow.Unfollow)
		u.Post("/follow/status", h.Follow.GetStatus)
		u.Post("/followers", h.Follow.GetFollowers)
		u.Post("/following", h.Follow.GetFollowing)
		u.Post("/counts", h.Follow.GetCounts)
		u.Post("/block", h.UserBlock.Block)
		u.Post("/unblock", h.UserBlock.Unblock)
		u.Post("/block/status", h.UserBlock.Status)
	})

	protected.PartyFunc("/blocks", func(b iris.Party) {
		b.Post("/list", h.UserBlock.List)
	})
}

// uploadRoutes 文件上传与预签名（按 key 公开读取在 publicRoutes 里）。
func (r *reg) uploadRoutes() {
	protected, h := r.protected, r.h

	// ── Upload ────────────────────────────────────────────────
	protected.PartyFunc("/upload", func(u iris.Party) {
		u.Post("", h.Upload.Upload)
		u.Post("/presign", h.Upload.Presign)
		u.Post("/confirm", h.Upload.Confirm)
		u.Post("/list", h.Upload.List)
		u.Get("/{id}", h.Upload.GetFile)
		u.Post("/{id}/delete", h.Upload.Delete)
	})
}
