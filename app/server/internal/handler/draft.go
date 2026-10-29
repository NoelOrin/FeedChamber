package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type DraftHandler interface {
	Save(ctx iris.Context)
	List(ctx iris.Context)
	Delete(ctx iris.Context)
}

type draftHandler struct{ svc service.DraftService }

func newDraftHandler(svc service.DraftService) DraftHandler {
	return &draftHandler{svc: svc}
}

// 保存草稿
// @Summary      保存草稿
// @Description  保存帖子或评论草稿；重复保存同一目标会覆盖。
// @Tags         草稿
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{kind=string,target_id=string,title=string,content=string,space_id=string,tags=[]string} true "草稿内容"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /drafts/save [post]
func (h *draftHandler) Save(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	var body struct {
		Kind     string   `json:"kind"`
		TargetID string   `json:"target_id"`
		Title    string   `json:"title"`
		Content  string   `json:"content"`
		SpaceID  string   `json:"space_id"`
		Tags     []string `json:"tags"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	draft, err := h.svc.Save(ctx.Request().Context(), u.ID, body.Kind, body.TargetID, body.Title, body.Content, body.SpaceID, body.Tags)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, draft)
}

// 草稿列表
// @Summary      草稿列表
// @Description  分页列出当前用户的草稿。
// @Tags         草稿
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{page=int,page_size=int} false "分页参数"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /drafts/list [post]
func (h *draftHandler) List(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	p, err := readListParams(ctx)
	if err != nil {
		return
	}
	items, total, err := h.svc.List(ctx.Request().Context(), u.ID, p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, total)
}

// 删除草稿
// @Summary      删除草稿
// @Description  删除当前用户的一条草稿。
// @Tags         草稿
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /drafts/{id}/delete [post]
func (h *draftHandler) Delete(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	if err := h.svc.Delete(ctx.Request().Context(), u.ID, ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
