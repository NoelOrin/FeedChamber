package handler

import (
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

type FavoriteHandler interface {
	Toggle(ctx iris.Context)
	GetStatus(ctx iris.Context)
	ListByUser(ctx iris.Context)
}

type favoriteHandler struct{ svc service.FavoriteService }

func newFavoriteHandler(svc service.FavoriteService) FavoriteHandler {
	return &favoriteHandler{svc: svc}
}

// Toggle 帖子收藏/取消收藏
// @Summary      帖子收藏/取消收藏
// @Description  切换帖子的收藏状态
// @Tags         收藏
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Success      200  {object}  pkg.Response{data=object{favorited=bool}}
// @Failure      401  {object}  pkg.Response
// @Router       /posts/{postId}/favorite [post]
func (h *favoriteHandler) Toggle(ctx iris.Context) {
	postID := ctx.Params().Get("postId")
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	if postID == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	favorited, err := h.svc.Toggle(ctx.Request().Context(), postID, u.ID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"favorited": favorited})
}

// GetStatus 获取帖子收藏状态
// @Summary      获取帖子收藏状态
// @Description  获取当前用户对帖子的收藏状态
// @Tags         收藏
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Success      200  {object}  pkg.Response{data=object{favorited=bool}}
// @Failure      401  {object}  pkg.Response
// @Router       /posts/{postId}/favorite/status [post]
func (h *favoriteHandler) GetStatus(ctx iris.Context) {
	postID := ctx.Params().Get("postId")
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	if postID == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	favorited, err := h.svc.HasFavorited(ctx.Request().Context(), postID, u.ID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"favorited": favorited})
}

// ListByUser 用户收藏列表
// @Summary      用户收藏列表
// @Description  分页获取当前用户的收藏帖子 ID 列表
// @Tags         收藏
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page     query  int  false  "页码"  default(1)
// @Param        pageSize query  int  false  "每页数量"  default(20)
// @Success      200  {object}  pkg.Response{data=[]string}
// @Failure      401  {object}  pkg.Response
// @Router       /favorites/list [post]
func (h *favoriteHandler) ListByUser(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	p, err := readListParams(ctx)
	if err != nil {
		return
	}
	ids, total, err := h.svc.ListByUser(ctx.Request().Context(), u.ID, p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, ids, total)
}
