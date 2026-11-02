package handler

import (
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

// TrustHandler 是 XP / 等级 / 连续签到阶梯的后台配置面。
//
// 路由全部挂在 /admin 下并由 level:manage 把守；等级与 XP 规则是「配置」，
// 不是用户数据，因此读写入口集中在这里，避免散落到各业务 handler。
type TrustHandler interface {
	ListLevels(ctx iris.Context)
	SaveLevel(ctx iris.Context)
	DeleteLevel(ctx iris.Context)
	ListXPRules(ctx iris.Context)
	SaveXPRule(ctx iris.Context)
	ListStreakBonuses(ctx iris.Context)
	SaveStreakBonus(ctx iris.Context)
	DeleteStreakBonus(ctx iris.Context)
	RecomputeAll(ctx iris.Context)
	GetUserXP(ctx iris.Context)
	AdjustXP(ctx iris.Context)
	ListAdjustments(ctx iris.Context)
	DeleteAdjustment(ctx iris.Context)
	GetPublicLevels(ctx iris.Context)
}

type trustHandler struct {
	trustSvc service.TrustService
	audit    service.AuditLogService
}

func newTrustHandler(trustSvc service.TrustService, audit service.AuditLogService) TrustHandler {
	return &trustHandler{trustSvc: trustSvc, audit: audit}
}

// ListLevels 等级定义列表
// @Summary      等级定义列表
// @Tags         信任等级
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=[]model.TrustLevel}
// @Router       /admin/levels/list [post]
func (h *trustHandler) ListLevels(ctx iris.Context) {
	levels, err := h.trustSvc.ListLevels(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, levels)
}

// SaveLevel 新增或更新等级定义
// @Summary      保存等级定义
// @Tags         信任等级
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /admin/levels/save [post]
func (h *trustHandler) SaveLevel(ctx iris.Context) {
	var req model.TrustLevel
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.trustSvc.SaveLevel(ctx.Request().Context(), &req); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), currentUserID(ctx), "trust.level_save", "", req.Name)
	pkg.Success(ctx, nil)
}

// DeleteLevel 删除等级定义
// @Summary      删除等级
// @Tags         信任等级
// @Produce      json
// @Security     BearerAuth
// @Param        level  path  int  true  "等级"
// @Success      200  {object}  pkg.Response
// @Router       /admin/levels/{level}/delete [post]
func (h *trustHandler) DeleteLevel(ctx iris.Context) {
	level, err := ctx.Params().GetInt("level")
	if err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "等级参数无效")
		return
	}
	if err := h.trustSvc.DeleteLevel(ctx.Request().Context(), level); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), currentUserID(ctx), "trust.level_delete", "", "")
	pkg.Success(ctx, nil)
}

// ListXPRules XP 规则列表
// @Summary      XP 规则列表
// @Tags         信任等级
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=[]model.XPRule}
// @Router       /admin/xp-rules/list [post]
func (h *trustHandler) ListXPRules(ctx iris.Context) {
	rules, err := h.trustSvc.ListXPRules(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, rules)
}

// SaveXPRule 保存 XP 规则
// @Summary      保存 XP 规则
// @Tags         信任等级
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /admin/xp-rules/save [post]
func (h *trustHandler) SaveXPRule(ctx iris.Context) {
	var req model.XPRule
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.trustSvc.SaveXPRule(ctx.Request().Context(), &req); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), currentUserID(ctx), "trust.xp_rule_save", "", req.Metric)
	pkg.Success(ctx, nil)
}

// ListStreakBonuses 连续签到阶梯列表
// @Summary      连续签到阶梯
// @Tags         信任等级
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=[]model.XPStreakBonus}
// @Router       /admin/xp-rules/streak/list [post]
func (h *trustHandler) ListStreakBonuses(ctx iris.Context) {
	rows, err := h.trustSvc.ListStreakBonuses(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, rows)
}

// SaveStreakBonus 保存连续签到阶梯
// @Summary      保存连续签到阶梯
// @Tags         信任等级
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /admin/xp-rules/streak/save [post]
func (h *trustHandler) SaveStreakBonus(ctx iris.Context) {
	var req model.XPStreakBonus
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.trustSvc.SaveStreakBonus(ctx.Request().Context(), &req); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), currentUserID(ctx), "trust.streak_save", "", "")
	pkg.Success(ctx, nil)
}

// DeleteStreakBonus 删除连续签到阶梯
// @Summary      删除连续签到阶梯
// @Tags         信任等级
// @Produce      json
// @Security     BearerAuth
// @Param        days  path  int  true  "里程碑天数"
// @Success      200  {object}  pkg.Response
// @Router       /admin/xp-rules/streak/{days}/delete [post]
func (h *trustHandler) DeleteStreakBonus(ctx iris.Context) {
	days, err := ctx.Params().GetInt("days")
	if err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "天数参数无效")
		return
	}
	if err := h.trustSvc.DeleteStreakBonus(ctx.Request().Context(), days); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), currentUserID(ctx), "trust.streak_delete", "", "")
	pkg.Success(ctx, nil)
}

// RecomputeAll 全量重算 XP / 等级
// @Summary      全量重算
// @Tags         信任等级
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=object{count=int}}
// @Router       /admin/trust/recompute [post]
func (h *trustHandler) RecomputeAll(ctx iris.Context) {
	count, err := h.trustSvc.RecomputeAll(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), currentUserID(ctx), "trust.recompute", "", "")
	pkg.Success(ctx, iris.Map{"count": count})
}

// GetUserXP 用户 XP / 等级 / 指标分解
// @Summary      用户 XP 详情
// @Tags         信任等级
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "用户 ID"
// @Success      200  {object}  pkg.Response{data=service.XPBreakdown}
// @Router       /admin/users/{id}/xp [get]
func (h *trustHandler) GetUserXP(ctx iris.Context) {
	breakdown, err := h.trustSvc.GetBreakdown(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, breakdown)
}

// AdjustXP 人工调整 XP
// @Summary      人工调整 XP
// @Tags         信任等级
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "用户 ID"
// @Param        body  body  object{delta=int,reason=string}  true  "调整"
// @Success      200  {object}  pkg.Response
// @Router       /admin/users/{id}/xp/adjust [post]
func (h *trustHandler) AdjustXP(ctx iris.Context) {
	var req struct {
		Delta  int    `json:"delta"`
		Reason string `json:"reason"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	adj, err := h.trustSvc.AdjustXP(ctx.Request().Context(), currentUserID(ctx), ctx.Params().Get("id"), req.Delta, req.Reason)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), currentUserID(ctx), "trust.xp_adjust", ctx.Params().Get("id"), req.Reason)
	pkg.Success(ctx, adj)
}

// ListAdjustments 人工调整记录
// @Summary      人工调整记录
// @Tags         信任等级
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "用户 ID"
// @Success      200  {object}  pkg.Response{data=[]model.XPAdjustment}
// @Router       /admin/users/{id}/xp/adjustments/list [post]
func (h *trustHandler) ListAdjustments(ctx iris.Context) {
	var req struct {
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
	}
	_ = ctx.ReadJSON(&req)
	items, total, err := h.trustSvc.ListAdjustments(ctx.Request().Context(), ctx.Params().Get("id"), req.Page, req.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, total)
}

// DeleteAdjustment 撤销人工调整
// @Summary      撤销人工调整
// @Tags         信任等级
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "调整记录 ID"
// @Success      200  {object}  pkg.Response
// @Router       /admin/xp-adjustments/{id}/delete [post]
func (h *trustHandler) DeleteAdjustment(ctx iris.Context) {
	if err := h.trustSvc.DeleteAdjustment(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), currentUserID(ctx), "trust.xp_adjust_delete", ctx.Params().Get("id"), "")
	pkg.Success(ctx, nil)
}

// GetPublicLevels 公开的等级名称表（前端把数值等级渲染成名称 / 展示升级进度）。
// @Summary      等级名称（公开）
// @Tags         信任等级
// @Produce      json
// @Success      200  {object}  pkg.Response{data=[]model.TrustLevel}
// @Router       /levels [get]
func (h *trustHandler) GetPublicLevels(ctx iris.Context) {
	levels, err := h.trustSvc.ListLevels(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	type publicLevel struct {
		Level int    `json:"level"`
		Key   string `json:"key"`
		Name  string `json:"name"`
		MinXP int    `json:"min_xp"`
	}
	out := make([]publicLevel, 0, len(levels))
	for _, l := range levels {
		if !l.Enabled {
			continue
		}
		out = append(out, publicLevel{Level: l.Level, Key: l.Key, Name: l.Name, MinXP: l.MinXP})
	}
	pkg.Success(ctx, out)
}
