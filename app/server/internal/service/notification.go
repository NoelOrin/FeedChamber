package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"feedchamber/internal/metrics"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/pkg/kvstore"
	"feedchamber/internal/repository"

	"github.com/sirupsen/logrus"
)

type notificationService struct {
	repo       repository.NotificationRepository
	kv         kvstore.Store
	emailSvc   EmailService
	userRepo   repository.UserRepository
	settingSvc SettingService
	blockSvc   UserBlockService
	emailDisp  *emailDispatcher
}

func NewNotificationService(repo repository.NotificationRepository, kv kvstore.Store, emailSvc EmailService, userRepo repository.UserRepository, settingSvc SettingService, blockSvc UserBlockService) NotificationService {
	svc := &notificationService{repo: repo, kv: kv, emailSvc: emailSvc, userRepo: userRepo, settingSvc: settingSvc, blockSvc: blockSvc}
	svc.emailDisp = newEmailDispatcher(svc)
	return svc
}

func (s *notificationService) unreadKey(userID string) string {
	return "notif:unread:" + userID
}

func (s *notificationService) publishUnread(ctx context.Context, userID string) {
	if userID == "" || s.kv == nil {
		return
	}
	count, err := s.repo.CountUnread(ctx, userID)
	if err != nil {
		logrus.Warnf("notification: db CountUnread after mutation failed: %v", err)
		return
	}
	if setErr := s.kv.Set(ctx, s.unreadKey(userID), fmt.Sprintf("%d", count), 5*time.Minute); setErr != nil {
		logrus.Warnf("notification: kv Set after mutation failed: %v", setErr)
	}
	payload, _ := json.Marshal(map[string]interface{}{"unread": count})
	if pubErr := s.kv.Publish(ctx, "notif:"+userID, payload); pubErr != nil {
		logrus.Warnf("notification: kv publish after mutation failed: %v", pubErr)
	}
}

func (s *notificationService) Create(ctx context.Context, userID, notifType, content, referenceID, senderID string) error {
	// Skip notifications from users the recipient has blocked.
	if s.blockSvc != nil && senderID != "" && userID != "" {
		if blocked, err := s.blockSvc.IsBlocked(ctx, userID, senderID); err == nil && blocked {
			return nil
		}
	}
	// Reconcile the cache with the DB BEFORE inserting the new row. The cached
	// unread count carries a 5-minute TTL; CountUnread honours it (Get ->
	// isExpired -> DB fallback + re-Set), while IncrBy reads the raw JetStream
	// value and does NOT check the TTL marker. Without this call, a notification
	// arriving after the TTL expired but before any read purged the stale value
	// would be pushed to SSE as stale+1. Reconciling before the insert keeps the
	// subsequent CAS-based IncrBy as exactly "+1 for this new row".
	if _, err := s.CountUnread(ctx, userID); err != nil {
		logrus.Warnf("notification: cache reconcile before create failed: %v", err)
	}
	n := model.NewNotification(userID, notifType, content, referenceID, senderID)
	if err := s.repo.Create(ctx, n); err != nil {
		return err
	}
	// Increment cached unread count; on failure invalidate cache so next
	// CountUnread falls back to DB and repopulates with the correct value.
	newCount, err := s.kv.IncrBy(ctx, s.unreadKey(userID), 1)
	if err != nil {
		logrus.Warnf("notification: kv IncrBy failed, falling back to DB: %v", err)
		// IncrBy failed — don't Delete (wastes another KV round-trip; TTL=5m expires naturally).
		// Fall back to DB for SSE push count so user still gets real-time notification.
		dbCount, dbErr := s.repo.CountUnread(ctx, userID)
		if dbErr != nil {
			logrus.Warnf("notification: db CountUnread fallback failed: %v", dbErr)
		} else {
			newCount = dbCount
			// Refresh KV cache with correct DB count so next CountUnread hits cache
			if setErr := s.kv.Set(ctx, s.unreadKey(userID), fmt.Sprintf("%d", dbCount), 5*time.Minute); setErr != nil {
				logrus.Warnf("notification: kv Set fallback failed: %v", setErr)
			}
		}
	}
	// Publish to Pub/Sub so SSE Stream clients receive the update immediately.
	if newCount > 0 {
		payload, _ := json.Marshal(map[string]interface{}{"unread": newCount})
		if pubErr := s.kv.Publish(ctx, "notif:"+userID, payload); pubErr != nil {
			logrus.Warnf("notification: kv publish failed: %v", pubErr)
		}
	}

	// 通过有界 worker 池异步发送邮件，避免裸 goroutine 泄漏
	s.emailDisp.dispatch(emailTask{
		ctx:       ctx,
		userID:    userID,
		notifType: notifType,
		content:   content,
	})

	return nil
}

func (s *notificationService) ListFiltered(ctx context.Context, userID string, page, pageSize int, typ, read string) ([]model.Notification, int64, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	offset := (page - 1) * pageSize
	items, err := s.repo.ListByUserFiltered(ctx, userID, typ, read, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.CountByUserFiltered(ctx, userID, typ, read)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *notificationService) CountUnread(ctx context.Context, userID string) (int64, error) {
	key := s.unreadKey(userID)
	if val, err := s.kv.Get(ctx, key); err == nil {
		if count, err := strconv.ParseInt(val, 10, 64); err == nil {
			return count, nil
		}
	}
	// Cache miss — query DB and populate cache
	count, err := s.repo.CountUnread(ctx, userID)
	if err != nil {
		return 0, err
	}
	if setErr := s.kv.Set(ctx, key, fmt.Sprintf("%d", count), 5*time.Minute); setErr != nil {
		logrus.Warnf("notification: kv Set cache populate failed: %v", setErr)
	}
	return count, nil
}

func (s *notificationService) MarkRead(ctx context.Context, id, userID string) error {
	n, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "通知不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if n.UserID != userID {
		return pkg.NewAppError(pkg.NOT_FOUND, "通知不存在")
	}
	if n.IsRead {
		return nil
	}
	if err := s.repo.MarkRead(ctx, id); err != nil {
		return err
	}
	if _, err := s.kv.IncrBy(ctx, s.unreadKey(userID), -1); err != nil {
		logrus.Warnf("notification: kv decr on mark-read failed: %v", err)
	}
	s.publishUnread(ctx, userID)
	return nil
}

func (s *notificationService) MarkAllRead(ctx context.Context, userID string) error {
	if err := s.repo.MarkAllRead(ctx, userID); err != nil {
		return err
	}
	if setErr := s.kv.Set(ctx, s.unreadKey(userID), "0", 0); setErr != nil {
		logrus.Warnf("notification: kv Set mark-all-read failed: %v", setErr)
	}
	s.publishUnread(ctx, userID)
	return nil
}

func (s *notificationService) Delete(ctx context.Context, id, userID string) error {
	n, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "通知不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if n.UserID != userID {
		return pkg.NewAppError(pkg.NOT_FOUND, "通知不存在")
	}
	wasUnread := !n.IsRead
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if wasUnread {
		_, _ = s.kv.IncrBy(ctx, s.unreadKey(userID), -1)
	}
	s.publishUnread(ctx, userID)
	return nil
}

// ── NotifDispatcher ──────────────────────────────────────
// 将通知创建的 IO 操作从 EventBus worker 中彻底分离。
// EventBus worker 只做内存级的事件参数解析，入队后立即返回。
// 独立的 worker 池消费队列，执行 DB 写入 + KV（NATS）操作。

// notifyTask 封装一次通知创建请求。
type notifyTask struct {
	ctx         context.Context
	userID      string
	notifType   string
	content     string
	referenceID string
	senderID    string
}

// NotifDispatcher 管理通知创建任务的异步分发。
type NotifDispatcher struct {
	ch       chan notifyTask
	notifSvc NotificationService
	wg       sync.WaitGroup
}

const (
	notifQueueSize     = 256
	defaultNotifWorker = 8
)

func newNotifDispatcher(svc NotificationService) *NotifDispatcher {
	return &NotifDispatcher{
		ch:       make(chan notifyTask, notifQueueSize),
		notifSvc: svc,
	}
}

// enqueue hands a task to the worker pool without ever blocking the caller.
// The caller is an EventBus worker shared by every subscriber; if notification
// creation is slow (DB/KV backpressure), a blocking send would stall unrelated
// event types too. Overflow is shed and made visible via a counter + log.
// Shutdown closes d.ch, so enqueue must only be called before Shutdown.
func (d *NotifDispatcher) enqueue(task notifyTask) {
	select {
	case d.ch <- task:
	default:
		metrics.NotifDispatcherDroppedTotal.WithLabelValues(task.notifType).Inc()
		logrus.WithField("type", task.notifType).
			WithField("user_id", task.userID).
			Warn("notif: dispatcher queue full, dropping notification task")
	}
}

func (d *NotifDispatcher) start(workers int) {
	for i := 0; i < workers; i++ {
		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			for task := range d.ch {
				if err := d.notifSvc.Create(task.ctx, task.userID, task.notifType, task.content, task.referenceID, task.senderID); err != nil {
					logrus.Errorf("notif: async create failed: %v", err)
				}
			}
		}()
	}
}

// Shutdown 等待所有已入队的通知创建任务完成。
func (d *NotifDispatcher) Shutdown() {
	close(d.ch)
	d.wg.Wait()
}

// ── emailDispatcher ──────────────────────────────────────
// 有界 worker 池，替代 Create 中的裸 goroutine 发送邮件。

type emailTask struct {
	ctx       context.Context
	userID    string
	notifType string
	content   string
}

type emailDispatcher struct {
	ch   chan emailTask
	svc  *notificationService
	wg   sync.WaitGroup
	quit chan struct{}
}

const (
	emailQueueSize   = 128
	emailWorkers     = 4
	emailMinInterval = 200 * time.Millisecond // min delay between sends per worker
)

func newEmailDispatcher(svc *notificationService) *emailDispatcher {
	d := &emailDispatcher{
		ch:   make(chan emailTask, emailQueueSize),
		svc:  svc,
		quit: make(chan struct{}),
	}
	for i := 0; i < emailWorkers; i++ {
		d.wg.Add(1)
		go d.worker()
	}
	return d
}

func (d *emailDispatcher) worker() {
	defer d.wg.Done()
	for {
		select {
		case task, ok := <-d.ch:
			if !ok {
				return
			}
			d.send(task)
		case <-d.quit:
			return
		}
	}
}

func (d *emailDispatcher) send(task emailTask) {
	// 使用 30 秒超时，避免 SMTP 挂起导致 worker 永久阻塞
	bg, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 简易速率限制：每次发送后等待最小间隔，防止批量邮件突发
	time.Sleep(emailMinInterval)
	user, err := d.svc.userRepo.FindByID(bg, task.userID)
	if err != nil || user == nil || user.Email == "" {
		return
	}
	subject := "新通知 — " + task.notifType
	text := fmt.Sprintf("您有一条新通知：%s", task.content)
	html := fmt.Sprintf(`<!DOCTYPE html><html><body style="font-family:sans-serif;max-width:480px;margin:0 auto;padding:24px">
	  <h2 style="color:#333">新通知</h2>
	  <p>%s</p>
	  <p style="color:#666;font-size:14px">此邮件由系统自动发送，请勿回复。</p>
	</body></html>`, task.content)
	_ = d.svc.emailSvc.SendNotification(bg, user.Email, subject, html, text)
}

func (d *emailDispatcher) dispatch(task emailTask) {
	select {
	case d.ch <- task:
		// 入队成功
	default:
		// 队列满，丢弃邮件任务（不阻塞业务请求）
		logrus.Warnf("notification: email queue full, dropping email for user %s", task.userID)
	}
}

// Shutdown 关闭 email dispatcher 并等待所有 worker 退出。
func (d *emailDispatcher) Shutdown() {
	close(d.quit)
	d.wg.Wait()
}

// ShutdownNotificationEmailDispatcher 关闭通知服务的邮件发送 worker 池。
// 在应用优雅关闭时调用。
func ShutdownNotificationEmailDispatcher(svc NotificationService) {
	if ns, ok := svc.(*notificationService); ok {
		ns.emailDisp.Shutdown()
	}
}

// SubscribeNotifications 注册事件总线订阅，将业务事件转化为通知。
// 返回 NotifDispatcher，应用退出时需调用其 Shutdown 确保任务排空。
func SubscribeNotifications(bus *eventbus.EventBus, notifSvc NotificationService) *NotifDispatcher {
	d := newNotifDispatcher(notifSvc)

	bus.Subscribe(eventbus.EventContentReplied, func(e eventbus.Event) {
		userID, _ := e.Payload["user_id"].(string)
		senderID, _ := e.Payload["sender_id"].(string)
		senderName, _ := e.Payload["sender_name"].(string)
		postID, _ := e.Payload["post_id"].(string)
		postTitle, _ := e.Payload["post_title"].(string)
		isReply, _ := e.Payload["is_reply"].(bool)

		var msg string
		if isReply {
			msg = senderName + " 回复了你的评论"
		} else {
			msg = senderName + " 评论了你的帖子《" + postTitle + "》"
		}
		d.enqueue(notifyTask{
			ctx:         e.Ctx,
			userID:      userID,
			notifType:   "comment",
			content:     msg,
			referenceID: postID,
			senderID:    senderID,
		})
	})

	bus.Subscribe(eventbus.EventContentLiked, func(e eventbus.Event) {
		userID, _ := e.Payload["user_id"].(string)
		senderID, _ := e.Payload["sender_id"].(string)
		senderName, _ := e.Payload["sender_name"].(string)
		// Prefer post_id for deep-link; fall back to content_id only if legacy payload.
		postID, _ := e.Payload["post_id"].(string)
		if postID == "" {
			postID, _ = e.Payload["content_id"].(string)
		}

		d.enqueue(notifyTask{
			ctx:         e.Ctx,
			userID:      userID,
			notifType:   "like",
			content:     senderName + " 赞了你的评论",
			referenceID: postID,
			senderID:    senderID,
		})
	})

	bus.Subscribe(eventbus.EventUserFollowed, func(e eventbus.Event) {
		userID, _ := e.Payload["user_id"].(string)
		senderID, _ := e.Payload["sender_id"].(string)
		senderName, _ := e.Payload["sender_name"].(string)
		followerID, _ := e.Payload["follower_id"].(string)

		d.enqueue(notifyTask{
			ctx:         e.Ctx,
			userID:      userID,
			notifType:   "follow",
			content:     senderName + " 关注了你",
			referenceID: followerID,
			senderID:    senderID,
		})
	})

	bus.Subscribe(eventbus.EventUserMentioned, func(e eventbus.Event) {
		userID, _ := e.Payload["user_id"].(string)
		senderID, _ := e.Payload["sender_id"].(string)
		senderName, _ := e.Payload["sender_name"].(string)
		postID, _ := e.Payload["post_id"].(string)
		snippet, _ := e.Payload["snippet"].(string)

		msg := senderName + " 提到了你"
		if snippet != "" {
			msg = senderName + " 提到了你: " + snippet
		}
		d.enqueue(notifyTask{
			ctx:         e.Ctx,
			userID:      userID,
			notifType:   "mention",
			content:     msg,
			referenceID: postID,
			senderID:    senderID,
		})
	})

	bus.Subscribe(eventbus.EventContentWatched, func(e eventbus.Event) {
		userID, _ := e.Payload["user_id"].(string)
		senderID, _ := e.Payload["sender_id"].(string)
		senderName, _ := e.Payload["sender_name"].(string)
		postID, _ := e.Payload["post_id"].(string)
		postTitle, _ := e.Payload["post_title"].(string)

		msg := senderName + " 更新了你关注的内容"
		if postTitle != "" {
			msg = senderName + " 更新了你关注的话题《" + postTitle + "》"
		}
		d.enqueue(notifyTask{
			ctx:         e.Ctx,
			userID:      userID,
			notifType:   "watch",
			content:     msg,
			referenceID: postID,
			senderID:    senderID,
		})
	})

	// ── 社区自治动议（motion）───────────────────────────────────────────────
	// 与 Report 事件完全分离：动议只通知"发起者"（结果反馈）与"目标作者"
	// （被表决），复用既有 SSE 通知通道，不新建端点（design D6）。

	// 审核结论通知作者：manual 模式下所有帖子先进待审，async/external 命中也会转待审；
	// 此前这个事件只喂了 webhook 与插件，作者被通过/拒绝后收不到任何站内提示。
	bus.Subscribe(eventbus.EventModerationDone, func(e eventbus.Event) {
		authorID, _ := e.Payload["author_id"].(string)
		if authorID == "" {
			return // 目标已被删或解析失败：没有收件人
		}
		decision, _ := e.Payload["decision"].(string)
		title, _ := e.Payload["title"].(string)
		reason, _ := e.Payload["reason"].(string)
		targetType, _ := e.Payload["target_type"].(string)
		targetID, _ := e.Payload["target_id"].(string)

		kind := "帖子"
		if targetType == "content" {
			kind = "评论"
		}
		var msg string
		switch decision {
		case "approved":
			msg = "你的" + kind + "《" + title + "》已通过审核"
		case "rejected":
			msg = "你的" + kind + "《" + title + "》未通过审核"
			if reason != "" {
				msg += "：" + reason
			}
		default:
			return
		}
		d.enqueue(notifyTask{
			ctx:         e.Ctx,
			userID:      authorID,
			notifType:   "moderation",
			content:     msg,
			referenceID: targetID,
		})
	})

	bus.Subscribe(eventbus.EventMotionCreated, func(e eventbus.Event) {
		initiatorID, _ := e.Payload["initiator_id"].(string)
		targetAuthorID, _ := e.Payload["target_author_id"].(string)
		motionType, _ := e.Payload["type"].(string)
		motionID, _ := e.Payload["motion_id"].(string)
		label := motionTypeLabel(motionType)

		// 发起者：结果反馈。
		if initiatorID != "" {
			d.enqueue(notifyTask{
				ctx:         e.Ctx,
				userID:      initiatorID,
				notifType:   "motion",
				content:     "你的" + label + "动议已公示，等待社区表决",
				referenceID: motionID,
			})
		}
		// 目标作者：被表决。此前这里读了 target_id 就丢弃，被折叠/放逐/移交的人
		// 在公示期与表决结果出来后都毫不知情。自指时（发起者即目标）跳过，避免双发。
		if targetAuthorID != "" && targetAuthorID != initiatorID {
			d.enqueue(notifyTask{
				ctx:         e.Ctx,
				userID:      targetAuthorID,
				notifType:   "motion",
				content:     "有人对你发起了" + label + "动议，已进入公示期",
				referenceID: motionID,
			})
		}
	})

	bus.Subscribe(eventbus.EventMotionResolved, func(e eventbus.Event) {
		initiatorID, _ := e.Payload["initiator_id"].(string)
		targetAuthorID, _ := e.Payload["target_author_id"].(string)
		motionID, _ := e.Payload["motion_id"].(string)
		status, _ := e.Payload["status"].(string)
		motionType, _ := e.Payload["type"].(string)
		label := motionTypeLabel(motionType)
		result := "未通过"
		if status == "passed" {
			result = "已通过"
		}

		if initiatorID != "" {
			d.enqueue(notifyTask{
				ctx:         e.Ctx,
				userID:      initiatorID,
				notifType:   "motion",
				content:     "你发起的" + label + "动议" + result,
				referenceID: motionID,
			})
		}
		// 目标作者必须知道表决结果：通过的折叠/放逐会直接改变他的可见性。
		if targetAuthorID != "" && targetAuthorID != initiatorID {
			d.enqueue(notifyTask{
				ctx:         e.Ctx,
				userID:      targetAuthorID,
				notifType:   "motion",
				content:     "针对你的" + label + "动议" + result,
				referenceID: motionID,
			})
		}
	})

	d.start(defaultNotifWorker)
	return d
}

// motionTypeLabel 把动议类型码翻成通知里的中文动作名。
func motionTypeLabel(t string) string {
	switch t {
	case model.MotionTypeFold:
		return "折叠"
	case model.MotionTypeWatch:
		return "观察"
	case model.MotionTypeEscalate:
		return "移交版主"
	case model.MotionTypeExile:
		return "放逐暗区"
	case model.MotionTypeReverse:
		return "翻案"
	default:
		return "自治"
	}
}
