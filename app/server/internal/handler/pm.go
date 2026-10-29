package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

// PMHandler handles private-message RPC endpoints.
type PMHandler interface {
	ListConversations(ctx iris.Context)
	CreateConversation(ctx iris.Context)
	ListMessages(ctx iris.Context)
	SendMessage(ctx iris.Context)
	MarkRead(ctx iris.Context)
}

type pmHandler struct {
	svc service.PMService
}

func newPMHandler(svc service.PMService) PMHandler {
	return &pmHandler{svc: svc}
}

// ListConversations 会话列表
// @Summary      私信会话列表
// @Description  分页获取当前用户的私信会话
// @Tags         私信
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body ListParams false "分页参数"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /messages/conversations/list [post]
func (h *pmHandler) ListConversations(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	p, err := readListParams(ctx)
	if err != nil {
		return
	}
	items, total, err := h.svc.ListConversations(ctx.Request().Context(), u.ID, p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, total)
}

type createConversationReq struct {
	PeerUserID string `json:"peer_user_id"`
	Body       string `json:"body"`
}

// CreateConversation 发起/复用私信会话
// @Summary      发起私信
// @Description  与目标用户开启直接会话并发送首条消息；已有会话则复用且不自动发消息
// @Tags         私信
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body createConversationReq true "目标用户与首条消息"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /messages/conversations/create [post]
func (h *pmHandler) CreateConversation(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	var req createConversationReq
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "invalid request body")
		return
	}
	conv, msg, err := h.svc.StartDirect(ctx.Request().Context(), u.ID, req.PeerUserID, req.Body)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{
		"conversation": conv,
		"message":      msg,
	})
}

// ListMessages 会话消息列表
// @Summary      私信消息列表
// @Description  分页获取指定会话的消息（第 1 页为最新窗口，响应按时间正序）
// @Tags         私信
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string true "会话 ID"
// @Param        body body ListParams false "分页参数"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /messages/conversations/{id}/messages/list [post]
func (h *pmHandler) ListMessages(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	id := ctx.Params().Get("id")
	p, err := readListParams(ctx)
	if err != nil {
		return
	}
	items, total, err := h.svc.ListMessages(ctx.Request().Context(), u.ID, id, p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, total)
}

type sendMessageReq struct {
	Body string `json:"body"`
}

// SendMessage 发送私信
// @Summary      发送私信
// @Description  在指定会话中发送一条消息
// @Tags         私信
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string true "会话 ID"
// @Param        body body sendMessageReq true "消息内容"
// @Success      200  {object}  pkg.Response
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /messages/conversations/{id}/messages/create [post]
func (h *pmHandler) SendMessage(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	id := ctx.Params().Get("id")
	var req sendMessageReq
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "invalid request body")
		return
	}
	msg, err := h.svc.Send(ctx.Request().Context(), u.ID, id, req.Body)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, msg)
}

// MarkRead 标记会话已读
// @Summary      标记私信已读
// @Description  将指定会话标记为当前用户已读
// @Tags         私信
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "会话 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /messages/conversations/{id}/read [post]
func (h *pmHandler) MarkRead(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	id := ctx.Params().Get("id")
	if err := h.svc.MarkRead(ctx.Request().Context(), u.ID, id); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
