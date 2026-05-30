package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type ContentHandler interface {
	ListByPost(ctx iris.Context)
	ListReplies(ctx iris.Context)
	Create(ctx iris.Context)
	Update(ctx iris.Context)
	Delete(ctx iris.Context)
}

type contentHandler struct{ svc service.ContentService }

func newContentHandler(svc service.ContentService) ContentHandler {
	return &contentHandler{svc: svc}
}

// ListByPost 帖子评论列表
// @Summary      帖子评论列表
// @Description  获取指定帖子下的所有评论（任意深度）
// @Tags         内容
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Success      200  {object}  []model.Content
// @Failure      500  {object}  object{error=string}
// @Router       /posts/{postId}/contents/ [get]
func (h *contentHandler) ListByPost(ctx iris.Context) {
	contents, err := h.svc.ListByPost(ctx.Request().Context(), ctx.Params().Get("postId"))
	if err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.JSON(contents)
}

// ListReplies 子回复列表
// @Summary      子回复列表
// @Description  获取指定内容的直接子回复
// @Tags         内容
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path      string  true  "内容 ID"
// @Success      200  {object}  []model.Content
// @Failure      500  {object}  object{error=string}
// @Router       /contents/{id}/replies [get]
func (h *contentHandler) ListReplies(ctx iris.Context) {
	contents, err := h.svc.ListReplies(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.JSON(contents)
}

// Create 创建评论/回复
// @Summary      创建评论/回复
// @Description  在指定帖子下创建评论或回复
// @Tags         内容
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Param        body   body      object{body=string,parentId=string}  true  "内容"
// @Success      201  {object}  model.Content
// @Failure      400  {object}  object{error=string}
// @Failure      401  {object}  object{error=string}
// @Router       /posts/{postId}/contents/ [post]
func (h *contentHandler) Create(ctx iris.Context) {
	var body struct {
		Body     string `json:"body"`
		ParentID string `json:"parentId"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		ctx.StopWithError(iris.StatusBadRequest, err)
		return
	}
	userID := ctx.Values().GetString("user_uuid")
	c, err := h.svc.Create(ctx.Request().Context(), ctx.Params().Get("postId"), userID, body.Body, body.ParentID)
	if err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.StatusCode(iris.StatusCreated)
	ctx.JSON(c)
}

// Update 编辑内容
// @Summary      编辑内容
// @Description  编辑指定内容
// @Tags         内容
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Param        id     path      string  true  "内容 ID"
// @Param        body   body      object{body=string}  true  "内容"
// @Success      200    {object}  pkg.Response{data=model.Content}
// @Failure      400    {object}  pkg.Response
// @Failure      401    {object}  pkg.Response
// @Failure      404    {object}  pkg.Response
// @Router       /posts/{postId}/contents/{id} [put]
func (h *contentHandler) Update(ctx iris.Context) {
	var body struct {
		Body string `json:"body"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	c, err := h.svc.Update(ctx.Request().Context(), ctx.Params().Get("postId"), ctx.Params().Get("id"), body.Body)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, c)
}

// Delete 删除内容
// @Summary      删除内容
// @Description  删除指定内容
// @Tags         内容
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Param        id     path      string  true  "内容 ID"
// @Success      204  "No Content"
// @Failure      401  {object}  object{error=string}
// @Failure      403  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /posts/{postId}/contents/{id} [delete]
func (h *contentHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("postId"), ctx.Params().Get("id")); err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.StatusCode(iris.StatusNoContent)
}
