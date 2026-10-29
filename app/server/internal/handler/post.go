package handler

import (
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type PostHandler interface {
	List(ctx iris.Context)
	GetByID(ctx iris.Context)
	GetDetail(ctx iris.Context)
	ListByIDs(ctx iris.Context)
	Create(ctx iris.Context)
	Update(ctx iris.Context)
	Delete(ctx iris.Context)
	Search(ctx iris.Context)
	Suggestions(ctx iris.Context)
	TogglePin(ctx iris.Context)
	ToggleLock(ctx iris.Context)
	SetClosed(ctx iris.Context)
	SetStatus(ctx iris.Context)
	SetFeatured(ctx iris.Context)
	SetPin(ctx iris.Context)
	Unpin(ctx iris.Context)
	ListPinned(ctx iris.Context)
	MarkRead(ctx iris.Context)
}

type postHandler struct{ svc service.PostService }

func newPostHandler(svc service.PostService) PostHandler {
	return &postHandler{svc: svc}
}

// List 帖子列表
// @Summary      帖子列表
// @Description  分页获取帖子列表，支持作者/状态/时间筛选
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{page=int,pageSize=int,authorId=string,status=string,startTime=string,endTime=string} false "筛选参数"
// @Success      200  {object}  pkg.Response{data=[]model.PostListItem}
// @Failure      401  {object}  pkg.Response
// @Router       /posts/list [post]
func (h *postHandler) List(ctx iris.Context) {
	var body struct {
		ListParams
		TagName    string `json:"tag_name"`
		AuthorID   string `json:"author_id"`
		Status     string `json:"status"`
		SpaceName  string `json:"space_name"`
		SpaceSlug  string `json:"space_slug"`
		SpaceID    string `json:"space_id"`
		IncludeSub bool   `json:"include_sub"`
		SortBy     string `json:"sort_by"`
		ListView   string `json:"list_view"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	body.Normalize()
	if body.ListView != "" && currentUserID(ctx) == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	st, et := readTimeRange(ctx, body.ListParams)
	reqCtx := service.WithListView(service.WithViewerRole(service.WithViewerID(ctx.Request().Context(), currentUserID(ctx)), currentRole(ctx)), body.ListView)
	posts, total, err := h.svc.List(reqCtx, body.Page, body.PageSize, body.TagName, body.AuthorID, body.Status, st, et, body.SpaceName, body.SpaceSlug, body.SpaceID, body.IncludeSub, body.SortBy)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	h.svc.EnrichWithReadState(reqCtx, currentUserID(ctx), posts)
	pkg.SuccessPaginated(ctx, posts, total)
}

// GetDetail 获取帖子详情（含正文）
// @Summary      获取帖子详情（含正文）
// @Description  根据帖子 ID 获取帖子完整信息，包含正文内容，自动增加浏览量；非公开状态（pending/rejected）仅作者本人与具备审核权限者可读，他人 404
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "帖子 ID"
// @Success      200  {object}  pkg.Response{data=model.PostDetail}
// @Failure      401  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /posts/{id}/detail [get]
func (h *postHandler) GetDetail(ctx iris.Context) {
	id := ctx.Params().Get("id")
	if id == "" {
		pkg.Fail(ctx, pkg.NOT_FOUND, "帖子 ID 不能为空")
		return
	}
	reqCtx := service.WithViewerID(service.WithViewerRole(ctx.Request().Context(), currentRole(ctx)), currentUserID(ctx))
	detail, err := h.svc.GetDetail(reqCtx, id)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	if uid := currentUserID(ctx); uid != "" {
		items := []model.PostListItem{detail.PostListItem}
		h.svc.EnrichWithReadState(reqCtx, uid, items)
		detail.PostListItem = items[0]
	}
	pkg.Success(ctx, detail)
}

// GetByID 获取帖子详情
// @Summary      获取帖子详情
// @Description  根据帖子 ID 获取帖子详情，自动增加浏览量；非公开状态（pending/rejected）仅作者本人与具备审核权限者可读，他人 404
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
	id := ctx.Params().Get("id")
	if id == "" {
		pkg.Fail(ctx, pkg.NOT_FOUND, "帖子 ID 不能为空")
		return
	}
	reqCtx := service.WithViewerID(service.WithViewerRole(ctx.Request().Context(), currentRole(ctx)), currentUserID(ctx))
	post, err := h.svc.GetByID(reqCtx, id)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	if uid := currentUserID(ctx); uid != "" && post != nil {
		items := []model.PostListItem{*post}
		h.svc.EnrichWithReadState(reqCtx, uid, items)
		*post = items[0]
	}
	pkg.Success(ctx, post)
}

// ListByIDs 批量获取帖子
// @Summary      批量获取帖子
// @Description  按 ID 列表批量获取帖子
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Router       /posts/batch [post]
func (h *postHandler) ListByIDs(ctx iris.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if len(req.IDs) > 100 {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "批量查询最多 100 条")
		return
	}
	if len(req.IDs) == 0 {
		pkg.Success(ctx, []model.PostListItem{})
		return
	}
	reqCtx := service.WithViewerRole(ctx.Request().Context(), currentRole(ctx))
	posts, err := h.svc.ListByIDs(reqCtx, req.IDs)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	if posts == nil {
		posts = []model.PostListItem{}
	}
	h.svc.EnrichWithReadState(reqCtx, currentUserID(ctx), posts)
	pkg.Success(ctx, posts)
}

// Create 创建帖子
// @Summary      创建帖子
// @Description  创建新帖子
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{title=string,content=string,spaceId=string} true "帖子信息"
// @Success      201  {object}  pkg.Response{data=model.Post}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /posts/create [post]
func (h *postHandler) Create(ctx iris.Context) {
	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		SpaceID string `json:"space_id"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if body.Title == "" || body.Content == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "标题和内容不能为空")
		return
	}
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	post, err := h.svc.Create(ctx.Request().Context(), u.ID, body.Title, body.Content, body.SpaceID)
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
	id := ctx.Params().Get("id")
	if id == "" {
		pkg.Fail(ctx, pkg.NOT_FOUND, "帖子 ID 不能为空")
		return
	}
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	post, err := h.svc.Update(ctx.Request().Context(), id, body.Title, body.Content, u.ID, u.Role)
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
	id := ctx.Params().Get("id")
	if id == "" {
		pkg.Fail(ctx, pkg.NOT_FOUND, "帖子 ID 不能为空")
		return
	}
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	if err := h.svc.Delete(ctx.Request().Context(), id, u.ID, u.Role); err != nil {
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
	reqCtx := service.WithViewerRole(service.WithViewerID(ctx.Request().Context(), currentUserID(ctx)), currentRole(ctx))
	posts, total, err := h.svc.Search(reqCtx, body.Q, body.Page, body.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	h.svc.EnrichWithReadState(reqCtx, currentUserID(ctx), posts)
	pkg.SuccessPaginated(ctx, posts, total)
}

// Suggestions 相关主题
// @Summary      相关主题
// @Description  获取当前主题的相关主题，同版面优先、最近活跃、排除当前主题
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Param        id path string true "帖子 ID"
// @Success      200  {object}  pkg.Response{data=[]model.PostListItem}
// @Failure      404  {object}  pkg.Response
// @Router       /posts/{id}/suggestions [post]
func (h *postHandler) Suggestions(ctx iris.Context) {
	id := ctx.Params().Get("id")
	if id == "" {
		pkg.Fail(ctx, pkg.NOT_FOUND, "帖子 ID 不能为空")
		return
	}
	reqCtx := service.WithViewerRole(service.WithViewerID(ctx.Request().Context(), currentUserID(ctx)), currentRole(ctx))
	items, err := h.svc.Suggestions(reqCtx, id, 5)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, items)
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
	post, err := h.svc.TogglePin(ctx.Request().Context(), ctx.Params().Get("id"), currentRole(ctx))
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
	post, err := h.svc.ToggleLock(ctx.Request().Context(), ctx.Params().Get("id"), currentRole(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, post)
}

// SetClosed 关闭/打开话题
// @Summary      关闭或打开话题
// @Description  将帖子状态在 approved 与 closed 之间切换
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "帖子 ID"
// @Param        body body object{closed=bool} true "关闭状态"
// @Success      200  {object}  pkg.Response{data=model.Post}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /posts/{id}/close [post]
func (h *postHandler) SetClosed(ctx iris.Context) {
	var body struct {
		Closed bool `json:"closed"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	post, err := h.svc.SetClosed(ctx.Request().Context(), ctx.Params().Get("id"), body.Closed, currentUserID(ctx), currentRole(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, post)
}

// SetStatus 变更帖子状态
// @Summary      变更帖子状态
// @Description  直接变更帖子状态，取值 approved / pending / rejected / closed。用于审核没有审核记录的存量待审帖；需要 report:manage 权限
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "帖子 ID"
// @Param        body body object{status=string} true "目标状态"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /posts/{id}/status [post]
func (h *postHandler) SetStatus(ctx iris.Context) {
	id := ctx.Params().Get("id")
	if id == "" {
		pkg.Fail(ctx, pkg.NOT_FOUND, "帖子 ID 不能为空")
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.SetStatus(ctx.Request().Context(), id, body.Status); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// SetFeatured 设置 / 取消精华
// @Summary      设为精华
// @Description  写 posts.featured_at；精华帖计入作者 XP（featured_post_count）
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "帖子 ID"
// @Param        body body object{featured=bool} true "是否精华"
// @Success      200  {object}  pkg.Response
// @Router       /posts/{id}/feature [post]
func (h *postHandler) SetFeatured(ctx iris.Context) {
	id := ctx.Params().Get("id")
	var body struct {
		Featured bool `json:"featured"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	post, err := h.svc.SetFeatured(ctx.Request().Context(), id, body.Featured)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, post)
}

// SetPin 精细控制置顶
// @Summary      设置置顶
// @Description  设置帖子的置顶排序和过期时间
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "帖子 ID"
// @Param        body body object{pin_order=int,pinned_expire_at=int64} true "置顶信息"
// @Router       /posts/{id}/pin/set [post]
func (h *postHandler) SetPin(ctx iris.Context) {
	var body struct {
		PinOrder       int    `json:"pin_order"`
		PinnedExpireAt *int64 `json:"pinned_expire_at"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	post, err := h.svc.SetPin(ctx.Request().Context(), ctx.Params().Get("id"), currentRole(ctx),
		service.SetPinRequest{PinOrder: body.PinOrder, PinnedExpireAt: body.PinnedExpireAt})
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, post)
}

// Unpin 取消置顶
// @Summary      取消置顶
// @Description  取消指定帖子的置顶状态
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "帖子 ID"
// @Router       /posts/{id}/pin/remove [post]
func (h *postHandler) Unpin(ctx iris.Context) {
	post, err := h.svc.Unpin(ctx.Request().Context(), ctx.Params().Get("id"), currentRole(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, post)
}

// ListPinned 置顶列表
// @Summary      置顶帖子列表
// @Description  获取指定版面的置顶帖子（或全部版面）
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Param        body body object{space_id=string,page=int,pageSize=int} false "筛选参数"
// @Router       /posts/pinned/list [post]
func (h *postHandler) ListPinned(ctx iris.Context) {
	var body struct {
		SpaceID  string `json:"space_id"`
		Page     int    `json:"page"`
		PageSize int    `json:"page_size"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	body.Page, body.PageSize = pkg.NormalizePagination(body.Page, body.PageSize)
	reqCtx := ctx.Request().Context()
	items, total, err := h.svc.ListPinned(reqCtx, body.SpaceID, body.Page, body.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	h.svc.EnrichWithReadState(reqCtx, currentUserID(ctx), items)
	pkg.SuccessPaginated(ctx, items, total)
}

// MarkRead 标记话题已读
// @Summary      标记话题已读
// @Description  显式标记当前用户已阅读指定帖子（更新 last_read_at）
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "帖子 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /posts/{id}/read [post]
func (h *postHandler) MarkRead(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	var body struct {
		LastContentID string `json:"last_content_id"`
	}
	_ = ctx.ReadJSON(&body)
	id := ctx.Params().Get("id")
	if id == "" {
		pkg.Fail(ctx, pkg.NOT_FOUND, "帖子 ID 不能为空")
		return
	}
	if err := h.svc.MarkReadToContent(ctx.Request().Context(), u.ID, id, body.LastContentID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
