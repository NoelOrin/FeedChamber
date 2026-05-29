package handler

import (
	"orinforum/internal/pkg"
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
	Search(ctx iris.Context)
	TogglePin(ctx iris.Context)
	ToggleLock(ctx iris.Context)
}

type postHandler struct{ svc service.PostService }

func newPostHandler(svc service.PostService) PostHandler {
	return &postHandler{svc: svc}
}

// List 帖子列表
// @Summary      帖子列表
// @Description  分页获取帖子列表
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page     query  int  false  "页码"  default(1)
// @Param        pageSize query  int  false  "每页数量"  default(20)
// @Success      200  {object}  pkg.Response{data=[]model.PostListItem}
// @Failure      401  {object}  pkg.Response
// @Router       /posts/ [get]
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
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, posts)
}

// GetByID 获取帖子详情
// @Summary      获取帖子详情
// @Description  根据帖子 ID 获取帖子详情，自动增加浏览量
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "帖子 ID"
// @Success      200  {object}  pkg.Response{data=model.PostListItem}
// @Failure      401  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /posts/{id} [get]
func (h *postHandler) GetByID(ctx iris.Context) {
	post, err := h.svc.GetByID(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, post)
}

// Create 创建帖子
// @Summary      创建帖子
// @Description  创建新帖子
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{title=string,content=string,categoryId=string} true "帖子信息"
// @Success      201  {object}  pkg.Response{data=model.Post}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /posts/ [post]
func (h *postHandler) Create(ctx iris.Context) {
	var body struct {
		Title      string `json:"title"`
		Content    string `json:"content"`
		CategoryID string `json:"categoryId"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	userID := ctx.Values().GetString("user_uuid")
	post, err := h.svc.Create(ctx.Request().Context(), userID, body.Title, body.Content, body.CategoryID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	ctx.StatusCode(iris.StatusCreated)
	pkg.Success(ctx, post)
}

// Update 更新帖子
// @Summary      更新帖子
// @Description  更新指定帖子的标题和内容
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "帖子 ID"
// @Param        body body object{title=string,content=string} true "更新信息"
// @Success      200  {object}  pkg.Response{data=model.Post}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /posts/{id} [put]
func (h *postHandler) Update(ctx iris.Context) {
	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	post, err := h.svc.Update(ctx.Request().Context(), ctx.Params().Get("id"), body.Title, body.Content)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, post)
}

// Delete 删除帖子
// @Summary      删除帖子
// @Description  删除指定帖子
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "帖子 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /posts/{id} [delete]
func (h *postHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// Search 搜索帖子
// @Summary      搜索帖子
// @Description  根据关键词搜索帖子
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        q        query  string  true   "搜索关键词"
// @Param        page     query  int     false  "页码"  default(1)
// @Param        pageSize query  int     false  "每页数量"  default(20)
// @Success      200  {object}  pkg.Response{data=[]model.PostListItem}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /posts/search [get]
func (h *postHandler) Search(ctx iris.Context) {
	q := ctx.URLParam("q")
	if q == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "search keyword is required")
		return
	}
	page, _ := strconv.Atoi(ctx.URLParamDefault("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.URLParamDefault("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	posts, err := h.svc.Search(ctx.Request().Context(), q, page, pageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, posts)
}

// TogglePin 切换帖子置顶
// @Summary      切换帖子置顶
// @Description  切换指定帖子的置顶状态
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "帖子 ID"
// @Success      200  {object}  pkg.Response{data=model.Post}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /posts/{id}/pin [put]
func (h *postHandler) TogglePin(ctx iris.Context) {
	post, err := h.svc.TogglePin(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, post)
}

// ToggleLock 切换帖子锁定
// @Summary      切换帖子锁定
// @Description  切换指定帖子的锁定状态
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "帖子 ID"
// @Success      200  {object}  pkg.Response{data=model.Post}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /posts/{id}/lock [put]
func (h *postHandler) ToggleLock(ctx iris.Context) {
	post, err := h.svc.ToggleLock(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, post)
}