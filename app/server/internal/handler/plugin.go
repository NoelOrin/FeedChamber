package handler

import (
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

type PluginHandler interface {
	List(ctx iris.Context)
	ListInstalls(ctx iris.Context)
	Install(ctx iris.Context)
	Update(ctx iris.Context)
	SetEnabled(ctx iris.Context)
	Delete(ctx iris.Context)
	Uninstall(ctx iris.Context)

	// 来源与审批
	ListSources(ctx iris.Context)
	CreateSource(ctx iris.Context)
	ApproveSource(ctx iris.Context)
	RejectSource(ctx iris.Context)

	// 安装前预览 / 版本历史 / 回滚
	Preview(ctx iris.Context)
	ListVersions(ctx iris.Context)
	Rollback(ctx iris.Context)
}

type pluginHandler struct{ svc service.PluginService }

func newPluginHandler(svc service.PluginService) PluginHandler {
	return &pluginHandler{svc: svc}
}

// 插件列表
// @Summary      插件列表
// @Description  列出已注册的插件。
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
// @Description  经已审批来源安装插件。manifest 原文随请求提交，能力清单以安装时为准落库。
// @Tags         插件
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{plugin_id=string,source_id=string,manifest=string} true "安装请求"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/plugins/install [post]
func (h *pluginHandler) Install(ctx iris.Context) {
	var input service.PluginInstallInput
	if err := ctx.ReadJSON(&input); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	install, err := h.svc.InstallAs(ctx.Request().Context(), input, currentUserID(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, install)
}

// 更新插件
// @Summary      更新插件
// @Description  修改插件的声明式元数据（名称、版本、描述等）。不改变安装时授予的能力快照。
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
	if err := h.svc.UpdateAs(ctx.Request().Context(), input, currentUserID(ctx)); err != nil {
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
	if err := h.svc.SetEnabledAs(ctx.Request().Context(), ctx.Params().Get("id"), body.Enabled, currentUserID(ctx)); err != nil {
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
	if err := h.svc.DeleteAs(ctx.Request().Context(), ctx.Params().Get("id"), currentUserID(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// 插件来源列表
// @Summary      插件来源列表
// @Description  列出全部插件来源及其审批状态。
// @Tags         插件
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/plugins/sources/list [post]
func (h *pluginHandler) ListSources(ctx iris.Context) {
	items, err := h.svc.ListSources(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, items)
}

// 新增插件来源
// @Summary      新增插件来源
// @Description  登记一条插件来源，初始状态为待审批。操作留审计。
// @Tags         插件
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{name=string,kind=string,url=string} true "来源信息"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/plugins/sources/create [post]
func (h *pluginHandler) CreateSource(ctx iris.Context) {
	var body struct {
		Name string `json:"name"`
		Kind string `json:"kind"`
		URL  string `json:"url"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	src, err := h.svc.CreateSource(ctx.Request().Context(), body.Name, body.Kind, body.URL, currentUserID(ctx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, src)
}

// 审批插件来源
// @Summary      审批插件来源
// @Description  将一条待审批的插件来源标记为已审批。未审批来源不可安装。
// @Tags         插件
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "来源 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/plugins/sources/{id}/approve [post]
func (h *pluginHandler) ApproveSource(ctx iris.Context) {
	if err := h.svc.ApproveSource(ctx.Request().Context(), ctx.Params().Get("id"), currentUserID(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// 拒绝插件来源
// @Summary      拒绝插件来源
// @Description  将一条待审批的插件来源标记为已拒绝。已拒绝来源不可安装。
// @Tags         插件
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "来源 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/plugins/sources/{id}/reject [post]
func (h *pluginHandler) RejectSource(ctx iris.Context) {
	if err := h.svc.RejectSource(ctx.Request().Context(), ctx.Params().Get("id"), currentUserID(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// 插件运行状态列表
// @Summary      插件运行状态列表
// @Description  列出全部安装记录（status / restarts / last_error），供后台展示运行时状态。
// @Tags         插件
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/plugins/installs/list [post]
func (h *pluginHandler) ListInstalls(ctx iris.Context) {
	items, err := h.svc.ListInstalls(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, items)
}

// 卸载插件
// @Summary      卸载插件
// @Description  停止插件进程并删除插件登记与安装记录（两张表），操作留审计。
// @Tags         插件
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/plugins/{id}/uninstall [post]
func (h *pluginHandler) Uninstall(ctx iris.Context) {
	if err := h.svc.UninstallAs(ctx.Request().Context(), ctx.Params().Get("id"), currentUserID(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// 安装前预览
// @Summary      安装前预览
// @Description  返回能力清单、依赖清单与来源地址，不落库、不起进程。
// @Tags         插件
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body service.PluginInstallInput true "安装请求"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/plugins/preview [post]
func (h *pluginHandler) Preview(ctx iris.Context) {
	var input service.PluginInstallInput
	if err := ctx.ReadJSON(&input); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	prev, err := h.svc.Preview(ctx.Request().Context(), input)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, prev)
}

// 版本历史
// @Summary      插件版本历史
// @Description  列出某插件的历次安装/升级/回滚快照（最新在前）。
// @Tags         插件
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{plugin_id=string} true "插件 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/plugins/versions/list [post]
func (h *pluginHandler) ListVersions(ctx iris.Context) {
	var body struct {
		PluginID string `json:"plugin_id"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	items, err := h.svc.ListVersions(ctx.Request().Context(), body.PluginID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, items)
}

// 回滚插件
// @Summary      回滚到历史版本
// @Description  按指定历史版本的 manifest 与来源重新安装（停进程 → 重新拉取 → 起进程）。
// @Tags         插件
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "插件 ID"
// @Param        body body object{version_id=string} true "目标版本记录 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/plugins/{id}/rollback [post]
func (h *pluginHandler) Rollback(ctx iris.Context) {
	var body struct {
		VersionID string `json:"version_id"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.RollbackAs(ctx.Request().Context(), ctx.Params().Get("id"),
		body.VersionID, currentUserID(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
