package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type CategoryHandler interface {
	List(ctx iris.Context)
	Create(ctx iris.Context)
	Update(ctx iris.Context)
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
// @Success      200  {object}  pkg.Response{data=[]model.Category}
// @Failure      500  {object}  pkg.Response
// @Router       /categories/ [get]
func (h *categoryHandler) List(ctx iris.Context) {
	var body struct {
		ListParams
	}
	_ = ctx.ReadJSON(&body)
	body.Normalize()
	cats, total, err := h.svc.List(ctx.Request().Context(), body.Page, body.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, cats, total)
}

// Create 创建分类
// @Summary      创建分类
// @Description  创建新分类
// @Tags         分类
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{name=string,description=string} true "分类信息"
// @Success      201  {object}  pkg.Response{data=model.Category}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /categories/ [post]
func (h *categoryHandler) Create(ctx iris.Context) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	cat, err := h.svc.Create(ctx.Request().Context(), body.Name, body.Description)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	ctx.StatusCode(iris.StatusCreated)
	pkg.Success(ctx, cat)
}

// Update 更新分类
// @Summary      更新分类
// @Description  更新指定分类的名称和描述
// @Tags         分类
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "分类 ID"
// @Param        body body object{name=string,description=string} true "分类信息"
// @Success      200  {object}  pkg.Response{data=model.Category}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /categories/{id} [put]
func (h *categoryHandler) Update(ctx iris.Context) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	cat, err := h.svc.Update(ctx.Request().Context(), ctx.Params().Get("id"), body.Name, body.Description)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, cat)
}

// Delete 删除分类
// @Summary      删除分类
// @Description  删除指定分类
// @Tags         分类
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "分类 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      500  {object}  pkg.Response
// @Router       /categories/{id} [delete]
func (h *categoryHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
