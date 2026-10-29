package handler

import (
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

// GateHandler 是门控规则引擎的后台配置面：规则 CRUD、能力域默认动作与试算。
type GateHandler interface {
	ListRules(ctx iris.Context)
	SaveRule(ctx iris.Context)
	DeleteRule(ctx iris.Context)
	ListDefaults(ctx iris.Context)
	SaveDefault(ctx iris.Context)
	Simulate(ctx iris.Context)
	Meta(ctx iris.Context)
}

type gateHandler struct {
	gateSvc service.GateService
	audit   service.AuditLogService
}

func newGateHandler(gateSvc service.GateService, audit service.AuditLogService) GateHandler {
	return &gateHandler{gateSvc: gateSvc, audit: audit}
}

// Meta 门控元数据（域 / 字段 / 操作符 / 动作 / 窗口指标），供后台表单渲染。
// @Summary      门控元数据
// @Tags         门控规则
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /admin/gates/meta [post]
func (h *gateHandler) Meta(ctx iris.Context) {
	pkg.Success(ctx, iris.Map{
		"domains":       model.GateDomainLabels,
		"fields":        model.GateFieldLabels,
		"field_ops":     model.GateFieldOps,
		"actions":       model.GateActions,
		"window_metric": model.GateWindowMetrics,
	})
}

// ListRules 规则列表
// @Summary      门控规则列表
// @Tags         门控规则
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=[]model.GateRule}
// @Router       /admin/gates/list [post]
func (h *gateHandler) ListRules(ctx iris.Context) {
	rules, err := h.gateSvc.ListRules(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, rules)
}

// SaveRule 新增或更新规则
// @Summary      保存门控规则
// @Tags         门控规则
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /admin/gates/save [post]
func (h *gateHandler) SaveRule(ctx iris.Context) {
	var req model.GateRule
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.gateSvc.SaveRule(ctx.Request().Context(), &req, currentUserID(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), currentUserID(ctx), "gate.rule_save", req.ID, req.Domain)
	pkg.Success(ctx, nil)
}

// DeleteRule 删除规则
// @Summary      删除门控规则
// @Tags         门控规则
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "规则 ID"
// @Success      200  {object}  pkg.Response
// @Router       /admin/gates/{id}/delete [post]
func (h *gateHandler) DeleteRule(ctx iris.Context) {
	if err := h.gateSvc.DeleteRule(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), currentUserID(ctx), "gate.rule_delete", ctx.Params().Get("id"), "")
	pkg.Success(ctx, nil)
}

// ListDefaults 能力域默认动作
// @Summary      能力域默认动作
// @Tags         门控规则
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=[]model.GateDomainDefault}
// @Router       /admin/gates/defaults/list [post]
func (h *gateHandler) ListDefaults(ctx iris.Context) {
	defs, err := h.gateSvc.ListDomainDefaults(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, defs)
}

// SaveDefault 保存默认动作
// @Summary      保存能力域默认动作
// @Tags         门控规则
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{domain=string,default_action=string} true "默认动作"
// @Success      200  {object}  pkg.Response
// @Router       /admin/gates/defaults/save [post]
func (h *gateHandler) SaveDefault(ctx iris.Context) {
	var req struct {
		Domain        string `json:"domain"`
		DefaultAction string `json:"default_action"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.gateSvc.SaveDomainDefault(ctx.Request().Context(), req.Domain, req.DefaultAction); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), currentUserID(ctx), "gate.default_save", req.Domain, req.DefaultAction)
	pkg.Success(ctx, nil)
}

// Simulate 以用户试算
// @Summary      以用户试算门控
// @Description  与线上求值走同一个 Evaluate，保证试算结果与实际请求一致
// @Tags         门控规则
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=service.GateDecision}
// @Router       /admin/gates/simulate [post]
func (h *gateHandler) Simulate(ctx iris.Context) {
	var req service.GateSimulateInput
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	decision, err := h.gateSvc.Simulate(ctx.Request().Context(), req)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, decision)
}
