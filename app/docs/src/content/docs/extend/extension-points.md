---
title: 扩展点
description: 不改核心流程就能挂上去的六类扩展
sidebar:
  order: 2
---

这六类扩展点都是既有实现，加东西不需要动核心链路。

## 1. 事件总线

```go
// 发布（service 层）
s.bus.Publish(eventbus.Event{
	Type: eventbus.EventContentReplied,
	Ctx:  ctx,
	Payload: map[string]any{"post_id": postID, "content_id": contentID},
})

// 订阅（container.Start 里接线）
bus.Subscribe(eventbus.EventUserMentioned, handler)
```

新增事件类型要同时改两处：常量块与 `AllEventTypes()`。
漏了后者会丢零值指标序列，告警里的 `absent()` 会报「事件静默消失」。

## 2. 门控规则引擎

不用写代码：后台 `/admin/gates` 配规则即可。

- 谓词集固定（等级 / XP / 版面 / 标签 / 作者等级 / 是否作者 / 游客 / 邮箱验证 / 封禁 / 暗区 / 外链 / 窗口指标）
- 动作只有四个：`allow` / `deny` / `require_review` / `rate_limit`
- 按优先级降序首个命中，无命中走域默认

配置即时生效（内存快照 + Pub/Sub 失效），并支持「以某个用户试算」。

## 3. 插件（声明式）

`/admin/plugins` 只做元数据登记与启停，**没有执行扩展点**。
这是刻意的：允许执行任意插件代码等于允许任意 RCE。

要加真正的执行能力，正确做法是加新的领域 service + 事件订阅，而不是给插件加 `eval`。

## 4. 主题

用户级主题偏好（`/api/v1/theme/*`）+ 站点级主题 token 覆盖（后台「站点自定义 → 主题」）。

- 用户偏好：light / dark / system，存 localStorage + 后端
- 站点覆盖：CSS 变量名 → 值，支持 `all` / `light` / `dark`

主题 token 只负责改色改圆角；要写任意 CSS 用「自定义代码」里的 CSS 片段。

## 5. 存储 Provider

```go
// internal/storage/storage.go
type Provider interface {
	Upload(ctx context.Context, key string, r io.Reader) (string, error)
	Delete(ctx context.Context, key string) error
	URL(key string) string
}
```

实现 `local` 与 `s3` 两种；加第三种（如 WebDAV）实现该接口并在
`pkg.BuildStorageConfig` 里接一个分支即可。存储是启动期单例，改配置要重启。

## 6. 邮件 Provider

```go
// internal/mailer/
type Mailer interface {
	Send(ctx context.Context, to, subject, html string) error
}
```

已有 SMTP / SendGrid / Resend / Log 四种。加新通道实现接口 + 在工厂里加分支。

## 不推荐的做法

| 想法 | 为什么不 |
|------|----------|
| 在 handler 里直接写业务规则 | 绕过 service 层的所有权与可见性判定 |
| 给插件加代码执行 | 等于开放 RCE |
| 在 repository 里判权限 | repository 拿不到完整的凭证上下文 |
| 用 `strings.Contains` 判唯一冲突 | 三家后端文案不同，应判断 `ErrDuplicate` |
