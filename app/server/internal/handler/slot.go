package handler

import (
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

// SlotHandler 提供壳层插槽的管理端点与公开读取端点。
type SlotHandler interface {
	List(ctx iris.Context)
	Create(ctx iris.Context)
	Update(ctx iris.Context)
	Delete(ctx iris.Context)
	Public(ctx iris.Context)
}

type slotHandler struct {
	svc   service.SlotService
	trust service.TrustService
}

func newSlotHandler(svc service.SlotService, trust service.TrustService) SlotHandler {
	return &slotHandler{svc: svc, trust: trust}
}

// SlotEntryRequest 是插槽条目的请求体。
type SlotEntryRequest struct {
	Slot       string         `json:"slot"`
	Kind       string         `json:"kind"`
	Props      map[string]any `json:"props"`
	Sort       int            `json:"sort"`
	Visibility string         `json:"visibility"`
	MinLevel   int            `json:"min_level"`
	Enabled    *bool          `json:"enabled"`
}

func (r SlotEntryRequest) toInput() service.SlotEntryInput {
	return service.SlotEntryInput{
		Slot:       r.Slot,
		Kind:       r.Kind,
		Props:      r.Props,
		Sort:       r.Sort,
		Visibility: r.Visibility,
		MinLevel:   r.MinLevel,
		// 直接透传指针：nil 表示「未指定」，由 service 按启用处理。
		Enabled: r.Enabled,
	}
}

// 插槽条目列表
// @Summary      插槽条目列表
// @Description  列出壳层插槽条目（含停用）。slot 为空时返回全部。
// @Tags         插槽
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{slot=string} false "按插槽筛选"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/slots/list [post]
func (h *slotHandler) List(ctx iris.Context) {
	var body struct {
		Slot string `json:"slot"`
	}
	_ = ctx.ReadJSON(&body)

	items, err := h.svc.List(ctx.Request().Context(), body.Slot)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, items)
}

// 创建插槽条目
// @Summary      创建插槽条目
// @Description  在指定插槽创建一条内容条目。不接受裸 HTML。
// @Tags         插槽
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body SlotEntryRequest true "条目内容"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Router       /admin/slots/create [post]
func (h *slotHandler) Create(ctx iris.Context) {
	var req SlotEntryRequest
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	e, err := h.svc.Create(ctx.Request().Context(), req.toInput(), currentUserID(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, e)
}

// 更新插槽条目
// @Summary      更新插槽条目
// @Description  修改一条插槽条目的内容或可见性。
// @Tags         插槽
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "条目 ID"
// @Param        body body SlotEntryRequest true "条目内容"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Router       /admin/slots/{id}/update [post]
func (h *slotHandler) Update(ctx iris.Context) {
	var req SlotEntryRequest
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	e, err := h.svc.Update(ctx.Request().Context(), ctx.Params().Get("id"), req.toInput(), currentUserID(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, e)
}

// 删除插槽条目
// @Summary      删除插槽条目
// @Description  删除一条插槽条目。
// @Tags         插槽
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "条目 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/slots/{id}/delete [post]
func (h *slotHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id"), currentUserID(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// 公开插槽内容
// @Summary      公开插槽内容
// @Description  返回当前访问者可见的插槽条目（已按 slot + sort 排序）。游客也可见。
// @Tags         插槽
// @Accept       json
// @Produce      json
// @Success      200  {object}  pkg.Response
// @Router       /slots/public [post]
func (h *slotHandler) Public(ctx iris.Context) {
	reqCtx := ctx.Request().Context()

	isLogin := currentUserID(ctx) != ""
	level := 0
	if h.trust != nil {
		// 与 GetPublicCustomization 用同一个取值口径：LevelOf 是廉价读，
		// 游客（空 userID）返回 0。不要在这里改走 FindByID——两处口径不一致时，
		// 导航与插槽会对同一个用户显示出不同的可见性。
		level = h.trust.LevelOf(reqCtx, currentUserID(ctx))
	}

	items, err := h.svc.ListForViewer(reqCtx, isLogin, level)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, items)
}
