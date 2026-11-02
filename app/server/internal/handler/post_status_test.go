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

// stubPostService 只实现 SetStatus，其余方法由嵌入的 nil 接口占位——
// 本测试只关心 handler 的参数解析与响应信封，不触达业务层。
type stubPostService struct {
	service.PostService
	lastID     string
	lastStatus string
	err        error
}

func (s *stubPostService) SetStatus(_ context.Context, id, status string) error {
	s.lastID = id
	s.lastStatus = status
	return s.err
}

func newPostStatusApp(svc service.PostService) *iris.Application {
	app := iris.New()
	app.Post("/api/v1/posts/{id}/status", newPostHandler(svc).SetStatus)
	return app
}

// 契约 1：成功返回统一信封 {code:0,msg:"success",data:null}。
func TestPostHandler_SetStatus_SuccessEnvelope(t *testing.T) {
	svc := &stubPostService{}
	e := httptest.New(t, newPostStatusApp(svc))

	resp := e.POST("/api/v1/posts/p1/status").WithJSON(iris.Map{"status": "approved"}).
		Expect().Status(200).JSON().Object()
	resp.Value("code").Number().IsEqual(0)
	resp.Value("msg").String().IsEqual("success")

	require.Equal(t, "p1", svc.lastID)
	require.Equal(t, "approved", svc.lastStatus)
}

// 契约 1：非法状态值返回 2xxx 参数错误。
func TestPostHandler_SetStatus_InvalidStatusIsParamError(t *testing.T) {
	svc := &stubPostService{err: pkg.NewAppError(pkg.INVALID_PARAMS, "status 必须为 approved、pending、rejected 或 closed")}
	e := httptest.New(t, newPostStatusApp(svc))

	e.POST("/api/v1/posts/p1/status").WithJSON(iris.Map{"status": "whatever"}).
		Expect().Status(400).JSON().Object().
		Value("code").Number().IsEqual(int64(pkg.INVALID_PARAMS))
}
