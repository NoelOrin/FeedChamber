package handler

import (
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
	Delete(ctx iris.Context)
}

type userHandler struct {
	svc service.UserService
}

func newUserHandler(svc service.UserService) UserHandler {
	return &userHandler{svc: svc}
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
	userUUID := ctx.Values().GetString("user_uuid")
	if userUUID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	user, err := h.svc.GetByUUID(ctx.Request().Context(), userUUID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, user)
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
	user, err := h.svc.GetByID(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, user)
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
// @Router       /user/list [get]
func (h *userHandler) List(ctx iris.Context) {
	p := readListParams(ctx)
	users, err := h.svc.List(ctx.Request().Context(), p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"list": users})
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
// @Router       /user/search [get]
func (h *userHandler) Search(ctx iris.Context) {
	var req struct {
		Keyword  string `json:"keyword"`
		Role     string `json:"role"`
		Page     int    `json:"page"`
		PageSize int    `json:"pageSize"`
	}
	_ = ctx.ReadJSON(&req)
	if req.Keyword == "" {
		req.Keyword = ctx.URLParam("q")
	}
	if req.Role == "" {
		req.Role = ctx.URLParam("role")
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 20
	}
	users, err := h.svc.Search(ctx.Request().Context(), req.Keyword, req.Role, req.Page, req.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"list": users})
}

// UpdateProfile 更新当前用户信息
// @Summary      更新当前用户信息
// @Description  更新当前登录用户的头像、昵称、简介
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{username=string,avatar=string,bio=string} true "更新信息"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /user/profile [put]
func (h *userHandler) UpdateProfile(ctx iris.Context) {
	var req struct {
		Username *string `json:"username"`
		Avatar   *string `json:"avatar"`
		Bio      *string `json:"bio"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	userUUID := ctx.Values().GetString("user_uuid")
	if userUUID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	if err := h.svc.UpdateProfile(ctx.Request().Context(), userUUID, req.Username, req.Avatar, req.Bio); err != nil {
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
// @Router       /user/{id}/role [put]
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
// @Router       /user/{id} [delete]
func (h *userHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}