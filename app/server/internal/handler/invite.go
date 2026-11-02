package handler

import (
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

// InviteHandler 邀请码管理接口。
type InviteHandler interface {
	Create(ctx iris.Context)
	List(ctx iris.Context)
	Delete(ctx iris.Context)
}

type inviteHandler struct {
	svc service.InviteService
}

func newInviteHandler(svc service.InviteService) InviteHandler {
	return &inviteHandler{svc: svc}
}

// Create 创建邀请码
// @Summary      创建邀请码
// @Tags         邀请码
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body service.CreateInviteRequest true "邀请码参数"
// @Success      200  {object}  pkg.Response{data=model.Invite}
// @Router       /admin/invites/create [post]
func (h *inviteHandler) Create(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	var req service.CreateInviteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		req = service.CreateInviteRequest{}
	}
	inv, err := h.svc.Create(ctx.Request().Context(), u.ID, &req)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, inv)
}

// List 邀请码列表
// @Summary      邀请码列表
// @Tags         邀请码
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=pkg.PaginatedData}
// @Router       /admin/invites/list [post]
func (h *inviteHandler) List(ctx iris.Context) {
	p, err := readListParams(ctx)
	if err != nil {
		return
	}
	items, total, err := h.svc.List(ctx.Request().Context(), p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, int(total))
}

// Delete 删除邀请码
// @Summary      删除邀请码
// @Tags         邀请码
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "邀请码 ID"
// @Success      200  {object}  pkg.Response
// @Router       /admin/invites/{id}/delete [post]
func (h *inviteHandler) Delete(ctx iris.Context) {
	id := ctx.Params().Get("id")
	if id == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "缺少邀请码 ID")
		return
	}
	if err := h.svc.Delete(ctx.Request().Context(), id); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
