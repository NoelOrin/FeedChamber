package handler

import (
	"orinforum/internal/service"
	"strconv"

	"github.com/kataras/iris/v12"
)

type PostHandler interface {
	List(ctx iris.Context)
	GetByID(ctx iris.Context)
	Create(ctx iris.Context)
	Update(ctx iris.Context)
	Delete(ctx iris.Context)
}

type postHandler struct{ svc service.PostService }

func newPostHandler(svc service.PostService) PostHandler {
	return &postHandler{svc: svc}
}

func (h *postHandler) List(ctx iris.Context) {
	page, _ := strconv.Atoi(ctx.URLParamDefault("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.URLParamDefault("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	posts, err := h.svc.List(ctx.Request().Context(), page, pageSize)
	if err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.JSON(posts)
}

func (h *postHandler) GetByID(ctx iris.Context) {
	post, err := h.svc.GetByID(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		ctx.StopWithJSON(iris.StatusNotFound, iris.Map{"error": "post not found"})
		return
	}
	ctx.JSON(post)
}

func (h *postHandler) Create(ctx iris.Context) {
	var body struct {
		Title      string `json:"title"`
		Content    string `json:"content"`
		CategoryID string `json:"categoryId"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		ctx.StopWithError(iris.StatusBadRequest, err)
		return
	}
	userID := ctx.Values().GetString("user_id")
	post, err := h.svc.Create(ctx.Request().Context(), userID, body.Title, body.Content, body.CategoryID)
	if err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.StatusCode(iris.StatusCreated)
	ctx.JSON(post)
}

func (h *postHandler) Update(ctx iris.Context) {
	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		ctx.StopWithError(iris.StatusBadRequest, err)
		return
	}
	post, err := h.svc.Update(ctx.Request().Context(), ctx.Params().Get("id"), body.Title, body.Content)
	if err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.JSON(post)
}

func (h *postHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.StatusCode(iris.StatusNoContent)
}