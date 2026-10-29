package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type BadgeHandler interface {
	List(ctx iris.Context)
	ListByUser(ctx iris.Context)
}

type badgeHandler struct{ svc service.BadgeService }

func newBadgeHandler(svc service.BadgeService) BadgeHandler {
	return &badgeHandler{svc: svc}
}

// 徽章列表
// @Summary      徽章列表
// @Description  列出全部可用徽章（公开接口）。
// @Tags         徽章
// @Produce      json
// @Success      200  {object}  pkg.Response
// @Router       /badges/list [post]
func (h *badgeHandler) List(ctx iris.Context) {
	items, err := h.svc.List(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, items)
}

// 用户徽章
// @Summary      用户徽章
// @Description  列出指定用户已获得的徽章。
// @Tags         徽章
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /user/{id}/badges/list [post]
func (h *badgeHandler) ListByUser(ctx iris.Context) {
	userID := ctx.Params().Get("id")
	items, err := h.svc.ListUserBadges(ctx.Request().Context(), userID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, items)
}
