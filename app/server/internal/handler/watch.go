package handler

import (
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

type WatchHandler interface {
	Set(ctx iris.Context)
	List(ctx iris.Context)
	Delete(ctx iris.Context)
	GetStatus(ctx iris.Context)
}

type watchHandler struct{ svc service.WatchService }

func newWatchHandler(svc service.WatchService) WatchHandler {
	return &watchHandler{svc: svc}
}

// Set 设置关注
// @Summary      设置关注
// @Tags         关注话题
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{target_type=string,target_id=string,level=string} true "关注参数"
// @Success      200 {object} pkg.Response
// @Router       /watches/set [post]
func (h *watchHandler) Set(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	var body struct {
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
		Level      string `json:"level"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	w, err := h.svc.Set(ctx.Request().Context(), u.ID, body.TargetType, body.TargetID, body.Level)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, w)
}

// List 关注列表
// @Summary      关注列表
// @Tags         关注话题
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} pkg.Response
// @Router       /watches/list [post]
func (h *watchHandler) List(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	p, err := readListParams(ctx)
	if err != nil {
		return
	}
	items, total, err := h.svc.List(ctx.Request().Context(), u.ID, p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, total)
}

// Delete 取消关注
// @Summary      取消关注
// @Tags         关注话题
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{target_type=string,target_id=string} true "取消参数"
// @Success      200 {object} pkg.Response
// @Router       /watches/delete [post]
func (h *watchHandler) Delete(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	var body struct {
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	if err := h.svc.Delete(ctx.Request().Context(), u.ID, body.TargetType, body.TargetID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// GetStatus 查询关注状态
// @Summary      查询关注状态
// @Tags         关注话题
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{target_type=string,target_id=string} true "查询参数"
// @Success      200 {object} pkg.Response
// @Router       /watches/status [post]
func (h *watchHandler) GetStatus(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	var body struct {
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	w, err := h.svc.Get(ctx.Request().Context(), u.ID, body.TargetType, body.TargetID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	if w == nil {
		pkg.Success(ctx, iris.Map{"watching": false, "level": ""})
		return
	}
	pkg.Success(ctx, iris.Map{"watching": true, "level": w.Level, "watch": w})
}
