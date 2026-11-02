package handler

import (
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

type PluginHandler interface {
	List(ctx iris.Context)
	Install(ctx iris.Context)
	Update(ctx iris.Context)
	SetEnabled(ctx iris.Context)
	Delete(ctx iris.Context)
}

type pluginHandler struct{ svc service.PluginService }

func newPluginHandler(svc service.PluginService) PluginHandler {
	return &pluginHandler{svc: svc}
}

// 插件列表
// @Summary      插件列表
// @Description  列出已注册的插件（仅声明式元数据，无可执行扩展点）。
// @Tags         插件
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/plugins/list [post]
func (h *pluginHandler) List(ctx iris.Context) {
	items, err := h.svc.List(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, items)
}

// 安装插件
// @Summary      安装插件
// @Description  登记一个插件的元数据。
// @Tags         插件
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{name=string,version=string,description=string} true "插件元数据"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/plugins/install [post]
func (h *pluginHandler) Install(ctx iris.Context) {
	var input service.PluginInput
	if err := ctx.ReadJSON(&input); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	plugin, err := h.svc.Install(ctx.Request().Context(), input)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, plugin)
}

// 更新插件
// @Summary      更新插件
// @Description  修改插件元数据。
// @Tags         插件
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Param        body body object{name=string,version=string,description=string} false "更新字段"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/plugins/{id}/update [post]
func (h *pluginHandler) Update(ctx iris.Context) {
	var input service.PluginInput
	if err := ctx.ReadJSON(&input); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if input.ID == "" {
		input.ID = ctx.Params().Get("id")
	}
	if err := h.svc.Update(ctx.Request().Context(), input); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// 启停插件
// @Summary      启停插件
// @Description  切换插件的启用状态。
// @Tags         插件
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Param        body body object{enabled=bool} true "启用状态"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/plugins/{id}/status [post]
func (h *pluginHandler) SetEnabled(ctx iris.Context) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.SetEnabled(ctx.Request().Context(), ctx.Params().Get("id"), body.Enabled); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// 删除插件
// @Summary      删除插件
// @Description  删除插件登记记录。
// @Tags         插件
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/plugins/{id}/delete [post]
func (h *pluginHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
