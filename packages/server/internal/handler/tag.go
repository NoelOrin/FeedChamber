package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type TagHandler interface {
	List(ctx iris.Context)
	Create(ctx iris.Context)
	Update(ctx iris.Context)
	Delete(ctx iris.Context)
	GetPostTags(ctx iris.Context)
	SetPostTags(ctx iris.Context)
}

type tagHandler struct{ svc service.TagService }

func newTagHandler(svc service.TagService) TagHandler {
	return &tagHandler{svc: svc}
}

// List 标签列表
// @Summary      标签列表
// @Description  获取所有标签列表
// @Tags         标签
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=[]model.Tag}
// @Failure      500  {object}  pkg.Response
// @Router       /tags/ [get]
func (h *tagHandler) List(ctx iris.Context) {
	tags, err := h.svc.List(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, tags)
}

// Create 创建标签
// @Summary      创建标签
// @Description  创建新标签
// @Tags         标签
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{name=string} true "标签信息"
// @Success      201  {object}  pkg.Response{data=model.Tag}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /tags/ [post]
func (h *tagHandler) Create(ctx iris.Context) {
	var body struct {
		Name string `json:"name"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	tag, err := h.svc.Create(ctx.Request().Context(), body.Name)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	ctx.StatusCode(iris.StatusCreated)
	pkg.Success(ctx, tag)
}

// Update 更新标签
// @Summary      更新标签
// @Description  更新指定标签的名称
// @Tags         标签
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "标签 ID"
// @Param        body body object{name=string} true "标签信息"
// @Success      200  {object}  pkg.Response{data=model.Tag}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /tags/{id} [put]
func (h *tagHandler) Update(ctx iris.Context) {
	var body struct {
		Name string `json:"name"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	tag, err := h.svc.Update(ctx.Request().Context(), ctx.Params().Get("id"), body.Name)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, tag)
}

// Delete 删除标签
// @Summary      删除标签
// @Description  删除指定标签
// @Tags         标签
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "标签 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      500  {object}  pkg.Response
// @Router       /tags/{id} [delete]
func (h *tagHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// GetPostTags 获取帖子标签
// @Summary      获取帖子标签
// @Description  获取指定帖子的所有标签
// @Tags         标签
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Success      200  {object}  pkg.Response{data=[]model.Tag}
// @Failure      500  {object}  pkg.Response
// @Router       /posts/{postId}/tags [get]
func (h *tagHandler) GetPostTags(ctx iris.Context) {
	tags, err := h.svc.GetTagsByPostID(ctx.Request().Context(), ctx.Params().Get("postId"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, tags)
}

// SetPostTags 设置帖子标签
// @Summary      设置帖子标签
// @Description  为指定帖子设置标签列表
// @Tags         标签
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Param        body   body      object{tags=[]string}  true  "标签名称列表"
// @Success      200  {object}  pkg.Response{data=[]model.Tag}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /posts/{postId}/tags [post]
func (h *tagHandler) SetPostTags(ctx iris.Context) {
	var body struct {
		Tags []string `json:"tags"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	postID := ctx.Params().Get("postId")
	if err := h.svc.SetPostTags(ctx.Request().Context(), postID, body.Tags); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	tags, err := h.svc.GetTagsByPostID(ctx.Request().Context(), postID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, tags)
}
