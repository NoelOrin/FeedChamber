package handler

import (
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type CommentHandler interface {
	ListByPost(ctx iris.Context)
	Create(ctx iris.Context)
	Delete(ctx iris.Context)
}

type commentHandler struct{ svc service.CommentService }

func newCommentHandler(svc service.CommentService) CommentHandler {
	return &commentHandler{svc: svc}
}

func (h *commentHandler) ListByPost(ctx iris.Context) {
	comments, err := h.svc.ListByPost(ctx.Request().Context(), ctx.Params().Get("postId"))
	if err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.JSON(comments)
}

func (h *commentHandler) Create(ctx iris.Context) {
	var body struct {
		Content  string `json:"content"`
		ParentID string `json:"parentId"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		ctx.StopWithError(iris.StatusBadRequest, err)
		return
	}
	userID := ctx.Values().GetString("user_id")
	c, err := h.svc.Create(ctx.Request().Context(), ctx.Params().Get("postId"), userID, body.Content, body.ParentID)
	if err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.StatusCode(iris.StatusCreated)
	ctx.JSON(c)
}

func (h *commentHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.StatusCode(iris.StatusNoContent)
}