package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type AdminHandler interface {
	GetStats(ctx iris.Context)
}

type adminHandler struct {
	svc service.AdminService
}

func newAdminHandler(svc service.AdminService) AdminHandler {
	return &adminHandler{svc: svc}
}

// GetStats 获取论坛统计数据
// @Summary      获取论坛统计数据
// @Description  获取用户、帖子、评论、分类的总数及今日新增数量
// @Tags         管理后台
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=service.AdminStats}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /admin/stats [get]
func (h *adminHandler) GetStats(ctx iris.Context) {
	stats, err := h.svc.GetStats(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, stats)
}
