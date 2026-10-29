package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"feedchamber/internal/metrics"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/repository"

	"github.com/sirupsen/logrus"
)

type WebhookService interface {
	Create(ctx context.Context, url, secret string, events []string, enabled, tlsVerify bool) (*model.Webhook, error)
	Update(ctx context.Context, id, url, secret string, events []string, enabled *bool) (*model.Webhook, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, page, pageSize int) ([]model.Webhook, int, error)
	ListDeliveries(ctx context.Context, webhookID string, page, pageSize int) ([]model.WebhookDelivery, int, error)
	Ping(ctx context.Context, id string) (*model.WebhookDelivery, error)
	RetryDelivery(ctx context.Context, id, deliveryID string) (*model.WebhookDelivery, error)
	Enqueue(ctx context.Context, event string, payload map[string]any)
	StartWorker(ctx context.Context) context.CancelFunc
	Subscribe(bus *eventbus.EventBus)
}

var allowedWebhookEvents = map[string]struct{}{
	eventbus.EventContentReplied: {},
	eventbus.EventContentLiked:   {},
	eventbus.EventUserFollowed:   {},
	eventbus.EventUserMentioned:  {},
	eventbus.EventContentWatched: {},
	eventbus.EventPostCreated:    {},
	eventbus.EventUserCreated:    {},
	eventbus.EventReportCreated:  {},
	eventbus.EventPostClosed:     {},
	eventbus.EventContentCreated: {},
	eventbus.EventUserRegistered: {},
	eventbus.EventModerationDone: {},
}

type webhookService struct {
	repo   repository.WebhookRepository
	client *http.Client
}

func NewWebhookService(repo repository.WebhookRepository) WebhookService {
	return &webhookService{repo: repo, client: &http.Client{Timeout: 10 * time.Second}}
}

func (s *webhookService) Create(ctx context.Context, url, secret string, events []string, enabled, tlsVerify bool) (*model.Webhook, error) {
	if strings.TrimSpace(url) == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "url 不能为空")
	}
	for _, ev := range events {
		if _, ok := allowedWebhookEvents[ev]; !ok {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "unknown webhook event: "+ev)
		}
	}
	raw, _ := json.Marshal(events)
	w := &model.Webhook{
		ID:        pkg.NewID(),
		URL:       url,
		Secret:    secret,
		Events:    string(raw),
		Enabled:   enabled,
		TLSVerify: tlsVerify,
		CreatedAt: time.Now(),
	}
	if err := s.repo.Create(ctx, w); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return w, nil
}

func (s *webhookService) Update(ctx context.Context, id, url, secret string, events []string, enabled *bool) (*model.Webhook, error) {
	w, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "webhook 不存在")
	}
	if events != nil {
		for _, ev := range events {
			if _, ok := allowedWebhookEvents[ev]; !ok {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "unknown webhook event: "+ev)
			}
		}
	}
	if url != "" {
		w.URL = url
	}
	if secret != "" {
		w.Secret = secret
	}
	if events != nil {
		raw, _ := json.Marshal(events)
		w.Events = string(raw)
	}
	if enabled != nil {
		w.Enabled = *enabled
	}
	if err := s.repo.Update(ctx, w); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return w, nil
}

func (s *webhookService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *webhookService) List(ctx context.Context, page, pageSize int) ([]model.Webhook, int, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	items, total, err := s.repo.List(ctx, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return items, int(total), nil
}

func (s *webhookService) ListDeliveries(ctx context.Context, webhookID string, page, pageSize int) ([]model.WebhookDelivery, int, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	items, total, err := s.repo.ListDeliveries(ctx, webhookID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return items, int(total), nil
}

func (s *webhookService) Ping(ctx context.Context, id string) (*model.WebhookDelivery, error) {
	hook, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "webhook 不存在")
	}
	payload, _ := json.Marshal(map[string]any{"event": "ping", "data": map[string]any{"message": "ping"}, "ts": time.Now().Unix()})
	d := &model.WebhookDelivery{
		ID:        pkg.NewID(),
		WebhookID: hook.ID,
		Event:     "ping",
		Payload:   string(payload),
		Status:    "pending",
		Kind:      "ping",
		CreatedAt: time.Now(),
	}
	if err := s.repo.CreateDelivery(ctx, d); err != nil {
		return nil, err
	}
	if err := s.deliver(ctx, hook, d); err != nil {
		d.Status = "failed"
		d.LastError = err.Error()
	} else {
		d.Status = "success"
		d.LastError = ""
	}
	if err := s.repo.UpdateDelivery(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *webhookService) RetryDelivery(ctx context.Context, id, deliveryID string) (*model.WebhookDelivery, error) {
	hook, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "webhook 不存在")
	}
	d, err := s.repo.GetDelivery(ctx, deliveryID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "delivery 不存在")
	}
	if d.WebhookID != hook.ID {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "delivery 不存在")
	}
	d.Status = "pending"
	d.Attempts = 0
	d.LastError = ""
	d.NextRetryAt = nil
	d.Kind = "retry"
	if err := s.repo.UpdateDelivery(ctx, d); err != nil {
		return nil, err
	}
	if err := s.deliver(ctx, hook, d); err != nil {
		d.Status = "failed"
		d.Attempts = 1
		d.LastError = err.Error()
	} else {
		d.Status = "success"
		d.Attempts = 1
		d.LastError = ""
	}
	if err := s.repo.UpdateDelivery(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *webhookService) Enqueue(ctx context.Context, event string, payload map[string]any) {
	if s.repo == nil || event == "" {
		return
	}
	hooks, err := s.repo.ListEnabled(ctx)
	if err != nil {
		logrus.Warnf("webhook list enabled: %v", err)
		return
	}
	body, _ := json.Marshal(map[string]any{"event": event, "data": payload, "ts": time.Now().Unix()})
	for _, h := range hooks {
		if !eventAllowed(h.Events, event) {
			continue
		}
		d := &model.WebhookDelivery{
			ID:        pkg.NewID(),
			WebhookID: h.ID,
			Event:     event,
			Payload:   string(body),
			Status:    "pending",
			Attempts:  0,
			CreatedAt: time.Now(),
		}
		if err := s.repo.CreateDelivery(ctx, d); err != nil {
			logrus.Warnf("webhook enqueue: %v", err)
		}
	}
}

func eventAllowed(eventsJSON, event string) bool {
	if _, ok := allowedWebhookEvents[event]; !ok {
		return false
	}
	if eventsJSON == "" || eventsJSON == "[]" || eventsJSON == "*" {
		return true
	}
	var events []string
	if err := json.Unmarshal([]byte(eventsJSON), &events); err != nil {
		return true
	}
	if len(events) == 0 {
		return true
	}
	for _, e := range events {
		if e == event || e == "*" {
			return true
		}
	}
	return false
}

func (s *webhookService) StartWorker(ctx context.Context) context.CancelFunc {
	cctx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-cctx.Done():
				return
			case <-ticker.C:
				s.processPending(cctx)
			}
		}
	}()
	return cancel
}

func (s *webhookService) processPending(ctx context.Context) {
	// Backlog gauge first, and from COUNT(*) — not from the batch below.
	// The batch is capped at 20, so len(items) pinned the gauge at 20.
	if n, err := s.repo.CountPendingDeliveries(ctx); err == nil {
		metrics.WebhookPendingDeliveries.Set(float64(n))
	}

	items, err := s.repo.ListPendingDeliveries(ctx, 20)
	if err != nil {
		return
	}
	for _, d := range items {
		if d.NextRetryAt != nil && d.NextRetryAt.After(time.Now()) {
			continue
		}
		hook, err := s.repo.Get(ctx, d.WebhookID)
		if err != nil || hook == nil || !hook.Enabled {
			d.Status = "failed"
			d.LastError = "webhook missing/disabled"
			_ = s.repo.UpdateDelivery(ctx, &d)
			continue
		}
		if err := s.deliver(ctx, hook, &d); err != nil {
			d.Attempts++
			d.LastError = err.Error()
			if d.Attempts >= 5 {
				d.Status = "failed"
				metrics.WebhookDeliveriesTotal.WithLabelValues(metrics.WebhookOutcomeFailedTerminal).Inc()
			} else {
				metrics.WebhookDeliveriesTotal.WithLabelValues(metrics.WebhookOutcomeRetryScheduled).Inc()
				next := time.Now().Add(webhookBackoff(d.Attempts))
				d.NextRetryAt = &next
			}
			_ = s.repo.UpdateDelivery(ctx, &d)
			continue
		}
		metrics.WebhookDeliveriesTotal.WithLabelValues(metrics.WebhookOutcomeDelivered).Inc()
		d.Attempts++
		d.Status = "success"
		d.LastError = ""
		d.NextRetryAt = nil
		_ = s.repo.UpdateDelivery(ctx, &d)
	}
}

func webhookBackoff(attempts int) time.Duration {
	switch {
	case attempts <= 1:
		return time.Minute
	case attempts == 2:
		return 5 * time.Minute
	case attempts == 3:
		return 30 * time.Minute
	case attempts == 4:
		return 2 * time.Hour
	default:
		return 12 * time.Hour
	}
}

func (s *webhookService) deliver(ctx context.Context, hook *model.Webhook, d *model.WebhookDelivery) (err error) {
	start := time.Now()
	defer func() {
		metrics.WebhookDeliveryDuration.Observe(time.Since(start).Seconds())
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook.URL, bytes.NewBufferString(d.Payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if hook.Secret != "" {
		mac := hmac.New(sha256.New, []byte(hook.Secret))
		_, _ = mac.Write([]byte(d.Payload))
		req.Header.Set("X-Orin-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	client := s.client
	if !hook.TLSVerify {
		client = &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	}
	resp, err := client.Do(req)
	if err != nil {
		d.LatencyMs = time.Since(start).Milliseconds()
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	d.ResponseStatus = resp.StatusCode
	d.ResponseBody = strings.TrimSpace(string(body))
	d.LatencyMs = time.Since(start).Milliseconds()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "status "+resp.Status)
	}
	return nil
}

func (s *webhookService) Subscribe(bus *eventbus.EventBus) {
	if bus == nil {
		return
	}
	for ev := range allowedWebhookEvents {
		e := ev
		bus.Subscribe(e, func(evt eventbus.Event) {
			s.Enqueue(evt.Ctx, e, evt.Payload)
		})
	}
}
