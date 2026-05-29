package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type CommentHandler interface {
	ListByPost(ctx iris.Context)
	Create(ctx iris.Context)
	Update(ctx iris.Context)
	Delete(ctx iris.Context)
}

type commentHandler struct{ svc service.CommentService }

func newCommentHandler(svc service.CommentService) CommentHandler {
	return &commentHandler{svc: svc}
}

// ListByPost 帖子评论列表
// @Summary      帖子评论列表
// @Description  获取指定帖子的所有评论
// @Tags         评论
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Success      200  {object}  []model.Comment
// @Failure      500  {object}  object{error=string}
// @Router       /posts/{postId}/comments/ [get]
func (h *commentHandler) ListByPost(ctx iris.Context) {
	comments, err := h.svc.ListByPost(ctx.Request().Context(), ctx.Params().Get("postId"))
	if err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.JSON(comments)
}

// Create 创建评论
// @Summary      创建评论
// @Description  在指定帖子下创建评论，可选父评论 ID 实现嵌套回复
// @Tags         评论
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Param        body   body      object{content=string,parentId=string}  true  "评论内容"
// @Success      201  {object}  model.Comment
// @Failure      400  {object}  object{error=string}
// @Failure      401  {object}  object{error=string}
// @Failure      403  {object}  object{error=string}
// @Router       /posts/{postId}/comments/ [post]
func (h *commentHandler) Create(ctx iris.Context) {
	var body struct {
		Content  string `json:"content"`
		ParentID string `json:"parentId"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		ctx.StopWithError(iris.StatusBadRequest, err)
		return
	}
	userID := ctx.Values().GetString("user_uuid")
	c, err := h.svc.Create(ctx.Request().Context(), ctx.Params().Get("postId"), userID, body.Content, body.ParentID)
	if err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.StatusCode(iris.StatusCreated)
	ctx.JSON(c)
}

// Update 编辑评论
// @Summary      编辑评论
// @Description  编辑指定评论的内容
// @Tags         评论
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Param        id     path      string  true  "评论 ID"
// @Param        body   body      object{content=string}  true  "评论内容"
// @Success      200    {object}  pkg.Response{data=model.Comment}
// @Failure      400    {object}  pkg.Response
// @Failure      401    {object}  pkg.Response
// @Failure      404    {object}  pkg.Response
// @Router       /posts/{postId}/comments/{id} [put]
func (h *commentHandler) Update(ctx iris.Context) {
	var body struct {
		Content string `json:"content"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	c, err := h.svc.Update(ctx.Request().Context(), ctx.Params().Get("postId"), ctx.Params().Get("id"), body.Content)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, c)
}

// Delete 删除评论
// @Summary      删除评论
// @Description  删除指定评论
// @Tags         评论
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Param        id     path      string  true  "评论 ID"
// @Success      204  "No Content"
// @Failure      401  {object}  object{error=string}
// @Failure      403  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /posts/{postId}/comments/{id} [delete]
func (h *commentHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.StatusCode(iris.StatusNoContent)
}