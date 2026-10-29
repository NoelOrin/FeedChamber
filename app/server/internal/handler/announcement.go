package handler

import (
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type AnnouncementHandler interface {
	Create(ctx iris.Context)
	Update(ctx iris.Context)
	Delete(ctx iris.Context)
	Get(ctx iris.Context)
	List(ctx iris.Context)
	ListPublished(ctx iris.Context)
	Publish(ctx iris.Context)
	Unpublish(ctx iris.Context)
}

type announcementHandler struct {
	svc service.AnnouncementService
}

func newAnnouncementHandler(svc service.AnnouncementService) AnnouncementHandler {
	return &announcementHandler{svc: svc}
}

// Create 创建公告
// @Summary      创建公告
// @Description  创建一条新公告
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{title=string,content=string,display_type=integer,position=string,is_pinned=boolean,sort=integer,status=string,expires_at=string} true "公告信息"
// @Success      200  {object}  pkg.Response{data=model.Announcement}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /admin/announcements/create [post]
func (h *announcementHandler) Create(ctx iris.Context) {
	var req struct {
		Title       string     `json:"title"`
		Content     string     `json:"content"`
		DisplayType int        `json:"display_type"`
		Position    string     `json:"position"`
		IsPinned    bool       `json:"is_pinned"`
		Sort        int        `json:"sort"`
		Status      string     `json:"status"`
		ExpiresAt   *time.Time `json:"expires_at"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if req.Title == "" || req.Content == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "标题和内容不能为空")
		return
	}

	u, ok := requireUser(ctx)
	if !ok {
		return
	}

	ann := &model.Announcement{
		Title:       req.Title,
		Content:     req.Content,
		AuthorID:    u.ID,
		Status:      req.Status,
		DisplayType: req.DisplayType,
		Position:    req.Position,
		IsPinned:    req.IsPinned,
		Sort:        req.Sort,
		ExpiresAt:   req.ExpiresAt,
	}

	result, err := h.svc.Create(ctx.Request().Context(), ann)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, result)
}

// Update 更新公告
// @Summary      更新公告
// @Description  更新指定公告的内容
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "公告 ID"
// @Param        body body      object{title=string,content=string,display_type=integer,position=string,is_pinned=boolean,sort=integer,status=string,expires_at=string} true "公告信息"
// @Success      200  {object}  pkg.Response{data=model.Announcement}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /admin/announcements/{id}/update [post]
func (h *announcementHandler) Update(ctx iris.Context) {
	var req struct {
		Title       string     `json:"title"`
		Content     string     `json:"content"`
		DisplayType int        `json:"display_type"`
		Position    string     `json:"position"`
		IsPinned    bool       `json:"is_pinned"`
		Sort        int        `json:"sort"`
		Status      string     `json:"status"`
		ExpiresAt   *time.Time `json:"expires_at"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}

	ann := &model.Announcement{
		ID:          ctx.Params().Get("id"),
		Title:       req.Title,
		Content:     req.Content,
		DisplayType: req.DisplayType,
		Position:    req.Position,
		IsPinned:    req.IsPinned,
		Sort:        req.Sort,
		Status:      req.Status,
		ExpiresAt:   req.ExpiresAt,
	}

	result, err := h.svc.Update(ctx.Request().Context(), ann)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, result)
}

// Delete 删除公告
// @Summary      删除公告
// @Description  删除指定公告
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "公告 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /admin/announcements/{id}/delete [post]
func (h *announcementHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// Get 获取公告详情
// @Summary      获取公告详情
// @Description  根据ID获取公告详情
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "公告 ID"
// @Success      200  {object}  pkg.Response{data=model.Announcement}
// @Failure      401  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /announcements/{id} [get]
func (h *announcementHandler) Get(ctx iris.Context) {
	ann, err := h.svc.GetByID(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, ann)
}

// List 公告列表（管理端）
// @Summary      公告列表
// @Description  分页获取所有公告（含草稿）
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page     query  int  false  "页码"  default(1)
// @Param        pageSize query  int  false  "每页数量"  default(20)
// @Success      200  {object}  pkg.Response{data=object{list=[]model.Announcement}}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /admin/announcements/list [post]
func (h *announcementHandler) List(ctx iris.Context) {
	p, err := readListParams(ctx)
	if err != nil {
		return
	}
	list, err := h.svc.List(ctx.Request().Context(), p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"items": list})
}

// ListPublished 已发布公告列表（公开）
// @Summary      已发布公告列表
// @Description  分页获取已发布且未过期的公告
// @Tags         公告
// @Accept       json
// @Produce      json
// @Param        page     query  int  false  "页码"  default(1)
// @Param        pageSize query  int  false  "每页数量"  default(20)
// @Success      200  {object}  pkg.Response{data=object{list=[]model.Announcement}}
// @Router       /announcements/published [post]
func (h *announcementHandler) ListPublished(ctx iris.Context) {
	p, err := readListParams(ctx)
	if err != nil {
		return
	}
	list, err := h.svc.ListPublished(ctx.Request().Context(), p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"items": list})
}

// Publish 发布公告
// @Summary      发布公告
// @Description  将指定公告状态改为已发布
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "公告 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /admin/announcements/{id}/publish [post]
func (h *announcementHandler) Publish(ctx iris.Context) {
	if err := h.svc.Publish(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// Unpublish 撤回公告
// @Summary      撤回公告
// @Description  将指定公告状态改为草稿
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "公告 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /admin/announcements/{id}/unpublish [post]
func (h *announcementHandler) Unpublish(ctx iris.Context) {
	if err := h.svc.Unpublish(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
