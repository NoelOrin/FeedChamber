package handler

import (
	"context"
	"errors"
	"io"

	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

// HistoryHandler 是「我的浏览记录」出口。
//
// 三件事都只作用于当前会话用户：访问者身份一律由 service 从 ctx 取
// （WithViewerID），这里 MUST NOT 把 userID 做成参数——那等于开出越权读写
// 别人阅读历史的口子。列表的可见性过滤在 SQL 里完成，见
// repository.PostRepository.ListReadByUser。
type HistoryHandler interface {
	List(ctx iris.Context)
	Delete(ctx iris.Context)
	Clear(ctx iris.Context)
}

type historyHandler struct{ svc service.PostService }

func newHistoryHandler(svc service.PostService) HistoryHandler {
	return &historyHandler{svc: svc}
}

// viewerCtx 把会话身份灌进 service 侧的读取上下文（与 postHandler 各出口一致：
// viewer id + role，等级与版面可见性据此判定）。浏览记录的访问者只能是
// ctx 里的这个人，所以身份不经参数传递。
func viewerCtx(ctx iris.Context) context.Context {
	return service.WithViewerRole(
		service.WithViewerID(ctx.Request().Context(), currentUserID(ctx)),
		currentRole(ctx),
	)
}

// List 我的浏览记录
// @Summary      我的浏览记录
// @Description  按最近浏览时间倒序返回当前用户打开过的主题，含每篇的续读游标（last_content_id）；被删除、待审、拒稿或等级/版面不可见的主题一律不出现，total 同样按可见结果计
// @Tags         浏览记录
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{page=integer,page_size=integer} false "分页参数（可空 body）"
// @Success      200  {object}  pkg.Response{data=[]model.PostListItem}
// @Failure      401  {object}  pkg.Response
// @Router       /history/list [post]
func (h *historyHandler) List(ctx iris.Context) {
	if _, ok := requireUser(ctx); !ok {
		return
	}
	var body struct {
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
	}
	// 空 body 是合法请求（默认分页），不能打成 400：ReadJSON 在无 body 时返回 io.EOF。
	if err := ctx.ReadJSON(&body); err != nil && !errors.Is(err, io.EOF) {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	body.Page, body.PageSize = pkg.NormalizePagination(body.Page, body.PageSize)
	items, total, err := h.svc.ListHistory(viewerCtx(ctx), body.Page, body.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, total)
}

// Delete 删除一条浏览记录
// @Summary      删除一条浏览记录
// @Description  删除当前用户对某主题的浏览记录；记录本就不存在时同样返回成功（幂等）
// @Tags         浏览记录
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        postId  path      string  true  "主题 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /history/{postId}/delete [post]
func (h *historyHandler) Delete(ctx iris.Context) {
	if _, ok := requireUser(ctx); !ok {
		return
	}
	postId := ctx.Params().Get("postId")
	if postId == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "主题 ID 不能为空")
		return
	}
	if err := h.svc.DeleteHistory(viewerCtx(ctx), postId); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// Clear 清空浏览记录
// @Summary      清空浏览记录
// @Description  清空当前用户的全部浏览记录；不影响主题本身，也不影响其他用户
// @Tags         浏览记录
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /history/clear [post]
func (h *historyHandler) Clear(ctx iris.Context) {
	if _, ok := requireUser(ctx); !ok {
		return
	}
	if err := h.svc.ClearHistory(viewerCtx(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
