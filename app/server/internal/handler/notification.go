package handler

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"feedchamber/internal/metrics"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/kvstore"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

type NotificationHandler interface {
	List(ctx iris.Context)
	CountUnread(ctx iris.Context)
	MarkRead(ctx iris.Context)
	MarkAllRead(ctx iris.Context)
	Delete(ctx iris.Context)
	Stream(ctx iris.Context)
}

type notificationHandler struct {
	svc        service.NotificationService
	kv         kvstore.Store
	settingSvc service.SettingService
}

func newNotificationHandler(svc service.NotificationService, kv kvstore.Store, settingSvc service.SettingService) NotificationHandler {
	return &notificationHandler{svc: svc, kv: kv, settingSvc: settingSvc}
}

// List 获取通知列表
// @Summary      获取通知列表
// @Description  分页获取当前用户的通知列表
// @Tags         通知
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page     query     int  false  "页码"      default(1)
// @Param        pageSize query     int  false  "每页数量"  default(20)
// @Success      200  {object}  pkg.Response{data=object{items=[]model.Notification}}
// @Failure      401  {object}  pkg.Response
// @Router       /notifications/list [post]
func (h *notificationHandler) List(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	var req struct {
		ListParams
		Type string `json:"type"`
		Read string `json:"read"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		if !errors.Is(err, io.EOF) {
			pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
			return
		}
	}
	req.Normalize()
	items, total, err := h.svc.ListFiltered(ctx.Request().Context(), u.ID, req.Page, req.PageSize, req.Type, req.Read)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"items": items, "total": total})
}

// CountUnread 获取未读通知数量
// @Summary      获取未读通知数量
// @Description  获取当前用户的未读通知数量
// @Tags         通知
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=object{count=int64}}
// @Failure      401  {object}  pkg.Response
// @Router       /notifications/unread_count [post]
func (h *notificationHandler) CountUnread(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	count, err := h.svc.CountUnread(ctx.Request().Context(), u.ID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"count": count})
}

// MarkRead 标记通知已读
// @Summary      标记通知已读
// @Description  标记指定通知为已读
// @Tags         通知
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "通知 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /notifications/{id}/read [post]
func (h *notificationHandler) MarkRead(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	id := ctx.Params().Get("id")
	if id == "" {
		pkg.Fail(ctx, pkg.NOT_FOUND, "通知 ID 不能为空")
		return
	}
	if err := h.svc.MarkRead(ctx.Request().Context(), id, u.ID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// MarkAllRead 标记所有通知已读
// @Summary      标记所有通知已读
// @Description  标记当前用户的所有通知为已读
// @Tags         通知
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /notifications/read_all [post]
func (h *notificationHandler) MarkAllRead(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	if err := h.svc.MarkAllRead(ctx.Request().Context(), u.ID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// Delete 删除通知
// @Summary      删除通知
// @Description  删除指定通知
// @Tags         通知
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "通知 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /notifications/{id}/delete [post]
func (h *notificationHandler) Delete(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	id := ctx.Params().Get("id")
	if id == "" {
		pkg.Fail(ctx, pkg.NOT_FOUND, "通知 ID 不能为空")
		return
	}
	if err := h.svc.Delete(ctx.Request().Context(), id, u.ID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// Stream SSE 推送未读通知数量
// @Summary      SSE 推送通知
// @Description  通过 SSE 实时推送未读通知数量，优先使用 KV Pub/Sub，降级为定时轮询
// @Tags         通知
// @Accept       json
// @Produce      text/event-stream
// @Security     BearerAuth
// @Success      200  {object}  string  "SSE event stream: data: {\"unread\":N}\n\n"
// @Failure      401  {object}  pkg.Response
// @Router       /notifications/stream [post]
func (h *notificationHandler) Stream(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}

	ctx.ContentType("text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")

	flusher, ok := ctx.ResponseWriter().Flusher()
	if !ok {
		slog.Error("notification stream: response writer does not support flushing")
		pkg.Fail(ctx, pkg.INTERNAL_ERROR)
		return
	}
	// The handler blocks for the stream's whole lifetime, so defer balances
	// the gauge on both the reqCtx.Done() and write-failure exits.
	metrics.SSEConnections.WithLabelValues(metrics.SSEChannelNotification).Inc()
	defer metrics.SSEConnections.WithLabelValues(metrics.SSEChannelNotification).Dec()

	reqCtx := ctx.Request().Context()
	w := ctx.ResponseWriter()

	writeSSE := func(data []byte) bool {
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			// Distinguish "could not deliver" from "client hung up": without
			// this the only signal was the connection gauge falling, which
			// looks identical to a normal tab close.
			metrics.ObserveSSEWriteFailure(metrics.SSEChannelNotification)
			return false
		}
		flusher.Flush()
		return true
	}

	writeSSEKeepalive := func(w io.Writer, f http.Flusher) bool {
		// SSE comment: ignored by every client, and the frontend parser only
		// looks at lines starting with "data: ", so this cannot surface as an
		// event with an empty payload.
		if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
			metrics.ObserveSSEWriteFailure(metrics.SSEChannelNotification)
			return false
		}
		f.Flush()
		return true
	}

	// Try Pub/Sub first.
	ch, err := h.kv.Subscribe(reqCtx, "notif:"+u.ID)
	if err == nil {
		// Send current unread count immediately on connect.
		if count, cErr := h.svc.CountUnread(reqCtx, u.ID); cErr == nil {
			_ = writeSSE([]byte(fmt.Sprintf(`{"unread":%d}`, count)))
		}
		// Keepalive: an idle notifications stream carries no bytes at all, so any
		// intermediary with a read timeout drops it. nginx's proxy_read_timeout
		// defaults to 60s, and the stream was measured disconnecting at ~61s while
		// /realtime/stream (which heartbeats) survived past 200s. An SSE comment
		// line is the protocol-correct keepalive: clients ignore it, the frontend
		// parser only reacts to "data: " lines, and it cannot be mistaken for an
		// event. Use the same 15s cadence as the realtime stream.
		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				if !writeSSE(msg) {
					return
				}
			case <-heartbeat.C:
				if !writeSSEKeepalive(w, flusher) {
					return
				}
			case <-reqCtx.Done():
				return
			}
		}
	}

	// Fallback: polling interval from settings, default 5s.
	pollInterval := pkg.GetSettingInt(ctx.Request().Context(), h.settingSvc, "sse_polling_interval", "", 5)
	ticker := time.NewTicker(time.Duration(pollInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			count, cErr := h.svc.CountUnread(reqCtx, u.ID)
			if cErr != nil {
				continue
			}
			if !writeSSE([]byte(fmt.Sprintf(`{"unread":%d}`, count))) {
				return
			}
		case <-reqCtx.Done():
			return
		}
	}
}
