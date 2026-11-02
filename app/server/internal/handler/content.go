package handler

import (
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

type ContentHandler interface {
	ListByPost(ctx iris.Context)
	ListReplies(ctx iris.Context)
	ListAll(ctx iris.Context)
	AdminGet(ctx iris.Context)
	Create(ctx iris.Context)
	Update(ctx iris.Context)
	Delete(ctx iris.Context)
	AdminDelete(ctx iris.Context)
}

type contentHandler struct{ svc service.ContentService }

func newContentHandler(svc service.ContentService) ContentHandler {
	return &contentHandler{svc: svc}
}

// ListByPost 帖子评论列表
// @Summary      帖子评论列表
// @Description  获取指定帖子下的所有评论（任意深度），支持分页
// @Tags         内容
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId   path      string  true   "帖子 ID"
// @Param        body     body      object{page=int,pageSize=int,startTime=string,endTime=string} false "筛选参数"
// @Success      200  {object}  pkg.Response{data=[]model.Content}
// @Failure      500  {object}  pkg.Response
// @Router       /posts/{postId}/contents/list [post]
func (h *contentHandler) ListByPost(ctx iris.Context) {
	p, err := readListParams(ctx)
	if err != nil {
		return
	}
	st, et := readTimeRange(ctx, p)
	contents, total, err := h.svc.ListByPost(ctx.Request().Context(), ctx.Params().Get("postId"), p.Page, p.PageSize, st, et)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, contents, total)
}

// ListReplies 子回复列表
// @Summary      子回复列表
// @Description  获取指定内容的直接子回复，支持分页
// @Tags         内容
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body     body      object{page=int,pageSize=int,startTime=string,endTime=string} false "筛选参数"
// @Success      200  {object}  pkg.Response{data=[]model.Content}
// @Failure      500  {object}  pkg.Response
// @Router       /contents/{contentId}/replies [post]
func (h *contentHandler) ListReplies(ctx iris.Context) {
	p, err := readListParams(ctx)
	if err != nil {
		return
	}
	st, et := readTimeRange(ctx, p)
	contents, total, err := h.svc.ListReplies(ctx.Request().Context(), ctx.Params().Get("contentId"), p.Page, p.PageSize, st, et)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, contents, total)
}

// ListAll 全局评论列表（管理员）
// @Summary      全局评论列表
// @Description  获取所有帖子下的评论，支持分页
// @Tags         内容
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body     body      object{page=int,pageSize=int} false "分页参数"
// @Success      200  {object}  pkg.Response{data=[]model.ContentListItem,total=int}
// @Failure      500  {object}  pkg.Response
// @Router       /posts/comments/list [post]
func (h *contentHandler) ListAll(ctx iris.Context) {
	p, err := readListParams(ctx)
	if err != nil {
		return
	}
	contents, total, err := h.svc.ListAll(ctx.Request().Context(), p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, contents, total)
}

// Create 创建评论/回复
// @Summary      创建评论/回复
// @Description  在指定帖子下创建评论或回复
// @Tags         内容
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Param        body   body      object{body=string,parentId=string}  true  "内容"
// @Success      201  {object}  pkg.Response{data=model.Content}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /posts/{postId}/contents/create [post]
func (h *contentHandler) Create(ctx iris.Context) {
	var body struct {
		Body     string `json:"body"`
		ParentID string `json:"parent_id"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if body.Body == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "评论内容不能为空")
		return
	}
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	c, err := h.svc.Create(ctx.Request().Context(), ctx.Params().Get("postId"), u.ID, body.Body, body.ParentID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	ctx.StatusCode(iris.StatusCreated)
	pkg.Success(ctx, c)
}

// Update 编辑内容
// @Summary      编辑内容
// @Description  编辑指定内容
// @Tags         内容
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Param        id     path      string  true  "内容 ID"
// @Param        body   body      object{body=string}  true  "内容"
// @Success      200    {object}  pkg.Response{data=model.Content}
// @Failure      400    {object}  pkg.Response
// @Failure      401    {object}  pkg.Response
// @Failure      404    {object}  pkg.Response
// @Router       /posts/{postId}/contents/{id}/update [post]
func (h *contentHandler) Update(ctx iris.Context) {
	var body struct {
		Body string `json:"body"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	c, err := h.svc.Update(ctx.Request().Context(), ctx.Params().Get("postId"), ctx.Params().Get("id"), body.Body, u.ID, u.Role)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, c)
}

// Delete 删除内容
// @Summary      删除内容
// @Description  删除指定内容
// @Tags         内容
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Param        id     path      string  true  "内容 ID"
// @Success      200    {object}  pkg.Response
// @Failure      401    {object}  pkg.Response
// @Failure      403    {object}  pkg.Response
// @Failure      500    {object}  pkg.Response
// @Router       /posts/{postId}/contents/{id}/delete [post]
func (h *contentHandler) Delete(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("postId"), ctx.Params().Get("id"), u.ID, u.Role); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// AdminGet 管理员获取单条评论详情
// @Summary      评论详情
// @Description  通过评论 ID 直接获取评论详情（含作者信息、所属帖子）
// @Tags         内容
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "评论 ID"
// @Success      200  {object}  pkg.Response{data=model.ContentListItem}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /comments/{id} [get]
func (h *contentHandler) AdminGet(ctx iris.Context) {
	item, err := h.svc.GetByID(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, item)
}

// AdminDelete 管理员删除评论（无需 postId）
// @Summary      管理员删除评论
// @Description  通过评论 ID 直接删除评论，自动递减帖子评论计数
// @Tags         内容
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "评论 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /comments/{id}/delete [post]
func (h *contentHandler) AdminDelete(ctx iris.Context) {
	if err := h.svc.AdminDelete(ctx.Request().Context(), ctx.Params().Get("id"), currentRole(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
