# Mailer 邮件发送抽象

四种发送实现，由 **settings 表**以 `email_*` 前缀配置；`pkg.BuildMailerConfig` 按
「settings 表 → 环境变量 → 硬编码默认」三级回退装配，**每次发送都重读**，改后台配置免重启。

> ⚠️ 2026-11-06 校正：本文件此前写的 key（`mail_provider` / `smtp_host` / `smtp_port` /
> `smtp_user` / `smtp_pass` / `sendgrid_api_key` / `resend_api_key` / `mail_from`）**一个都不
> 存在**，接口签名也是错的。照旧文档配置的结果是静默落回 `log` provider——日志看着正常、
> 邮件一封不发。以本版为准。

## 接口

```go
type Sender interface {
    Send(ctx context.Context, msg Message) error
}

type Message struct {
    To       []string
    Subject  string
    HTMLBody string
    TextBody string // 纯文本兜底；HTMLBody 为空时用它
}
```

`Config` 的字段由 `pkg.BuildMailerConfig` 填充，零值等价于 log-only。工厂 `New(cfg)`
在配置不自洽时**直接失败**（SMTP 缺 host/from、resend/sendgrid 缺 api key、未知 provider），
不会静默降级——这是刻意的：静默降级会让人以为在发信。

## 实现与选择条件

| 实现 | 驱动 | 选择条件 |
|------|------|---------|
| SMTP | `net/smtp` | `email_provider` = `smtp` |
| SendGrid | 手写 `net/http`（**不是** sendgrid-go） | `email_provider` = `sendgrid` |
| Resend | 手写 `net/http`（**不是** resend-go） | `email_provider` = `resend` |
| Log | logrus | `email_provider` 为空或 `log`（降级，只写日志） |

SendGrid / Resend 各自带 15s 超时的 `http.Client`（TLS 握手 5s、等响应头 10s），
避免一次 API 卡死拖住通知 worker。

## 配置项（真实 key）

| setting key | env 回退 | 默认 | 说明 |
|---|---|---|---|
| `email_provider` | `EMAIL_PROVIDER` | `log` | `log` / `smtp` / `sendgrid` / `resend` |
| `email_from` | `EMAIL_FROM` | 空 | 发件地址；SMTP 模式下**必填** |
| `email_from_name` | `EMAIL_FROM_NAME` | `Feed Chamber` | 显示名，非 ASCII 会被 MIME 编码 |
| `email_smtp_host` | `EMAIL_SMTP_HOST` | 空 | SMTP 主机；SMTP 模式下**必填** |
| `email_smtp_port` | `EMAIL_SMTP_PORT` | `587` | 端口；0 时按 TLS 模式补 465/587 |
| `email_username` | `EMAIL_USERNAME` | 空 | SMTP 用户名 |
| `email_password` | `EMAIL_PASSWORD` | 空 | SMTP 密码（`IsSecretSettingKey` 覆盖，审计脱敏） |
| `email_use_tls` | `EMAIL_USE_TLS` | `false` | `true` = 隐式 TLS(465)，`false` = STARTTLS(587) |
| `email_api_key` | `EMAIL_API_KEY` | 空 | SendGrid / Resend 的 key（同上，审计脱敏） |
| `email_enabled` | `EMAIL_ENABLED` | `false` | **只**控制用户侧验证码流程，不影响后台测试发信 |

## SMTP 行为（`smtp.go`）

- **两种拨号**：`email_use_tls=true` 走 `tls.Dial` + `smtp.NewClient`（465）；否则 `smtp.Dial`
  后 `StartTLS`（587）。`EffectivePort` 是端口的唯一计算入口——后台诊断报告也读它，
  避免「显示给管理员的端口」和「真正拨的端口」各算一套。
- **证书校验不可关**：`newTLSConfig` 生产恒用系统信任库，自签证书会被拒（`TestSend_RejectsUntrustedServerCert`
  钉住）。这个变量是包级 `var`，**只为测试注入 CA**，不要在任何生产路径覆盖它。
- **明文永不发送凭据**：中继不通告 STARTTLS 时 `StartTLS` 直接报错，不会退回明文 `AUTH`
  （`TestSend_StopsOnPlaintextOnlyRelay`）。
- **认证只用 `AUTH PLAIN`**：少数只通告 `AUTH LOGIN` 的服务商会报
  `server doesn't support AUTH`。两个分支都无条件 `c.Auth()`，匿名中继用不了——
  需要时再按需扩展，不要删掉这层以「兼容」匿名中继。
- **信封地址是裸地址**：显示名（含 MIME 编码）只出现在 `From:` 头，`MAIL FROM:` 始终是
  `cfg.From`（`TestSend_EncodesDisplayNameKeepsBareEnvelope`）。
- **入站消毒**：`sanitizeAddr` 剥掉收件人里的 CRLF（防 SMTP 头注入 / 多次 RCPT），
  `sanitizeHTML` 用 bluemonday UGC policy 过滤正文 HTML。
- **MIME**：同时有 HTML 与纯文本时发 `multipart/alternative`，纯文本 part 在前；
  主题与显示名走 `mime.QEncoding`（非 ASCII 必须编码，否则邮件头被判畸形）。

## 后台「发送测试邮件」

`emailService.SendTest(ctx, to)` → `POST /admin/settings/email/test`（`setting:manage`）：

- 用**当前已保存**的配置真发一封信，返回 `MailTestReport`（provider / host / port /
  tls_mode / from / email_enabled / elapsed_ms / error / notice）。
- **配置与投递失败都写在 report 里（`ok=false`，HTTP 仍 200）**，只有参数非法才回
  `*pkg.AppError`。原因：这条路径的价值就是让管理员看到失败原因，打成 500 只会看到
  「Internal Server Error」。
- `provider=log` 时 `ok=true` 但带 `notice` 说明「只写日志、收件人不会收到」；
  `email_enabled=false` 时同样提示「用户侧验证码邮件仍被拦截」。
- 报告结构体**不含任何凭据字段**，密码/API Key 不可能被顺手序列化出去
  （`TestSendTest_ReportNeverContainsCredentials`）。

## 测试

`fake_smtp_test.go` 自带一个假 SMTP 服务器（现签 CA + 叶子证书、EHLO 能力通告、
STARTTLS 升级后才给 AUTH、DATA 读终止点），不引第三方 mock——
`net/smtp` 依赖的正是这些协议细节，自己实现一遍才看得见客户端到底发了什么。
`smtp_test.go` 覆盖两条拨号路径、证书拒绝、明文不送凭据、头注入、HTML 消毒与 MIME 组装；
`service/email_test.go` 覆盖报告内容、凭据不外泄与失败不外抛。

## 消费方

`service/email.go` 的 `SendCode` / `SendNotification` / `SendTest` 是唯一构造点
（每次发送重新 `BuildMailerConfig` + `mailer.New`）。新增发信场景请走 `EmailService`，
不要在别处另建 `mailer.New`——否则绕过了配置回退与重读语义。
