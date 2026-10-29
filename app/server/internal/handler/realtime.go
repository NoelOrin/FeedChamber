package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"orinforum/internal/metrics"
	"orinforum/internal/pkg"
	"orinforum/internal/pkg/kvstore"

	"github.com/kataras/iris/v12"
)

type RealtimeHandler interface {
	Stream(ctx iris.Context)
}

type realtimeHandler struct {
	kv kvstore.Store
}

func newRealtimeHandler(kv kvstore.Store) RealtimeHandler {
	return &realtimeHandler{kv: kv}
}

// 通用实时流 (SSE)
// @Summary      通用实时流 (SSE)
// @Description  以 SSE 推送事件；仅允许订阅本人的通知频道。
// @Tags         实时
// @Accept       json
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        body body object{channels=[]string} false "订阅频道"
// @Success      200  {string}  string
// @Failure      401  {object}  pkg.Response
// @Router       /realtime/stream [post]
// Stream POST /api/v1/realtime/stream — generic SSE endpoint.
func (h *realtimeHandler) Stream(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	var body struct {
		Channels []string `json:"channels"`
	}
	_ = ctx.ReadJSON(&body)
	channels := make([]string, 0, len(body.Channels))
	seen := map[string]struct{}{}
	for _, ch := range body.Channels {
		if ch == "" {
			continue
		}
		// P0: restrict channel subscription to own notification channel only.
		// Discourse baseline: users must not subscribe to arbitrary channels or other users' notif streams.
		if ch != "notif:"+u.ID {
			continue
		}
		if _, ok := seen[ch]; ok {
			continue
		}
		seen[ch] = struct{}{}
		channels = append(channels, ch)
	}
	if len(channels) == 0 {
		channels = []string{"notif:" + u.ID}
	}

	ctx.ContentType("text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	flusher, ok := ctx.ResponseWriter().Flusher()
	if !ok {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR)
		return
	}
	metrics.SSEConnections.WithLabelValues(metrics.SSEChannelRealtime).Inc()
	defer metrics.SSEConnections.WithLabelValues(metrics.SSEChannelRealtime).Dec()

	w := ctx.ResponseWriter()
	var eventID int64
	reqCtx := ctx.Request().Context()

	writeEnvelope := func(channel string, payload any) bool {
		id := atomic.AddInt64(&eventID, 1)
		envelope := map[string]any{
			"type":    "message",
			"channel": channel,
			"payload": payload,
			"id":      id,
		}
		data, _ := json.Marshal(envelope)
		if _, err := fmt.Fprintf(w, "id: %d\ndata: %s\n\n", id, data); err != nil {
			// See notification.Stream: a failed write is a delivery problem,
			// not a client disconnect, and must be distinguishable.
			metrics.ObserveSSEWriteFailure(metrics.SSEChannelRealtime)
			return false
		}
		flusher.Flush()
		return true
	}

	subs := make(map[string]<-chan []byte, len(channels))
	for _, ch := range channels {
		sub, err := h.kv.Subscribe(reqCtx, ch)
		if err == nil {
			subs[ch] = sub
		} else {
			slog.Warn("realtime subscribe failed", "channel", ch, "error", err)
		}
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-heartbeat.C:
			if !writeEnvelope("heartbeat", map[string]any{"ts": time.Now().Unix()}) {
				return
			}
		case <-reqCtx.Done():
			return
		default:
			ready := false
			for ch, sub := range subs {
				select {
				case msg, ok := <-sub:
					if !ok {
						delete(subs, ch)
						continue
					}
					ready = true
					var payload any
					if err := json.Unmarshal(msg, &payload); err != nil {
						payload = string(msg)
					}
					if !writeEnvelope(ch, payload) {
						return
					}
				default:
				}
			}
			if !ready {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}
