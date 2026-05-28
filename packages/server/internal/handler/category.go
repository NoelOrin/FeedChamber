package handler

import (
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type CategoryHandler interface {
	List(ctx iris.Context)
	Create(ctx iris.Context)
	Delete(ctx iris.Context)
}

type categoryHandler struct{ svc service.CategoryService }

func newCategoryHandler(svc service.CategoryService) CategoryHandler {
	return &categoryHandler{svc: svc}
}

func (h *categoryHandler) List(ctx iris.Context) {
	cats, err := h.svc.List(ctx.Request().Context())
	if err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.JSON(cats)
}

func (h *categoryHandler) Create(ctx iris.Context) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		ctx.StopWithError(iris.StatusBadRequest, err)
		return
	}
	cat, err := h.svc.Create(ctx.Request().Context(), body.Name, body.Description)
	if err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.StatusCode(iris.StatusCreated)
	ctx.JSON(cat)
}

func (h *categoryHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.StatusCode(iris.StatusNoContent)
}