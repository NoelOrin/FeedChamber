package handler

import (
	"context"
	"testing"

	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
	"github.com/stretchr/testify/require"
)

// noopAuditLog 让 handler 的审计写入不依赖真实仓储。
type noopAuditLog struct{ service.AuditLogService }

func (noopAuditLog) Log(context.Context, string, string, string, string) error    { return nil }
func (noopAuditLog) Record(context.Context, string, string, string, string) error { return nil }

// stubEmailSvc 只实现 SendTest；其余方法由嵌入的 nil 接口占位。
type stubEmailSvc struct {
	service.EmailService
	gotTo string
	rep   *service.MailTestReport
	err   error
}

func (s *stubEmailSvc) SendTest(_ context.Context, to string) (*service.MailTestReport, error) {
	s.gotTo = to
	return s.rep, s.err
}

func newEmailTestApp(svc service.EmailService) *iris.Application {
	app := iris.New()
	h := newSettingHandler(nil, noopAuditLog{}, svc)
	app.Post("/api/v1/admin/settings/email/test", h.SendEmailTest)
	return app
}

// 契约：成功返回统一信封，data 是诊断报告。
func TestSettingHandler_SendEmailTest_Envelope(t *testing.T) {
	svc := &stubEmailSvc{rep: &service.MailTestReport{Ok: true, Provider: "smtp", Host: "smtp.test", Port: 587}}
	e := httptest.New(t, newEmailTestApp(svc))

	resp := e.POST("/api/v1/admin/settings/email/test").WithJSON(iris.Map{"to": "admin@example.com"}).
		Expect().Status(200).JSON().Object()
	resp.Value("code").Number().IsEqual(0)
	resp.Value("msg").String().IsEqual("success")
	resp.Value("data").Object().Value("ok").Boolean().IsTrue()
	resp.Value("data").Object().Value("provider").String().IsEqual("smtp")

	require.Equal(t, "admin@example.com", svc.gotTo)
}

// 契约：投递失败不是 500——报告原样回给前端，管理员才看得到原因。
func TestSettingHandler_SendEmailTest_TransportFailureIsStill200(t *testing.T) {
	svc := &stubEmailSvc{rep: &service.MailTestReport{Ok: false, Error: "smtp dial: connection refused"}}
	e := httptest.New(t, newEmailTestApp(svc))

	resp := e.POST("/api/v1/admin/settings/email/test").WithJSON(iris.Map{"to": "admin@example.com"}).
		Expect().Status(200).JSON().Object()
	resp.Value("code").Number().IsEqual(0)
	resp.Value("data").Object().Value("ok").Boolean().IsFalse()
	resp.Value("data").Object().Value("error").String().Contains("connection refused")
}

// 契约：参数错误走业务码（2xxx → 400），不落到 500。
func TestSettingHandler_SendEmailTest_ParamError(t *testing.T) {
	svc := &stubEmailSvc{err: pkg.NewAppError(pkg.INVALID_PARAMS, "收件人邮箱格式不正确")}
	e := httptest.New(t, newEmailTestApp(svc))

	resp := e.POST("/api/v1/admin/settings/email/test").WithJSON(iris.Map{"to": "nope"}).
		Expect().Status(400).JSON().Object()
	resp.Value("code").Number().IsEqual(int64(pkg.INVALID_PARAMS))
}
