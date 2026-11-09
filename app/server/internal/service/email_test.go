package service

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"feedchamber/internal/mock"
	"feedchamber/internal/pkg"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// closedPort 返回一个刚被释放的端口，用来稳定复现「连不上」。
func closedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	return port
}

func mailSettings(vals map[string]string) *mock.SettingService {
	s := mock.NewSettingService()
	for k, v := range vals {
		s = s.WithSetting(k, v)
	}
	return s
}

// 收件人缺失/畸形要在发出任何 SMTP 连接之前挡住，并回 2xxx 参数错误。
func TestSendTest_RejectsBadRecipient(t *testing.T) {
	ctx := context.Background()
	svc := &emailService{settingSvc: mailSettings(map[string]string{"email_provider": "log"})}

	_, err := svc.SendTest(ctx, "")
	require.Error(t, err)
	assert.Equal(t, pkg.INVALID_PARAMS, err.(*pkg.AppError).Code)

	_, err = svc.SendTest(ctx, "not-an-email")
	require.Error(t, err)
	assert.Equal(t, pkg.INVALID_PARAMS, err.(*pkg.AppError).Code)
}

// provider=log 不会真的投递：必须成功但在报告里说清楚，
// 否则管理员看到「发送成功」会以为收件人收到了。
func TestSendTest_LogProviderSaysItDidNotReallySend(t *testing.T) {
	ctx := context.Background()
	svc := &emailService{settingSvc: mailSettings(map[string]string{"email_provider": "log"})}

	rep, err := svc.SendTest(ctx, "admin@example.com")
	require.NoError(t, err)
	require.NotNil(t, rep)
	assert.True(t, rep.Ok)
	assert.Equal(t, "log", rep.Provider)
	assert.Contains(t, rep.Notice, "log", "log provider 必须说清楚没有真投递")
}

// 配置本身不合法（SMTP 缺 host）时不能把请求打成 500：
// 诊断结果要作为数据回给后台，让管理员看到原因。
func TestSendTest_ReportsInvalidConfigAsData(t *testing.T) {
	ctx := context.Background()
	svc := &emailService{settingSvc: mailSettings(map[string]string{
		"email_provider": "smtp",
		"email_from":     "bot@example.com",
	})}

	rep, err := svc.SendTest(ctx, "admin@example.com")
	require.NoError(t, err)
	assert.False(t, rep.Ok)
	assert.Contains(t, rep.Error, "host")
}

// 连不上服务器同理：Ok=false + 原因，HTTP 仍是成功响应。
func TestSendTest_ReportsTransportFailure(t *testing.T) {
	ctx := context.Background()
	svc := &emailService{settingSvc: mailSettings(map[string]string{
		"email_provider":  "smtp",
		"email_smtp_host": "127.0.0.1",
		"email_smtp_port": "1",
		"email_username":  "u",
		"email_password":  "sup3rs3cret",
		"email_from":      "bot@example.com",
		"email_use_tls":   "false",
		"email_api_key":   "krl-key-should-not-leak",
		"email_enabled":   "false",
		"email_from_name": "Feed Chamber",
	})}

	rep, err := svc.SendTest(ctx, "admin@example.com")
	require.NoError(t, err)
	assert.False(t, rep.Ok)
	assert.NotEmpty(t, rep.Error)
	assert.Equal(t, "smtp", rep.Provider)
	assert.Equal(t, 1, rep.Port)
	// email_enabled 只影响用户侧验证码流程，测试发信照跑，但要把状态回出来。
	assert.False(t, rep.EmailEnabled)
}

// 报告里绝不能出现凭据——整个结构体就是「能安全回给前端」的那一层。
func TestSendTest_ReportNeverContainsCredentials(t *testing.T) {
	ctx := context.Background()
	svc := &emailService{settingSvc: mailSettings(map[string]string{
		"email_provider":  "smtp",
		"email_smtp_host": "127.0.0.1",
		"email_smtp_port": "1",
		"email_username":  "smtpuser",
		"email_password":  "sup3rs3cret",
		"email_api_key":   "resend-key-abcdef",
		"email_from":      "bot@example.com",
		"email_use_tls":   "false",
	})}

	rep, err := svc.SendTest(ctx, "admin@example.com")
	require.NoError(t, err)

	raw, marshalErr := json.Marshal(rep)
	require.NoError(t, marshalErr)
	body := string(raw)
	assert.NotContains(t, body, "sup3rs3cret")
	assert.NotContains(t, body, "resend-key-abcdef")
}

// 成功路径要把 TLS 模式与目标主机回出来，管理员才知道改的配置生效了没有。
func TestSendTest_ReportsEffectiveRoute(t *testing.T) {
	ctx := context.Background()
	svc := &emailService{settingSvc: mailSettings(map[string]string{
		"email_provider":  "smtp",
		"email_smtp_host": "smtp.example.com",
		"email_smtp_port": "465",
		"email_use_tls":   "true",
		"email_from":      "bot@example.com",
		"email_enabled":   "true",
	})}

	// 域名解析不到的话只断言到「有原因」即可；这里只检查报告字段来自生效配置。
	rep, err := svc.SendTest(ctx, "admin@example.com")
	require.NoError(t, err)
	assert.Equal(t, "smtp.example.com", rep.Host)
	assert.Equal(t, 465, rep.Port)
	assert.Equal(t, "implicit", rep.TLSMode)
	assert.True(t, rep.EmailEnabled)
	assert.False(t, rep.Ok, "解析不到主机时不该假装成功")
}
