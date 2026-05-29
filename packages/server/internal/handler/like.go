package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type LikeHandler interface {
	TogglePostLike(ctx iris.Context)
	ToggleCommentLike(ctx iris.Context)
	GetPostLikeStatus(ctx iris.Context)
	GetCommentLikeStatus(ctx iris.Context)
}

type likeHandler struct{ svc service.LikeService }

func newLikeHandler(svc service.LikeService) LikeHandler {
	return &likeHandler{svc: svc}
}

// TogglePostLike 帖子点赞/取消点赞
// @Summary      帖子点赞/取消点赞
// @Description  切换帖子的点赞状态
// @Tags         点赞
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Success      200  {object}  pkg.Response{data=object{liked=bool}}
// @Failure      401  {object}  pkg.Response
// @Router       /posts/{postId}/like [post]
func (h *likeHandler) TogglePostLike(ctx iris.Context) {
	postID := ctx.Params().Get("postId")
	userID := ctx.Values().GetString("user_uuid")
	if userID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	liked, err := h.svc.TogglePostLike(ctx.Request().Context(), postID, userID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"liked": liked})
}

// ToggleCommentLike 评论点赞/取消点赞
// @Summary      评论点赞/取消点赞
// @Description  切换评论的点赞状态
// @Tags         点赞
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        commentId path      string  true  "评论 ID"
// @Success      200  {object}  pkg.Response{data=object{liked=bool}}
// @Failure      401  {object}  pkg.Response
// @Router       /comments/{commentId}/like [post]
func (h *likeHandler) ToggleCommentLike(ctx iris.Context) {
	commentID := ctx.Params().Get("commentId")
	userID := ctx.Values().GetString("user_uuid")
	if userID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	liked, err := h.svc.ToggleCommentLike(ctx.Request().Context(), commentID, userID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"liked": liked})
}

// GetPostLikeStatus 获取帖子点赞状态
// @Summary      获取帖子点赞状态
// @Description  获取当前用户对帖子的点赞状态和总点赞数
// @Tags         点赞
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId path      string  true  "帖子 ID"
// @Success      200  {object}  pkg.Response{data=object{liked=bool,count=int}}
// @Failure      401  {object}  pkg.Response
// @Router       /posts/{postId}/like [get]
func (h *likeHandler) GetPostLikeStatus(ctx iris.Context) {
	postID := ctx.Params().Get("postId")
	userID := ctx.Values().GetString("user_uuid")
	if userID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	liked, err := h.svc.HasPostLiked(ctx.Request().Context(), postID, userID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	count, err := h.svc.CountPostLikes(ctx.Request().Context(), postID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"liked": liked, "count": count})
}

// GetCommentLikeStatus 获取评论点赞状态
// @Summary      获取评论点赞状态
// @Description  获取当前用户对评论的点赞状态和总点赞数
// @Tags         点赞
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        commentId path      string  true  "评论 ID"
// @Success      200  {object}  pkg.Response{data=object{liked=bool,count=int}}
// @Failure      401  {object}  pkg.Response
// @Router       /comments/{commentId}/like [get]
func (h *likeHandler) GetCommentLikeStatus(ctx iris.Context) {
	commentID := ctx.Params().Get("commentId")
	userID := ctx.Values().GetString("user_uuid")
	if userID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}
	liked, err := h.svc.HasCommentLiked(ctx.Request().Context(), commentID, userID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	count, err := h.svc.CountCommentLikes(ctx.Request().Context(), commentID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"liked": liked, "count": count})
}
