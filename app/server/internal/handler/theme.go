package handler

import (
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

type ThemeHandler interface {
	List(ctx iris.Context)
	GetPreference(ctx iris.Context)
	SetPreference(ctx iris.Context)
}

type themeHandler struct{ svc service.ThemeService }

func newThemeHandler(svc service.ThemeService) ThemeHandler {
	return &themeHandler{svc: svc}
}

// 主题列表
// @Summary      主题列表
// @Description  列出可选主题（公开接口）。
// @Tags         主题
// @Produce      json
// @Success      200  {object}  pkg.Response
// @Router       /themes/list [post]
func (h *themeHandler) List(ctx iris.Context) {
	pkg.Success(ctx, h.svc.List())
}

// 读取主题偏好
// @Summary      读取主题偏好
// @Description  读取当前用户的主题偏好。
// @Tags         主题
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /themes/preference [post]
func (h *themeHandler) GetPreference(ctx iris.Context) {
	pref, err := h.svc.GetPreference(ctx.Request().Context(), currentUserID(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, map[string]string{"theme": pref})
}

// 设置主题偏好
// @Summary      设置主题偏好
// @Description  保存当前用户的主题偏好。
// @Tags         主题
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{theme=string} true "主题标识"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /themes/preference/update [post]
func (h *themeHandler) SetPreference(ctx iris.Context) {
	var body struct {
		Theme string `json:"theme"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.SetPreference(ctx.Request().Context(), currentUserID(ctx), body.Theme); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
