package handler

import (
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type UserHandler interface {
	GetProfile(ctx iris.Context)
	GetByID(ctx iris.Context)
	List(ctx iris.Context)
	Search(ctx iris.Context)
	UpdateProfile(ctx iris.Context)
	UpdateRole(ctx iris.Context)
	UpdateTrustLevel(ctx iris.Context)
	Delete(ctx iris.Context)
	GetPublicProfile(ctx iris.Context)
	GetActivity(ctx iris.Context)
}

type userHandler struct {
	svc       service.UserService
	followSvc service.FollowService
	audit     service.AuditLogService
	trustSvc  service.TrustService
}

func newUserHandler(svc service.UserService, followSvc service.FollowService, audit service.AuditLogService, trustSvc service.TrustService) UserHandler {
	return &userHandler{svc: svc, followSvc: followSvc, audit: audit, trustSvc: trustSvc}
}

func toUserResponses(users []model.User) []model.UserResponse {
	result := make([]model.UserResponse, len(users))
	for i, u := range users {
		result[i] = u.ToResponse()
	}
	return result
}

// GetProfile 获取当前用户信息
// @Summary      获取当前用户信息
// @Description  获取当前登录用户的详细信息
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=model.User}
// @Failure      401  {object}  pkg.Response
// @Router       /user/profile [get]
func (h *userHandler) GetProfile(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	user, err := h.svc.GetByID(ctx.Request().Context(), u.ID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, user.ToResponse())
}

// GetByID 获取指定用户信息
// @Summary      获取指定用户信息
// @Description  根据用户 ID 获取用户详细信息
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "用户 ID"
// @Success      200  {object}  pkg.Response{data=model.User}
// @Failure      401  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /user/{id} [get]
func (h *userHandler) GetByID(ctx iris.Context) {
	id := ctx.Params().Get("id")
	if id == "" {
		pkg.Fail(ctx, pkg.NOT_FOUND, "用户 ID 不能为空")
		return
	}
	user, err := h.svc.GetByID(ctx.Request().Context(), id)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, user.ToResponse())
}

// List 用户列表
// @Summary      用户列表
// @Description  分页获取用户列表
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page     query  int  false  "页码"  default(1)
// @Param        pageSize query  int  false  "每页数量"  default(20)
// @Success      200  {object}  pkg.Response{data=object{list=[]model.User}}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /user/list [post]
func (h *userHandler) List(ctx iris.Context) {
	p, err := readListParams(ctx)
	if err != nil {
		return
	}
	users, err := h.svc.List(ctx.Request().Context(), p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"items": toUserResponses(users)})
}

// Search 搜索用户
// @Summary      搜索用户
// @Description  根据关键词和角色筛选用户
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        q        query  string  false  "搜索关键词（用户名或邮箱）"
// @Param        role     query  string  false  "角色筛选"
// @Param        page     query  int     false  "页码"  default(1)
// @Param        pageSize query  int     false  "每页数量"  default(20)
// @Success      200  {object}  pkg.Response{data=object{list=[]model.User}}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /user/search [post]
func (h *userHandler) Search(ctx iris.Context) {
	var req struct {
		Keyword  string `json:"keyword"`
		Role     string `json:"role"`
		Page     int    `json:"page"`
		PageSize int    `json:"page_size"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if req.Keyword == "" {
		req.Keyword = ctx.URLParam("q")
	}
	if req.Role == "" {
		req.Role = ctx.URLParam("role")
	}
	req.Page, req.PageSize = pkg.NormalizePagination(req.Page, req.PageSize)
	users, err := h.svc.Search(ctx.Request().Context(), req.Keyword, req.Role, req.Page, req.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"items": toUserResponses(users)})
}

// UpdateProfile 更新当前用户信息
// @Summary      更新当前用户信息
// @Description  更新当前登录用户的头像、昵称、简介
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{username=string,display_name=string,avatar=string,bio=string} true "更新信息"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /user/profile/update [post]
func (h *userHandler) UpdateProfile(ctx iris.Context) {
	var req struct {
		Username      *string `json:"username"`
		DisplayName   *string `json:"display_name"`
		Avatar        *string `json:"avatar"`
		Bio           *string `json:"bio"`
		CoverImage    *string `json:"cover_image"`
		ProfileLayout *string `json:"profile_layout"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	if err := h.svc.UpdateProfile(ctx.Request().Context(), u.ID, req.Username, req.DisplayName, req.Avatar, req.Bio, req.CoverImage, req.ProfileLayout); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// UpdateRole 修改用户角色
// @Summary      修改用户角色
// @Description  修改指定用户的角色
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "用户 ID"
// @Param        body body      object{role=string}  true  "角色信息"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /user/{id}/role/update [post]
func (h *userHandler) UpdateRole(ctx iris.Context) {
	var req struct {
		Role string `json:"role"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.UpdateRole(ctx.Request().Context(), ctx.Params().Get("id"), req.Role); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), currentUserID(ctx), "user.role_update", ctx.Params().Get("id"), "role="+req.Role)
	pkg.Success(ctx, nil)
}

// UpdateTrustLevel 更新用户信任等级
// @Summary      更新用户信任等级
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Router       /user/{id}/trust_level/update [post]
func (h *userHandler) UpdateTrustLevel(ctx iris.Context) {
	var req struct {
		Level int `json:"level"`
		// Manual 缺省为 true（手工设级）；显式传 false 表示解除手工锁定并回到自动求值。
		Manual *bool `json:"manual"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if h.trustSvc == nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "信任等级服务未启用")
		return
	}
	manual := true
	if req.Manual != nil {
		manual = *req.Manual
	}
	if err := h.trustSvc.SetManualLevel(ctx.Request().Context(), ctx.Params().Get("id"), req.Level, manual); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// Delete 删除用户
// @Summary      删除用户
// @Description  删除指定用户
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "用户 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /user/{id}/delete [post]
func (h *userHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), currentUserID(ctx), "user.delete", ctx.Params().Get("id"), "")
	pkg.Success(ctx, nil)
}

// GetPublicProfile 获取用户公开资料
// @Summary      获取用户公开资料
// @Description  根据用户 ID 获取用户公开资料（含帖子/评论/点赞统计）
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "用户 ID"
// @Success      200  {object}  pkg.Response{data=model.PublicProfileResponse}
// @Failure      404  {object}  pkg.Response
// @Router       /user/{id}/profile [get]
func (h *userHandler) GetPublicProfile(ctx iris.Context) {
	id := ctx.Params().Get("id")
	profile, err := h.svc.GetPublicProfile(ctx.Request().Context(), id)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	currentUserID, _ := ctx.Values().Get(pkg.CtxUserID).(string) // safe zero-value ("") on absent key
	if currentUserID != "" && h.followSvc != nil {
		isFollowing, _ := h.followSvc.IsFollowing(ctx.Request().Context(), currentUserID, id)
		profile.IsFollowing = isFollowing
	}
	pkg.Success(ctx, profile)
}

// GetActivity 获取用户活跃度数据
// @Summary      获取用户活跃度数据
// @Description  获取指定用户某年的活跃度热力图数据
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "用户 ID"
// @Param        year body      int     true  "年份"
// @Success      200  {object}  pkg.Response{data=model.ActivityResponse}
// @Failure      401  {object}  pkg.Response
// @Router       /user/{id}/activity [post]
func (h *userHandler) GetActivity(ctx iris.Context) {
	id := ctx.Params().Get("id")
	var req struct {
		Year int `json:"year"`
	}
	if err := ctx.ReadJSON(&req); err != nil || req.Year == 0 {
		req.Year = time.Now().Year()
	}
	activity, err := h.svc.GetActivity(ctx.Request().Context(), id, req.Year)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, activity)
}
