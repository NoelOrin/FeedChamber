package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type RevisionHandler interface {
	ListPost(ctx iris.Context)
	RestorePost(ctx iris.Context)
	ListComment(ctx iris.Context)
	RestoreComment(ctx iris.Context)
}

type revisionHandler struct{ svc service.RevisionService }

func newRevisionHandler(svc service.RevisionService) RevisionHandler {
	return &revisionHandler{svc: svc}
}

// 帖子修订历史
// @Summary      帖子修订历史
// @Description  分页列出该帖子的历史版本。
// @Tags         修订
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Param        body body object{page=int,page_size=int} false "分页参数"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /posts/{id}/revisions/list [post]
func (h *revisionHandler) ListPost(ctx iris.Context) {
	p, err := readListParams(ctx)
	if err != nil {
		return
	}
	items, total, err := h.svc.List(ctx.Request().Context(), "post", ctx.Params().Get("id"), p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, total)
}

// 恢复帖子版本
// @Summary      恢复帖子版本
// @Description  把帖子内容回滚到指定历史版本。
// @Tags         修订
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Param        revisionId path string true "修订记录 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /posts/{id}/revisions/{revisionId}/restore [post]
func (h *revisionHandler) RestorePost(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	post, err := h.svc.RestorePost(ctx.Request().Context(), ctx.Params().Get("revisionId"), u.ID, u.Role)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, post)
}

// 评论修订历史
// @Summary      评论修订历史
// @Description  分页列出该评论的历史版本。
// @Tags         修订
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Param        body body object{page=int,page_size=int} false "分页参数"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /comments/{id}/revisions/list [post]
func (h *revisionHandler) ListComment(ctx iris.Context) {
	p, err := readListParams(ctx)
	if err != nil {
		return
	}
	items, total, err := h.svc.List(ctx.Request().Context(), "comment", ctx.Params().Get("id"), p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, total)
}

// 恢复评论版本
// @Summary      恢复评论版本
// @Description  把评论内容回滚到指定历史版本。
// @Tags         修订
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Param        revisionId path string true "修订记录 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /comments/{id}/revisions/{revisionId}/restore [post]
func (h *revisionHandler) RestoreComment(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	content, err := h.svc.RestoreComment(ctx.Request().Context(), ctx.Params().Get("revisionId"), u.ID, u.Role)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, content)
}
