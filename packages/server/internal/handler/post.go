package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

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
// @Description  分页获取帖子列表，支持分类/作者/状态/时间筛选
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{page=int,pageSize=int,categoryId=string,authorId=string,status=string,startTime=string,endTime=string} false "筛选参数"
// @Success      200  {object}  pkg.Response{data=[]model.PostListItem}
// @Failure      401  {object}  pkg.Response
// @Router       /posts/list [post]
func (h *postHandler) List(ctx iris.Context) {
	var body struct {
		ListParams
		CategoryName string `json:"categoryName"`
		TagName      string `json:"tagName"`
		AuthorID     string `json:"authorId"`
		Status       string `json:"status"`
	}
	_ = ctx.ReadJSON(&body)
	body.Normalize()
	st, et := readTimeRange(ctx, body.ListParams)
	posts, total, err := h.svc.List(ctx.Request().Context(), body.Page, body.PageSize, body.CategoryName, body.TagName, body.AuthorID, body.Status, st, et)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, posts, total)
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
// @Router       /posts/create [post]
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
// @Router       /posts/{id}/update [post]
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
// @Router       /posts/{id}/delete [post]
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
// @Param        body body object{q=string,page=int,pageSize=int} true "搜索参数"
// @Success      200  {object}  pkg.Response{data=[]model.PostListItem}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /posts/search [post]
func (h *postHandler) Search(ctx iris.Context) {
	var body struct {
		ListParams
		Q string `json:"q"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if body.Q == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "搜索关键词不能为空")
		return
	}
	body.Normalize()
	posts, total, err := h.svc.Search(ctx.Request().Context(), body.Q, body.Page, body.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, posts, total)
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
// @Router       /posts/{id}/pin [post]
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
// @Router       /posts/{id}/lock [post]
func (h *postHandler) ToggleLock(ctx iris.Context) {
	post, err := h.svc.ToggleLock(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, post)
}
