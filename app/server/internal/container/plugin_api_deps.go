package container

import (
	"context"

	"feedchamber/internal/pkg/kvstore"
	"feedchamber/internal/plugin"
	"feedchamber/internal/repository"
	"feedchamber/internal/service"
)

// 本文件把 container 里的真实 service 适配成 internal/plugin.APIDeps 的七个
// 接缝。刻意放在 container 包：internal/plugin 有「不 import repository /
// 不持有 gorm」的硬约束，接缝接口留在那边，真实接线留在 composition root。
//
// 每个适配器只暴露插件规格表允许的窄动作，不把 service 整个交出去——
// 例如 PostWriter 只有 SetStatus / AddTag，插件拿不到 PostService 的
// Delete / SetFeatured。
//
// 重要：StartPlugin 用 exec.CommandContext(ctx) 管子进程，hostRuntime 因此
// 固定用 context.Background()，绝不能把 HTTP 请求 ctx 透进来（请求结束即杀
// 掉插件进程）。

// hostRuntime 把 *plugin.Host 适配成 service.PluginRuntime。
type hostRuntime struct {
	host *plugin.Host
}

func (r hostRuntime) StartPlugin(id string, m *plugin.Manifest, caps plugin.CapabilitySet) error {
	return r.host.StartPlugin(context.Background(), id, m, caps)
}

func (r hostRuntime) StopPlugin(id string) error { return r.host.StopPlugin(id) }
func (r hostRuntime) Running(id string) bool     { return r.host.Running(id) }

// ── 接缝适配器 ──────────────────────────────────────────────────────

// postWriterWithTags 把 PostService（改状态）与 TagService（加标签）合成一个
// PostWriter 接缝。插件语义是「加一个标签」，而 SetPostTags 是整组替换，
// 所以先读回已有标签再追加——否则插件每加一个标签就会把其它标签抹掉。
type postWriterWithTags struct {
	posts service.PostService
	tags  service.TagService
}

func (a postWriterWithTags) SetStatus(ctx context.Context, postID, status string) error {
	return a.posts.SetStatus(ctx, postID, status)
}

func (a postWriterWithTags) AddTag(ctx context.Context, postID, tag string) error {
	rows, err := a.tags.GetTagsByPostID(ctx, postID)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(rows)+1)
	for _, r := range rows {
		if r.Name == tag {
			return nil
		}
		names = append(names, r.Name)
	}
	names = append(names, tag)
	return a.tags.SetPostTags(ctx, postID, names)
}

type commentWriter struct{ svc service.ContentService }

func (a commentWriter) SetStatus(ctx context.Context, commentID, status string) error {
	return a.svc.SetStatusForPlugin(ctx, commentID, status)
}

type userReader struct{ repo repository.UserRepository }

func (a userReader) GetLevel(ctx context.Context, userID string) (int, error) {
	u, err := a.repo.FindByID(ctx, userID)
	if err != nil {
		return 0, err
	}
	return u.TrustLevel, nil
}

func (a userReader) GetEmail(ctx context.Context, userID string) (string, error) {
	u, err := a.repo.FindByID(ctx, userID)
	if err != nil {
		return "", err
	}
	return u.Email, nil
}

type notifier struct{ svc service.NotificationService }

func (a notifier) Send(ctx context.Context, userID, title, body string) error {
	// 插件发的通知统一用 "plugin" 类型、sender 为空（系统来源）。
	return a.svc.Create(ctx, userID, "plugin", body, "", "")
}

type pluginKV struct{ store kvstore.Store }

func (a pluginKV) Get(ctx context.Context, key string) (string, error) {
	v, err := a.store.Get(ctx, key)
	if err == kvstore.ErrKeyNotFound {
		return "", nil
	}
	return v, err
}

func (a pluginKV) Set(ctx context.Context, key, value string) error {
	return a.store.Set(ctx, key, value, 0)
}

type slotWriter struct {
	svc      service.SlotService
	pluginID string
}

func (a slotWriter) Set(ctx context.Context, slot, kind string, props map[string]any, sortOrder int) error {
	return a.svc.SetForPlugin(ctx, a.pluginID, slot, kind, props, sortOrder)
}

func (a slotWriter) Delete(ctx context.Context, slot string) error {
	return a.svc.DeleteForPlugin(ctx, a.pluginID, slot)
}

type auditRecorder struct{ svc service.AuditLogService }

func (a auditRecorder) RecordPluginAction(ctx context.Context, pluginID, action, target, detail string) error {
	return a.svc.Record(ctx, "plugin:"+pluginID, action, target, detail)
}

// newPluginAPIDeps 为每个插件构造宿主 API 依赖。
//
// pluginID 用于 KV 前缀（由 api.go 拼好后传进来）、插槽 owner 归属与审计来源，
// 因此每次启动（含崩溃重启）都要重新构造，不能复用上一代的 slotWriter。
func newPluginAPIDeps(c *Container, pluginID string, _ plugin.CapabilitySet) plugin.APIDeps {
	return plugin.APIDeps{
		Posts:         postWriterWithTags{posts: c.PostSvc, tags: c.TagSvc},
		Comments:      commentWriter{svc: c.ContentSvc},
		Users:         userReader{repo: c.UserRepo},
		Notifications: notifier{svc: c.NotificationSvc},
		KV:            pluginKV{store: c.KV},
		Slots:         slotWriter{svc: c.SlotSvc, pluginID: pluginID},
		Audit:         auditRecorder{svc: c.AuditLogSvc},
	}
}

// 让编译器在依赖漂移时立刻报错：这些断言钉住适配器确实实现了接缝接口。
var (
	_ plugin.PostWriter    = postWriterWithTags{}
	_ plugin.CommentWriter = commentWriter{}
	_ plugin.UserReader    = userReader{}
	_ plugin.Notifier      = notifier{}
	_ plugin.KVStore       = pluginKV{}
	_ plugin.SlotWriter    = slotWriter{}
	_ plugin.AuditRecorder = auditRecorder{}
)
