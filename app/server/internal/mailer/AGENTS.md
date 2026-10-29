# Mailer 邮件发送抽象

支持四种邮件发送实现，由 settings 表配置。

## 接口

```go
type Mailer interface {
    Send(to, subject, body string) error
}
```

## 实现

| 实现 | 驱动 | 选择条件 |
|------|------|---------|
| SMTP | net/smtp | setting `mail_provider` == "smtp" |
| SendGrid | sendgrid-go | setting `mail_provider` == "sendgrid" |
| Resend | resend-go | setting `mail_provider` == "resend" |
| Log | logrus | setting `mail_provider` 为空或未配置（降级，仅打印日志） |

## 依赖

`SettingService` 在启动时注入，读取：
- `mail_provider`, `smtp_host`, `smtp_port`, `smtp_user`, `smtp_pass`, `sendgrid_api_key`, `resend_api_key`, `mail_from`
