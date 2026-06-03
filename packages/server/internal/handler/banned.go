package handler

import (
	"time"

	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type BannedHandler interface {
	Ban(ctx iris.Context)
	Unban(ctx iris.Context)
	Get(ctx iris.Context)
	List(ctx iris.Context)
}

type bannedHandler struct {
	svc service.BannedService
}

func newBannedHandler(svc service.BannedService) BannedHandler {
	return &bannedHandler{svc: svc}
}

// Ban 禁言用户
// @Summary      禁言用户
// @Description  禁言指定用户，支持永久或临时禁言
// @Tags         禁言
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "用户 ID"
// @Param        body body      object{reason=string,is_permanent=boolean,duration_minutes=integer}  true  "禁言信息"
// @Success      200  {object}  pkg.Response{data=model.Banned}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /user/{id}/banned [post]
func (h *bannedHandler) Ban(ctx iris.Context) {
	var req struct {
		Reason          string `json:"reason"`
		IsPermanent     bool   `json:"is_permanent"`
		DurationMinutes *int   `json:"duration_minutes"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}

	if req.Reason == "" {
		req.Reason = "未提供原因"
	}

	var dur *time.Duration
	if !req.IsPermanent && req.DurationMinutes != nil && *req.DurationMinutes > 0 {
		d := time.Duration(*req.DurationMinutes) * time.Minute
		dur = &d
	}

	createdBy := ctx.Values().GetString("username")
	if createdBy == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}

	banned, err := h.svc.Ban(ctx.Request().Context(), ctx.Params().Get("id"), req.Reason, createdBy, req.IsPermanent, dur)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, banned)
}

// Unban 解禁用户
// @Summary      解禁用户
// @Description  解除对指定用户的禁言
// @Tags         禁言
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "用户 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /user/{id}/banned [delete]
func (h *bannedHandler) Unban(ctx iris.Context) {
	if err := h.svc.Unban(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// Get 获取用户的禁言记录
// @Summary      获取用户禁言记录
// @Description  获取指定用户的禁言状态和记录
// @Tags         禁言
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "用户 ID"
// @Success      200  {object}  pkg.Response{data=model.Banned}
// @Failure      401  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /user/{id}/banned [get]
func (h *bannedHandler) Get(ctx iris.Context) {
	banned, err := h.svc.GetByUserID(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, banned)
}

// List 禁言记录列表
// @Summary      禁言记录列表
// @Description  分页获取禁言记录列表
// @Tags         禁言
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page     query  int  false  "页码"  default(1)
// @Param        pageSize query  int  false  "每页数量"  default(20)
// @Success      200  {object}  pkg.Response{data=object{list=[]model.Banned}}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /banned/list [get]
func (h *bannedHandler) List(ctx iris.Context) {
	p := readListParams(ctx)
	bans, err := h.svc.List(ctx.Request().Context(), p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"list": bans})
}
