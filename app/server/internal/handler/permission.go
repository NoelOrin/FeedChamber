package handler

import (
	"encoding/json"

	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
	"github.com/sirupsen/logrus"
)

type PermissionHandler interface {
	ListPermissions(ctx iris.Context)
	GetRolePermissions(ctx iris.Context)
	SyncRolePermissions(ctx iris.Context)
	GetSpaceRolePermissions(ctx iris.Context)
	GetEffectiveSpaceRolePermissions(ctx iris.Context)
	SyncSpaceRolePermissions(ctx iris.Context)
	ClearSpaceRolePermissions(ctx iris.Context)
	GetSpacePermissionsTree(ctx iris.Context)
}

type permissionHandler struct {
	permSvc     service.PermissionService
	auditLogSvc service.AuditLogService
}

func newPermissionHandler(permSvc service.PermissionService, auditLogSvc service.AuditLogService) PermissionHandler {
	return &permissionHandler{permSvc: permSvc, auditLogSvc: auditLogSvc}
}

// ListPermissions 权限列表
// @Summary      权限列表
// @Description  获取所有权限定义列表
// @Tags         权限
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=[]model.Permission}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /permission/list [post]
func (h *permissionHandler) ListPermissions(ctx iris.Context) {
	perms, err := h.permSvc.ListPermissions(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, perms)
}

// GetRolePermissions 获取角色权限
// @Summary      获取角色权限
// @Description  获取指定角色的权限码列表
// @Tags         权限
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        role query     string  true  "角色名称"
// @Success      200  {object}  pkg.Response{data=object{role=string,permissions=[]string}}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /permission/role [post]
func (h *permissionHandler) GetRolePermissions(ctx iris.Context) {
	var req struct {
		Role string `json:"role"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if req.Role == "" {
		req.Role = ctx.URLParam("role")
	}
	if req.Role == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "角色不能为空")
		return
	}
	codes := h.permSvc.GetRolePermissions(req.Role)
	pkg.Success(ctx, iris.Map{
		"role":        req.Role,
		"permissions": codes,
	})
}

// SyncRolePermissions 同步角色权限
// @Summary      同步角色权限
// @Description  为指定角色同步设置权限列表
// @Tags         权限
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{role=string,permissions=[]string} true "角色权限信息"
// @Success      200  {object}  pkg.Response{data=object{role=string,permissions=[]string}}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /permission/sync [post]
func (h *permissionHandler) SyncRolePermissions(ctx iris.Context) {
	var req struct {
		Role        string   `json:"role"`
		Permissions []string `json:"permissions"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if req.Role == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "角色不能为空")
		return
	}

	if err := h.permSvc.SyncRolePermissions(ctx.Request().Context(), req.Role, req.Permissions); err != nil {
		pkg.HandleError(ctx, err)
		return
	}

	// Audit log for permission changes; non-fatal if logging fails
	actorID := currentUserID(ctx)
	detail, _ := json.Marshal(map[string]any{"permissions": req.Permissions})
	if err := h.auditLogSvc.Log(ctx.Request().Context(), actorID, "permission.sync", req.Role, string(detail)); err != nil {
		logrus.Warnf("audit log write failed: %v", err)
	}

	pkg.Success(ctx, iris.Map{
		"role":        req.Role,
		"permissions": req.Permissions,
	})
}

// GetSpaceRolePermissions 获取版块角色权限
// @Summary      获取版块角色权限
// @Tags         权限
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Router       /permission/space [post]
func (h *permissionHandler) GetSpaceRolePermissions(ctx iris.Context) {
	var req struct {
		SpaceID string `json:"space_id"`
		Role    string `json:"role"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if req.SpaceID == "" || req.Role == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "版块和角色不能为空")
		return
	}
	codes, hasOverride, err := h.permSvc.GetSpaceRolePermissions(ctx.Request().Context(), req.SpaceID, req.Role)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{
		"space_id":     req.SpaceID,
		"role":         req.Role,
		"permissions":  codes,
		"has_override": hasOverride,
	})
}

// SyncSpaceRolePermissions 设置版块角色权限覆盖
// @Summary      设置版块角色权限覆盖
// @Tags         权限
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Router       /permission/space/sync [post]
func (h *permissionHandler) SyncSpaceRolePermissions(ctx iris.Context) {
	var req struct {
		SpaceID     string   `json:"space_id"`
		Role        string   `json:"role"`
		Permissions []string `json:"permissions"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if req.SpaceID == "" || req.Role == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "版块和角色不能为空")
		return
	}
	if len(req.Permissions) == 0 {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "覆盖权限不能为空；如需回退全局请用清除接口")
		return
	}
	if err := h.permSvc.SyncSpaceRolePermissions(ctx.Request().Context(), req.SpaceID, req.Role, req.Permissions); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	actorID := currentUserID(ctx)
	detail, _ := json.Marshal(map[string]any{"space_id": req.SpaceID, "permissions": req.Permissions})
	if err := h.auditLogSvc.Log(ctx.Request().Context(), actorID, "permission.space.sync", req.Role, string(detail)); err != nil {
		logrus.Warnf("audit log write failed: %v", err)
	}
	pkg.Success(ctx, iris.Map{"space_id": req.SpaceID, "role": req.Role, "permissions": req.Permissions})
}

// ClearSpaceRolePermissions 清除版块角色权限覆盖
// @Summary      清除版块角色权限覆盖
// @Tags         权限
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Router       /permission/space/clear [post]
func (h *permissionHandler) ClearSpaceRolePermissions(ctx iris.Context) {
	var req struct {
		SpaceID string `json:"space_id"`
		Role    string `json:"role"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if req.SpaceID == "" || req.Role == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "版块和角色不能为空")
		return
	}
	if err := h.permSvc.ClearSpaceRoleOverride(ctx.Request().Context(), req.SpaceID, req.Role); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	actorID := currentUserID(ctx)
	detail, _ := json.Marshal(map[string]any{"space_id": req.SpaceID})
	if err := h.auditLogSvc.Log(ctx.Request().Context(), actorID, "permission.space.clear", req.Role, string(detail)); err != nil {
		logrus.Warnf("audit log write failed: %v", err)
	}
	pkg.Success(ctx, nil)
}

// GetEffectiveSpaceRolePermissions 获取层级继承后的有效权限（NGA）
// @Summary      获取版块有效权限（继承）
// @Description  返回指定版面+角色在层级继承后的有效权限及来源（最近覆盖或全局）
// @Tags         权限
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{space_id=string,role=string} true "查询"
// @Success      200  {object}  pkg.Response
// @Router       /permission/space/effective [post]
func (h *permissionHandler) GetEffectiveSpaceRolePermissions(ctx iris.Context) {
	var req struct {
		SpaceID string `json:"space_id"`
		Role    string `json:"role"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if req.SpaceID == "" || req.Role == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "版块和角色不能为空")
		return
	}
	codes, sourceID, hasOverride, inherited, err := h.permSvc.GetEffectiveSpaceRolePermissions(ctx.Request().Context(), req.SpaceID, req.Role)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	if codes == nil {
		codes = []string{}
	}
	pkg.Success(ctx, iris.Map{
		"space_id":        req.SpaceID,
		"role":            req.Role,
		"permissions":     codes,
		"source_space_id": sourceID,
		"has_override":    hasOverride,
		"inherited":       inherited,
	})
}

// GetSpacePermissionsTree 获取指定角色在所有版面上的有效权限树（NGA）
// @Summary      获取版面权限树（有效）
// @Description  若 space_ids 为空则返回空映射；需显式传入要查询的版面 ID 列表
// @Tags         权限
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{role=string,space_ids=[]string} true "角色及版面列表"
// @Success      200  {object}  pkg.Response
// @Router       /permission/space/tree [post]
func (h *permissionHandler) GetSpacePermissionsTree(ctx iris.Context) {
	var req struct {
		Role     string   `json:"role"`
		SpaceIDs []string `json:"space_ids"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		// 允许空 body 时从 query 取 role（兼容 GET 风格）
		req.Role = ctx.URLParam("role")
		if req.Role == "" {
			pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
			return
		}
	}
	// query 覆盖：若 body 中 role 为空则尝试 query
	if req.Role == "" {
		req.Role = ctx.URLParam("role")
	}
	if req.Role == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "角色不能为空")
		return
	}
	codesMap := map[string][]string{}
	if len(req.SpaceIDs) > 0 {
		m, err := h.permSvc.ListSpacesWithEffectivePermissions(ctx.Request().Context(), req.Role, req.SpaceIDs)
		if err != nil {
			pkg.HandleError(ctx, err)
			return
		}
		codesMap = m
	}
	pkg.Success(ctx, iris.Map{"role": req.Role, "permissions": codesMap})
}
