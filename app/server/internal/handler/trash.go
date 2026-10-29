package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type TrashHandler interface {
	ListPosts(ctx iris.Context)
	RestorePost(ctx iris.Context)
	ListComments(ctx iris.Context)
	RestoreComment(ctx iris.Context)
}

type trashHandler struct {
	posts    service.PostService
	contents service.ContentService
	audit    service.AuditLogService
}

func newTrashHandler(posts service.PostService, contents service.ContentService, audit service.AuditLogService) TrashHandler {
	return &trashHandler{posts: posts, contents: contents, audit: audit}
}

// 回收站帖子列表
// @Summary      回收站帖子列表
// @Description  分页列出软删除的帖子。
// @Tags         回收站
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{page=int,page_size=int} false "分页参数"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/trash/posts/list [post]
func (h *trashHandler) ListPosts(ctx iris.Context) {
	var body struct {
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
	}
	_ = ctx.ReadJSON(&body)
	items, total, err := h.posts.ListTrash(ctx.Request().Context(), body.Page, body.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, total)
}

// 恢复帖子
// @Summary      恢复帖子
// @Description  从回收站恢复帖子并写入审计日志。
// @Tags         回收站
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/trash/posts/{id}/restore [post]
func (h *trashHandler) RestorePost(ctx iris.Context) {
	id := ctx.Params().Get("id")
	if err := h.posts.RestoreTrash(ctx.Request().Context(), id); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), currentUserID(ctx), "trash.restore_post", id, "")
	pkg.Success(ctx, nil)
}

// 回收站评论列表
// @Summary      回收站评论列表
// @Description  分页列出软删除的评论。
// @Tags         回收站
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{page=int,page_size=int} false "分页参数"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/trash/comments/list [post]
func (h *trashHandler) ListComments(ctx iris.Context) {
	var body struct {
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
	}
	_ = ctx.ReadJSON(&body)
	items, total, err := h.contents.ListTrash(ctx.Request().Context(), body.Page, body.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, total)
}

// 恢复评论
// @Summary      恢复评论
// @Description  从回收站恢复评论并写入审计日志。
// @Tags         回收站
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/trash/comments/{id}/restore [post]
func (h *trashHandler) RestoreComment(ctx iris.Context) {
	id := ctx.Params().Get("id")
	if err := h.contents.RestoreTrash(ctx.Request().Context(), id); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), currentUserID(ctx), "trash.restore_comment", id, "")
	pkg.Success(ctx, nil)
}
