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

// List 分类列表
// @Summary      分类列表
// @Description  获取所有分类列表
// @Tags         分类
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  []model.Category
// @Failure      500  {object}  object{error=string}
// @Router       /categories/ [get]
func (h *categoryHandler) List(ctx iris.Context) {
	cats, err := h.svc.List(ctx.Request().Context())
	if err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.JSON(cats)
}

// Create 创建分类
// @Summary      创建分类
// @Description  创建新分类
// @Tags         分类
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{name=string,description=string} true "分类信息"
// @Success      201  {object}  model.Category
// @Failure      400  {object}  object{error=string}
// @Failure      401  {object}  object{error=string}
// @Failure      403  {object}  object{error=string}
// @Router       /categories/ [post]
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

// Delete 删除分类
// @Summary      删除分类
// @Description  删除指定分类
// @Tags         分类
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "分类 ID"
// @Success      204  "No Content"
// @Failure      401  {object}  object{error=string}
// @Failure      403  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /categories/{id} [delete]
func (h *categoryHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.StatusCode(iris.StatusNoContent)
}