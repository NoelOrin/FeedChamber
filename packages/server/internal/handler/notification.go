package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type NotificationHandler interface {
	List(ctx iris.Context)
	CountUnread(ctx iris.Context)
	MarkRead(ctx iris.Context)
	MarkAllRead(ctx iris.Context)
	Delete(ctx iris.Context)
}

type notificationHandler struct {
	svc service.NotificationService
}

func newNotificationHandler(svc service.NotificationService) NotificationHandler {
	return &notificationHandler{svc: svc}
}

// List 获取通知列表
// @Summary      获取通知列表
// @Description  分页获取当前用户的通知列表
// @Tags         通知
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page     query     int  false  "页码"      default(1)
// @Param        pageSize query     int  false  "每页数量"  default(20)
// @Success      200  {object}  pkg.Response{data=object{list=[]model.Notification}}
// @Failure      401  {object}  pkg.Response
// @Router       /notifications [get]
func (h *notificationHandler) List(ctx iris.Context) {
	userID := ctx.Values().GetString("user_uuid")
	if userID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	p := readListParams(ctx)
	notifications, err := h.svc.List(ctx.Request().Context(), userID, p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"list": notifications})
}

// CountUnread 获取未读通知数量
// @Summary      获取未读通知数量
// @Description  获取当前用户的未读通知数量
// @Tags         通知
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=object{count=int64}}
// @Failure      401  {object}  pkg.Response
// @Router       /notifications/unread_count [get]
func (h *notificationHandler) CountUnread(ctx iris.Context) {
	userID := ctx.Values().GetString("user_uuid")
	if userID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	count, err := h.svc.CountUnread(ctx.Request().Context(), userID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"count": count})
}

// MarkRead 标记通知已读
// @Summary      标记通知已读
// @Description  标记指定通知为已读
// @Tags         通知
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "通知 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /notifications/{id}/read [put]
func (h *notificationHandler) MarkRead(ctx iris.Context) {
	userID := ctx.Values().GetString("user_uuid")
	if userID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	id := ctx.Params().Get("id")
	if err := h.svc.MarkRead(ctx.Request().Context(), id, userID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// MarkAllRead 标记所有通知已读
// @Summary      标记所有通知已读
// @Description  标记当前用户的所有通知为已读
// @Tags         通知
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /notifications/read_all [put]
func (h *notificationHandler) MarkAllRead(ctx iris.Context) {
	userID := ctx.Values().GetString("user_uuid")
	if userID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	if err := h.svc.MarkAllRead(ctx.Request().Context(), userID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// Delete 删除通知
// @Summary      删除通知
// @Description  删除指定通知
// @Tags         通知
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "通知 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /notifications/{id} [delete]
func (h *notificationHandler) Delete(ctx iris.Context) {
	userID := ctx.Values().GetString("user_uuid")
	if userID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id"), userID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
