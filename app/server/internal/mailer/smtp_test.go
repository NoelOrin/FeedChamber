package mailer

import (
	"context"
	"crypto/tls"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// trustFakeCA 把假服务器的 CA 注入发送方的 TLS 配置。
// 生产路径永远走系统信任库（见 TestSend_RejectsUntrustedServerCert），
// 只有测试需要接受这张现签的自签证书。
func trustFakeCA(t *testing.T, s *fakeSMTP) {
	t.Helper()
	prev := newTLSConfig
	newTLSConfig = func(serverName string) *tls.Config {
		cfg := prev(serverName)
		cfg.RootCAs = s.ca.pool
		return cfg
	}
	t.Cleanup(func() { newTLSConfig = prev })
}

func smtpTestConfig(s *fakeSMTP) Config {
	return Config{
		Provider: "smtp",
		Host:     "127.0.0.1",
		Port:     s.port(),
		Username: "smtp-user",
		Password: "smtp-pass",
		From:     "bot@feedchamber.test",
		FromName: "Feed Chamber",
	}
}

func testMessage() Message {
	return Message{
		To:       []string{"alice@example.com", "bob@example.com"},
		Subject:  "重置密码验证码",
		HTMLBody: "<p>您的验证码是 <b>123456</b></p>",
		TextBody: "您的验证码是 123456",
	}
}

// 隐式 TLS（465）整条链路：握手 → AUTH → MAIL/RCPT/DATA。
func TestSend_ImplicitTLS(t *testing.T) {
	s := startFakeSMTP(t, true, true)
	trustFakeCA(t, s)

	cfg := smtpTestConfig(s)
	cfg.UseTLS = true
	sender, err := New(cfg)
	require.NoError(t, err)

	require.NoError(t, sender.Send(context.Background(), testMessage()))

	// 信封地址必须是不带显示名的裸地址：显示名只该出现在 From 头里。
	require.Equal(t, "bot@feedchamber.test", s.mail.mailFrom)
	require.Equal(t, []string{"alice@example.com", "bob@example.com"}, s.mail.rcpts)
	require.Equal(t, "\x00smtp-user\x00smtp-pass", s.mail.creds)
	require.Contains(t, s.mail.body, "From: Feed Chamber <bot@feedchamber.test>")
	require.Contains(t, s.mail.body, "multipart/alternative; boundary=")
	// 中文主题必须 MIME 编码，否则非 ASCII 直接进头会被判畸形/乱码。
	require.Contains(t, s.mail.body, "Subject: =?utf-8?q?")
	require.Contains(t, s.mail.body, "Content-Type: text/plain; charset=utf-8")
	require.Contains(t, s.mail.body, "Content-Type: text/html; charset=utf-8")
}

// 非 ASCII 显示名要编码进 From 头，但信封 MAIL FROM 仍必须是裸地址。
func TestSend_EncodesDisplayNameKeepsBareEnvelope(t *testing.T) {
	s := startFakeSMTP(t, true, true)
	trustFakeCA(t, s)

	cfg := smtpTestConfig(s)
	cfg.UseTLS = true
	cfg.FromName = "论坛通知"
	sender, err := New(cfg)
	require.NoError(t, err)

	require.NoError(t, sender.Send(context.Background(), Message{
		To: []string{"alice@example.com"}, Subject: "hi", TextBody: "hi",
	}))

	require.Contains(t, s.mail.body, "From: =?utf-8?q?")
	require.Contains(t, s.mail.body, "<bot@feedchamber.test>")
	require.Equal(t, "bot@feedchamber.test", s.mail.mailFrom)
}

// STARTTLS（587）：明文建连 → 升级 TLS → 才允许 AUTH。
func TestSend_StartTLS(t *testing.T) {
	s := startFakeSMTP(t, false, true)
	trustFakeCA(t, s)

	cfg := smtpTestConfig(s)
	cfg.UseTLS = false
	sender, err := New(cfg)
	require.NoError(t, err)

	require.NoError(t, sender.Send(context.Background(), testMessage()))

	require.Equal(t, "bot@feedchamber.test", s.mail.mailFrom)
	require.Equal(t, "\x00smtp-user\x00smtp-pass", s.mail.creds)
	require.Contains(t, s.mail.body, "Subject: =?utf-8?q?")
}

// 安全性质：证书链不可信时不发信（这条同时证明上面的用例真的走了 TLS 校验，
// 而不是恰好通过）。
func TestSend_RejectsUntrustedServerCert(t *testing.T) {
	s := startFakeSMTP(t, true, true) // 不注入 CA → 走系统信任库

	cfg := smtpTestConfig(s)
	cfg.UseTLS = true
	sender, err := New(cfg)
	require.NoError(t, err)

	err = sender.Send(context.Background(), testMessage())
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "certificate") || strings.Contains(err.Error(), "x509"),
		"expected a certificate verification failure, got: %v", err)
	require.Empty(t, s.mail.creds, "凭据不得在证书校验失败前送出")
}

// 安全性质：中继不通告 TLS 时直接失败，绝不在明文上送 AUTH。
func TestSend_StopsOnPlaintextOnlyRelay(t *testing.T) {
	s := startFakeSMTP(t, false, false)

	cfg := smtpTestConfig(s)
	cfg.UseTLS = false
	sender, err := New(cfg)
	require.NoError(t, err)

	err = sender.Send(context.Background(), testMessage())
	require.Error(t, err)
	require.Contains(t, err.Error(), "starttls")
	require.Empty(t, s.mail.creds, "凭据不得在明文连接上送出")
	require.Empty(t, s.mail.mailFrom, "不该进入 MAIL FROM")
}

// 收件人里的 CRLF 必须被剥掉，否则等于开放 SMTP 头注入 / 多次 RCPT。
func TestSend_SanitizesCRLFInRecipients(t *testing.T) {
	s := startFakeSMTP(t, true, true)
	trustFakeCA(t, s)

	cfg := smtpTestConfig(s)
	cfg.UseTLS = true
	sender, err := New(cfg)
	require.NoError(t, err)

	msg := Message{
		To:       []string{"alice@example.com\r\nRCPT TO:<evil@spam.test>"},
		Subject:  "hi",
		TextBody: "body",
	}
	require.NoError(t, sender.Send(context.Background(), msg))

	require.Len(t, s.mail.rcpts, 1, "注入的换行不得派生出第二个收件人")
	require.NotContains(t, s.mail.rcpts[0], "\r")
	require.NotContains(t, s.mail.rcpts[0], "\n")
	for _, line := range strings.Split(s.mail.body, "\r\n") {
		require.False(t, strings.HasPrefix(strings.ToUpper(line), "RCPT TO:"),
			"注入内容不得成为独立的头行: %q", line)
	}
}

// 正文里的 HTML 要先过消毒：script/onerror 不该出现在信体。
func TestSend_SanitizesHTMLBody(t *testing.T) {
	s := startFakeSMTP(t, true, true)
	trustFakeCA(t, s)

	cfg := smtpTestConfig(s)
	cfg.UseTLS = true
	sender, err := New(cfg)
	require.NoError(t, err)

	msg := Message{
		To:       []string{"alice@example.com"},
		Subject:  "hi",
		HTMLBody: `<p onclick="x()">正常</p><script>alert(1)</script><img src=x onerror=alert(1)>`,
	}
	require.NoError(t, sender.Send(context.Background(), msg))

	// 正文是 quoted-printable，中文会以 =E6=AD=A3 形式出现，所以只断言结构。
	require.Contains(t, s.mail.body, "<p>")
	require.NotContains(t, strings.ToLower(s.mail.body), "<script")
	require.NotContains(t, strings.ToLower(s.mail.body), "onerror")
	require.NotContains(t, strings.ToLower(s.mail.body), "onclick")
}

// host 与 from 是 SMTP 的两个必要条件，缺任一都要在构造期失败，
// 而不是等到用户点「忘记密码」时才报。
func TestNew_SMTPRequiresHostAndFrom(t *testing.T) {
	_, err := New(Config{Provider: "smtp", From: "a@b.test"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "host")

	_, err = New(Config{Provider: "smtp", Host: "smtp.test"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "from")

	_, err = New(Config{Provider: "smtp", Host: "smtp.test", From: "a@b.test"})
	require.NoError(t, err)
}

func TestNew_UnknownProviderFailsLoud(t *testing.T) {
	_, err := New(Config{Provider: "ses"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported provider")

	for _, p := range []string{"resend", "sendgrid"} {
		_, err := New(Config{Provider: p})
		require.Error(t, err, "%s 缺 api key 必须报错，不能静默降级成 log", p)
	}
}

// 空 / "log" 走 log-only：不发网络请求。
func TestNew_DefaultsToLogSender(t *testing.T) {
	for _, p := range []string{"", "log"} {
		s, err := New(Config{Provider: p})
		require.NoError(t, err)
		require.IsType(t, &logSender{}, s)
	}
}

// Port 未配置时按 TLS 模式补默认值：465=隐式，587=STARTTLS。
func TestSMTPAddr_DefaultsPortByTLSMode(t *testing.T) {
	require.Equal(t, "smtp.test:465", smtpAddr(Config{Host: "smtp.test", UseTLS: true}))
	require.Equal(t, "smtp.test:587", smtpAddr(Config{Host: "smtp.test", UseTLS: false}))
	require.Equal(t, "smtp.test:2525", smtpAddr(Config{Host: "smtp.test", Port: 2525, UseTLS: true}))
}

func TestBuildMIMEBody_MultipartWhenBothBodies(t *testing.T) {
	body, err := buildMIMEBody("=?utf-8?Q?Feed_Chamber?= <bot@x.test>", Message{
		To:       []string{"a@x.test"},
		Subject:  "验证码",
		HTMLBody: "<p>hi</p>",
		TextBody: "hi",
	})
	require.NoError(t, err)

	require.Contains(t, string(body), "From: =?utf-8?Q?Feed_Chamber?= <bot@x.test>")
	require.Contains(t, string(body), "To: a@x.test")
	require.Contains(t, string(body), "MIME-Version: 1.0")
	require.Contains(t, string(body), "multipart/alternative; boundary=")
	require.Contains(t, string(body), "Content-Type: text/plain; charset=utf-8")
	require.Contains(t, string(body), "Content-Type: text/html; charset=utf-8")
	// 纯文本 part 必须在 HTML part 之前（RFC 2046 的惯例，客户端据此挑选）。
	require.Less(t, strings.Index(string(body), "text/plain"), strings.Index(string(body), "text/html"))
}

func TestBuildMIMEBody_SinglePartFallbacks(t *testing.T) {
	htmlOnly, err := buildMIMEBody("bot@x.test", Message{To: []string{"a@x.test"}, Subject: "s", HTMLBody: "<p>hi</p>"})
	require.NoError(t, err)
	require.NotContains(t, string(htmlOnly), "multipart/")
	require.Contains(t, string(htmlOnly), "Content-Type: text/html; charset=utf-8")

	textOnly, err := buildMIMEBody("bot@x.test", Message{To: []string{"a@x.test"}, Subject: "s", TextBody: "hi"})
	require.NoError(t, err)
	require.Contains(t, string(textOnly), "Content-Type: text/plain; charset=utf-8")
}

// 报告里的端口必须与实际拨号端口同源，否则后台显示的 587 可能对应一次 465 拨号。
func TestEffectivePort_DefaultsByTLSMode(t *testing.T) {
	require.Equal(t, 465, EffectivePort(Config{UseTLS: true}))
	require.Equal(t, 587, EffectivePort(Config{}))
	require.Equal(t, 2525, EffectivePort(Config{Port: 2525, UseTLS: true}))
}
