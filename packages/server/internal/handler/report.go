package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type ReportHandler interface {
	Create(ctx iris.Context)
	List(ctx iris.Context)
	UpdateStatus(ctx iris.Context)
	Delete(ctx iris.Context)
}

type reportHandler struct{ svc service.ReportService }

func newReportHandler(svc service.ReportService) ReportHandler {
	return &reportHandler{svc: svc}
}

// Create 举报内容
// @Summary      举报内容
// @Description  举报帖子或评论
// @Tags         举报
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{targetType=string,targetId=string,reason=string} true "举报信息"
// @Success      201  {object}  pkg.Response{data=model.Report}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /reports/ [post]
func (h *reportHandler) Create(ctx iris.Context) {
	var body struct {
		TargetType string `json:"targetType"`
		TargetID   string `json:"targetId"`
		Reason     string `json:"reason"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	userID := ctx.Values().GetString("user_uuid")
	report, err := h.svc.Create(ctx.Request().Context(), userID, body.TargetType, body.TargetID, body.Reason)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	ctx.StatusCode(iris.StatusCreated)
	pkg.Success(ctx, report)
}

// List 举报列表
// @Summary      举报列表
// @Description  分页获取举报列表，可按状态筛选
// @Tags         举报
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        status   query  string  false  "举报状态"  Enums(pending, resolved, dismissed)
// @Param        page     query  int     false  "页码"     default(1)
// @Param        pageSize query  int     false  "每页数量"  default(20)
// @Success      200  {object}  pkg.Response{data=[]model.Report}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /reports/ [get]
func (h *reportHandler) List(ctx iris.Context) {
	var req struct {
		Status   string `json:"status"`
		Page     int    `json:"page"`
		PageSize int    `json:"pageSize"`
	}
	_ = ctx.ReadJSON(&req)
	if req.Status == "" {
		req.Status = ctx.URLParam("status")
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 20
	}
	reports, err := h.svc.List(ctx.Request().Context(), req.Status, req.Page, req.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, reports)
}

// UpdateStatus 更新举报状态
// @Summary      更新举报状态
// @Description  更新指定举报的状态
// @Tags         举报
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "举报 ID"
// @Param        body body object{status=string} true "状态信息"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /reports/{id}/status [put]
func (h *reportHandler) UpdateStatus(ctx iris.Context) {
	var body struct {
		Status string `json:"status"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.UpdateStatus(ctx.Request().Context(), ctx.Params().Get("id"), body.Status); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// Delete 删除举报
// @Summary      删除举报
// @Description  删除指定举报
// @Tags         举报
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "举报 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /reports/{id} [delete]
func (h *reportHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
