package handler

import (
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type TagHandler interface {
	List(ctx iris.Context)
	Create(ctx iris.Context)
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
// @Success      200  {object}  []model.Tag
// @Failure      500  {object}  object{error=string}
// @Router       /tags/ [get]
func (h *tagHandler) List(ctx iris.Context) {
	tags, err := h.svc.List(ctx.Request().Context())
	if err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.JSON(tags)
}

// Create 创建标签
// @Summary      创建标签
// @Description  创建新标签
// @Tags         标签
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{name=string} true "标签信息"
// @Success      201  {object}  model.Tag
// @Failure      400  {object}  object{error=string}
// @Failure      401  {object}  object{error=string}
// @Failure      403  {object}  object{error=string}
// @Router       /tags/ [post]
func (h *tagHandler) Create(ctx iris.Context) {
	var body struct {
		Name string `json:"name"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		ctx.StopWithError(iris.StatusBadRequest, err)
		return
	}
	tag, err := h.svc.Create(ctx.Request().Context(), body.Name)
	if err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.StatusCode(iris.StatusCreated)
	ctx.JSON(tag)
}

// Delete 删除标签
// @Summary      删除标签
// @Description  删除指定标签
// @Tags         标签
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "标签 ID"
// @Success      204  "No Content"
// @Failure      401  {object}  object{error=string}
// @Failure      403  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /tags/{id} [delete]
func (h *tagHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.StatusCode(iris.StatusNoContent)
}

// GetPostTags 获取帖子标签
// @Summary      获取帖子标签
// @Description  获取指定帖子的所有标签
// @Tags         标签
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Success      200  {object}  []model.Tag
// @Failure      500  {object}  object{error=string}
// @Router       /posts/{postId}/tags [get]
func (h *tagHandler) GetPostTags(ctx iris.Context) {
	tags, err := h.svc.GetTagsByPostID(ctx.Request().Context(), ctx.Params().Get("postId"))
	if err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.JSON(tags)
}

// SetPostTags 设置帖子标签
// @Summary      设置帖子标签
// @Description  为指定帖子设置标签列表
// @Tags         标签
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Param        body   body      object{tags=[]string}  true  "标签 ID 列表"
// @Success      200  {object}  []model.Tag
// @Failure      400  {object}  object{error=string}
// @Failure      401  {object}  object{error=string}
// @Failure      403  {object}  object{error=string}
// @Router       /posts/{postId}/tags [post]
func (h *tagHandler) SetPostTags(ctx iris.Context) {
	var body struct {
		Tags []string `json:"tags"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		ctx.StopWithError(iris.StatusBadRequest, err)
		return
	}
	postID := ctx.Params().Get("postId")
	if err := h.svc.SetPostTags(ctx.Request().Context(), postID, body.Tags); err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	tags, err := h.svc.GetTagsByPostID(ctx.Request().Context(), postID)
	if err != nil {
		ctx.StopWithJSON(iris.StatusInternalServerError, iris.Map{"error": err.Error()})
		return
	}
	ctx.JSON(tags)
}
