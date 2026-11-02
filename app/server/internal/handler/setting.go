package handler

import (
	"encoding/json"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
	"github.com/sirupsen/logrus"
)

type SettingHandler interface {
	GetAll(ctx iris.Context)
	GetPublic(ctx iris.Context)
	Get(ctx iris.Context)
	Set(ctx iris.Context)
	BatchSet(ctx iris.Context)
	Delete(ctx iris.Context)
}

type settingHandler struct {
	svc         service.SettingService
	auditLogSvc service.AuditLogService
}

func newSettingHandler(svc service.SettingService, auditLogSvc service.AuditLogService) SettingHandler {
	return &settingHandler{svc: svc, auditLogSvc: auditLogSvc}
}

// GetPublic 获取公开设置
// @Summary      获取公开设置
// @Description  获取无需鉴权即可读取的社区设置
// @Tags         系统设置
// @Produce      json
// @Success      200  {object}  pkg.Response{data=map[string]string}
// @Router       /settings/public [post]
func (h *settingHandler) GetPublic(ctx iris.Context) {
	all, err := h.svc.GetAll(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	public := make(map[string]string, len(model.PublicSettingKeys))
	for _, key := range model.PublicSettingKeys {
		if v, ok := all[key]; ok {
			public[key] = v
		}
	}
	pkg.Success(ctx, public)
}

// GetAll 获取所有系统设置
// @Summary      获取所有系统设置
// @Description  获取全部系统设置键值对
// @Tags         系统设置
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=map[string]string}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /admin/settings [post]
func (h *settingHandler) GetAll(ctx iris.Context) {
	settings, err := h.svc.GetAll(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, settings)
}

// Get 获取单个系统设置
// @Summary      获取单个系统设置
// @Description  根据 key 获取系统设置值
// @Tags         系统设置
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        key   path      string  true  "设置键名"
// @Success      200  {object}  pkg.Response{data=string}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /admin/settings/{key} [post]
func (h *settingHandler) Get(ctx iris.Context) {
	key := ctx.Params().Get("key")
	value, err := h.svc.Get(ctx.Request().Context(), key)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, value)
}

// Set 设置系统设置
// @Summary      设置系统设置
// @Description  创建或更新指定 key 的系统设置
// @Tags         系统设置
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        key   path      string  true  "设置键名"
// @Param        body  body      object{value=string}  true  "设置值"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /admin/settings/{key}/update [post]
func (h *settingHandler) Set(ctx iris.Context) {
	key := ctx.Params().Get("key")
	var req struct {
		Value string `json:"value"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.Set(ctx.Request().Context(), key, req.Value); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	detail := model.RedactSettingAuditDetail(key, req.Value)
	if err := h.auditLogSvc.Record(ctx.Request().Context(), currentUserID(ctx), "settings.update", key, detail); err != nil {
		logrus.Warnf("audit log write failed: %v", err)
	}
	pkg.Success(ctx, nil)
}

// BatchSet 批量设置系统设置
// @Summary      批量设置系统设置
// @Description  批量创建或更新系统设置
// @Tags         系统设置
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      object{settings=object}  true  "设置键值对"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /admin/settings/batch [post]
func (h *settingHandler) BatchSet(ctx iris.Context) {
	var req struct {
		Settings map[string]string `json:"settings"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if len(req.Settings) == 0 {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "设置项不能为空")
		return
	}
	if err := h.svc.BatchSet(ctx.Request().Context(), req.Settings); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	keys := make([]string, 0, len(req.Settings))
	for k := range req.Settings {
		keys = append(keys, k)
	}
	detail, _ := json.Marshal(map[string]any{"keys": keys})
	if err := h.auditLogSvc.Record(ctx.Request().Context(), currentUserID(ctx), "settings.batch_update", "settings", string(detail)); err != nil {
		logrus.Warnf("audit log write failed: %v", err)
	}
	pkg.Success(ctx, nil)
}

// Delete 删除系统设置
// @Summary      删除系统设置
// @Description  删除指定 key 的系统设置
// @Tags         系统设置
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        key   path      string  true  "设置键名"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /admin/settings/{key}/delete [post]
func (h *settingHandler) Delete(ctx iris.Context) {
	key := ctx.Params().Get("key")
	if err := h.svc.Delete(ctx.Request().Context(), key); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
