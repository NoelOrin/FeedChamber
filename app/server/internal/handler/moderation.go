package handler

import (
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
	"github.com/sirupsen/logrus"
)

type ModerationHandler interface {
	ListPending(ctx iris.Context)
	Approve(ctx iris.Context)
	Reject(ctx iris.Context)
	CreateRule(ctx iris.Context)
	UpdateRule(ctx iris.Context)
	DeleteRule(ctx iris.Context)
	ListRules(ctx iris.Context)
	GetConfigs(ctx iris.Context)
	SetConfig(ctx iris.Context)
}

type moderationHandler struct {
	svc         service.ModerationService
	auditLogSvc service.AuditLogService
}

func newModerationHandler(svc service.ModerationService, auditLogSvc service.AuditLogService) ModerationHandler {
	return &moderationHandler{svc: svc, auditLogSvc: auditLogSvc}
}

// ListPending 待审核列表
// @Router /admin/moderation/pending [post]
func (h *moderationHandler) ListPending(ctx iris.Context) {
	var req struct {
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	req.Page, req.PageSize = pkg.NormalizePagination(req.Page, req.PageSize)
	items, total, err := h.svc.ListPending(ctx.Request().Context(), req.Page, req.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, int(total))
}

// Approve 审核通过
// @Router /admin/moderation/{id}/approve [post]
func (h *moderationHandler) Approve(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	id := ctx.Params().Get("id")
	if err := h.svc.Approve(ctx.Request().Context(), id, u.ID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	if err := h.auditLogSvc.Record(ctx.Request().Context(), u.ID, "moderation.approve", id, ""); err != nil {
		logrus.Warnf("audit log write failed: %v", err)
	}
	pkg.Success(ctx, nil)
}

// Reject 审核拒绝
// @Router /admin/moderation/{id}/reject [post]
func (h *moderationHandler) Reject(ctx iris.Context) {
	var req struct {
		Reason string `json:"reason"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	id := ctx.Params().Get("id")
	if err := h.svc.Reject(ctx.Request().Context(), id, u.ID, req.Reason); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	if err := h.auditLogSvc.Record(ctx.Request().Context(), u.ID, "moderation.reject", id, req.Reason); err != nil {
		logrus.Warnf("audit log write failed: %v", err)
	}
	pkg.Success(ctx, nil)
}

// CreateRule 创建审核规则
// @Router /admin/moderation/rules/create [post]
func (h *moderationHandler) CreateRule(ctx iris.Context) {
	var rule model.ModerationRule
	if err := ctx.ReadJSON(&rule); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.CreateRule(ctx.Request().Context(), &rule); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, rule)
}

// UpdateRule 更新审核规则
// @Router /admin/moderation/rules/update [post]
func (h *moderationHandler) UpdateRule(ctx iris.Context) {
	var rule model.ModerationRule
	if err := ctx.ReadJSON(&rule); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.UpdateRule(ctx.Request().Context(), &rule); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// DeleteRule 删除审核规则
// @Router /admin/moderation/rules/delete [post]
func (h *moderationHandler) DeleteRule(ctx iris.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.DeleteRule(ctx.Request().Context(), req.ID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// ListRules 审核规则列表
// @Router /admin/moderation/rules/list [post]
func (h *moderationHandler) ListRules(ctx iris.Context) {
	var req struct {
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	req.Page, req.PageSize = pkg.NormalizePagination(req.Page, req.PageSize)
	items, total, err := h.svc.ListRules(ctx.Request().Context(), req.Page, req.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, int(total))
}

// GetConfigs 获取审核配置
// @Router /admin/moderation/configs [post]
func (h *moderationHandler) GetConfigs(ctx iris.Context) {
	configs, err := h.svc.GetConfigs(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, configs)
}

// SetConfig 设置审核配置
// @Router /admin/moderation/configs/set [post]
func (h *moderationHandler) SetConfig(ctx iris.Context) {
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.SetConfig(ctx.Request().Context(), req.Key, req.Value); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
