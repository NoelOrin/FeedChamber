package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type FollowHandler interface {
	Follow(ctx iris.Context)
	Unfollow(ctx iris.Context)
	GetStatus(ctx iris.Context)
	GetFollowers(ctx iris.Context)
	GetFollowing(ctx iris.Context)
	GetCounts(ctx iris.Context)
}

type followHandler struct {
	svc service.FollowService
}

func newFollowHandler(svc service.FollowService) FollowHandler {
	return &followHandler{svc: svc}
}

// Follow 关注用户
// @Summary      关注用户
// @Description  关注指定用户
// @Tags         关注
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        userId path      string  true  "目标用户 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /users/{userId}/follow [post]
func (h *followHandler) Follow(ctx iris.Context) {
	followerID := ctx.Values().GetString("user_uuid")
	if followerID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	followingID := ctx.Params().Get("userId")
	if err := h.svc.Follow(ctx.Request().Context(), followerID, followingID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// Unfollow 取消关注
// @Summary      取消关注
// @Description  取消关注指定用户
// @Tags         关注
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        userId path      string  true  "目标用户 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /users/{userId}/follow [delete]
func (h *followHandler) Unfollow(ctx iris.Context) {
	followerID := ctx.Values().GetString("user_uuid")
	if followerID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	followingID := ctx.Params().Get("userId")
	if err := h.svc.Unfollow(ctx.Request().Context(), followerID, followingID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// GetStatus 获取关注状态
// @Summary      获取关注状态
// @Description  获取当前用户对指定用户的关注状态
// @Tags         关注
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        userId path      string  true  "目标用户 ID"
// @Success      200  {object}  pkg.Response{data=object{following=bool}}
// @Failure      401  {object}  pkg.Response
// @Router       /users/{userId}/follow [get]
func (h *followHandler) GetStatus(ctx iris.Context) {
	followerID := ctx.Values().GetString("user_uuid")
	if followerID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	followingID := ctx.Params().Get("userId")
	following, err := h.svc.IsFollowing(ctx.Request().Context(), followerID, followingID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"following": following})
}

// GetFollowers 粉丝列表
// @Summary      粉丝列表
// @Description  分页获取指定用户的粉丝列表
// @Tags         关注
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        userId   path      string  true   "用户 ID"
// @Param        page     query     int     false  "页码"  default(1)
// @Param        pageSize query     int     false  "每页数量"  default(20)
// @Success      200  {object}  pkg.Response{data=object{list=[]model.User}}
// @Failure      401  {object}  pkg.Response
// @Router       /users/{userId}/followers [get]
func (h *followHandler) GetFollowers(ctx iris.Context) {
	userID := ctx.Params().Get("userId")
	page := ctx.URLParamIntDefault("page", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	followers, err := h.svc.GetFollowers(ctx.Request().Context(), userID, page, pageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"list": followers})
}

// GetFollowing 关注列表
// @Summary      关注列表
// @Description  分页获取指定用户的关注列表
// @Tags         关注
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        userId   path      string  true   "用户 ID"
// @Param        page     query     int     false  "页码"  default(1)
// @Param        pageSize query     int     false  "每页数量"  default(20)
// @Success      200  {object}  pkg.Response{data=object{list=[]model.User}}
// @Failure      401  {object}  pkg.Response
// @Router       /users/{userId}/following [get]
func (h *followHandler) GetFollowing(ctx iris.Context) {
	userID := ctx.Params().Get("userId")
	page := ctx.URLParamIntDefault("page", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	following, err := h.svc.GetFollowing(ctx.Request().Context(), userID, page, pageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"list": following})
}

// GetCounts 获取关注计数
// @Summary      获取关注计数
// @Description  获取指定用户的粉丝数和关注数
// @Tags         关注
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        userId path      string  true  "用户 ID"
// @Success      200  {object}  pkg.Response{data=object{followers=int,following=int}}
// @Failure      401  {object}  pkg.Response
// @Router       /users/{userId}/counts [get]
func (h *followHandler) GetCounts(ctx iris.Context) {
	userID := ctx.Params().Get("userId")
	followers, err := h.svc.CountFollowers(ctx.Request().Context(), userID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	following, err := h.svc.CountFollowing(ctx.Request().Context(), userID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"followers": followers, "following": following})
}
