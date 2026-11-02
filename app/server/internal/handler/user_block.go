package handler

import (
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

type UserBlockHandler interface {
	Block(ctx iris.Context)
	Unblock(ctx iris.Context)
	Status(ctx iris.Context)
	List(ctx iris.Context)
}

type userBlockHandler struct{ svc service.UserBlockService }

func newUserBlockHandler(svc service.UserBlockService) UserBlockHandler {
	return &userBlockHandler{svc: svc}
}

// Block 屏蔽用户
// @Router /users/{userId}/block [post]
func (h *userBlockHandler) Block(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	targetID := ctx.Params().Get("userId")
	if err := h.svc.Block(ctx.Request().Context(), u.ID, targetID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// Unblock 取消屏蔽
// @Router /users/{userId}/unblock [post]
func (h *userBlockHandler) Unblock(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	targetID := ctx.Params().Get("userId")
	if err := h.svc.Unblock(ctx.Request().Context(), u.ID, targetID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// Status 屏蔽状态
// @Router /users/{userId}/block/status [post]
func (h *userBlockHandler) Status(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	targetID := ctx.Params().Get("userId")
	blocked, err := h.svc.IsBlocked(ctx.Request().Context(), u.ID, targetID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"blocked": blocked})
}

// List 屏蔽列表
// @Router /blocks/list [post]
func (h *userBlockHandler) List(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	p, err := readListParams(ctx)
	if err != nil {
		return
	}
	ids, total, err := h.svc.ListBlockedIDs(ctx.Request().Context(), u.ID, p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, ids, total)
}
