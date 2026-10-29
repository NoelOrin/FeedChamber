package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type LikeHandler interface {
	ToggleContentLike(ctx iris.Context)
	GetContentLikeStatus(ctx iris.Context)
}

type likeHandler struct{ svc service.LikeService }

func newLikeHandler(svc service.LikeService) LikeHandler {
	return &likeHandler{svc: svc}
}

// ToggleContentLike 内容点赞/取消点赞
// @Summary      内容点赞/取消点赞
// @Description  切换内容的点赞状态（帖子正文或评论）
// @Tags         点赞
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        contentId path      string  true  "内容 ID"
// @Success      200  {object}  pkg.Response{data=object{liked=bool}}
// @Failure      401  {object}  pkg.Response
// @Router       /contents/{contentId}/like [post]
func (h *likeHandler) ToggleContentLike(ctx iris.Context) {
	contentID := ctx.Params().Get("contentId")
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	liked, err := h.svc.ToggleContentLike(ctx.Request().Context(), contentID, u.ID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"liked": liked})
}

// GetContentLikeStatus 获取内容点赞状态
// @Summary      获取内容点赞状态
// @Description  获取当前用户对内容的点赞状态和总点赞数
// @Tags         点赞
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        contentId path      string  true  "内容 ID"
// @Success      200  {object}  pkg.Response{data=object{liked=bool,count=int}}
// @Failure      401  {object}  pkg.Response
// @Router       /contents/{contentId}/like/status [post]
func (h *likeHandler) GetContentLikeStatus(ctx iris.Context) {
	contentID := ctx.Params().Get("contentId")
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	liked, err := h.svc.HasContentLiked(ctx.Request().Context(), contentID, u.ID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	count, err := h.svc.CountContentLikes(ctx.Request().Context(), contentID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"liked": liked, "count": count})
}
