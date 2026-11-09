package handler

import (
	"context"
	"testing"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
	"github.com/stretchr/testify/require"
)

// stubHistoryService 只实现本用例关心的四个出口，其余由嵌入的 nil 接口占位。
// handler 层只验参数解析、ctx 身份注入与统一信封，业务判定在 service 层的
// history_test.go（真实 sqlite）里。
type stubHistoryService struct {
	service.PostService
	listPage      int
	listPageSize  int
	viewPostID    string
	viewCursor    string
	deletedPostID string
	cleared       bool
	viewErr       error
}

func (s *stubHistoryService) ListHistory(_ context.Context, page, pageSize int) ([]model.PostListItem, int, error) {
	s.listPage, s.listPageSize = page, pageSize
	return []model.PostListItem{{ID: "p1", LastContentID: "c-9", LastViewedAt: nil}}, 1, nil
}

func (s *stubHistoryService) RecordView(_ context.Context, postID, lastContentID string) error {
	s.viewPostID, s.viewCursor = postID, lastContentID
	return s.viewErr
}

func (s *stubHistoryService) DeleteHistory(_ context.Context, postID string) error {
	s.deletedPostID = postID
	return nil
}

func (s *stubHistoryService) ClearHistory(_ context.Context) error {
	s.cleared = true
	return nil
}

// historyTestApp 复刻网关在 protected 路由上做的事：把会话身份写进 ctx values，
// 再挂上被测 handler。不挂 Enforce 是因为本用例测的是 handler 本身。
func historyTestApp(svc service.PostService) *iris.Application {
	app := iris.New()
	h := newHistoryHandler(svc)
	ph := newPostHandler(svc)
	app.Use(func(ctx iris.Context) {
		ctx.Values().Set(pkg.CtxUserID, "u-1")
		ctx.Values().Set(pkg.CtxRole, "user")
		ctx.Next()
	})
	app.Post("/api/v1/history/list", h.List)
	app.Post("/api/v1/history/clear", h.Clear)
	app.Post("/api/v1/history/{postId}/delete", h.Delete)
	app.Post("/api/v1/posts/{id}/view", ph.RecordView)
	return app
}

// 列表：空 body 是合法请求（默认分页），不能打成 400。
// 这条是仓库里反复踩的坑：ctx.ReadJSON 在无 body 时返回 io.EOF。
func TestHistoryHandler_List_EmptyBodyUsesDefaultPagination(t *testing.T) {
	svc := &stubHistoryService{}
	e := httptest.New(t, historyTestApp(svc))

	resp := e.POST("/api/v1/history/list").Expect().Status(200).JSON().Object()
	resp.Value("code").Number().IsEqual(0)
	resp.Value("msg").String().IsEqual("success")
	require.Equal(t, 1, svc.listPage)
	require.Equal(t, pkg.PageDefaultSize, svc.listPageSize)
}

func TestHistoryHandler_List_BodyCarriesPageAndPageSize(t *testing.T) {
	svc := &stubHistoryService{}
	e := httptest.New(t, historyTestApp(svc))

	e.POST("/api/v1/history/list").WithJSON(iris.Map{"page": 3, "pageSize": 7}).
		Expect().Status(200)
	require.Equal(t, 3, svc.listPage)
	require.Equal(t, 7, svc.listPageSize)
}

// 列表响应必须带 total，且字段从 model.PostListItem 里透出续读游标。
func TestHistoryHandler_List_ReturnsCursorAndTotal(t *testing.T) {
	svc := &stubHistoryService{}
	e := httptest.New(t, historyTestApp(svc))

	resp := e.POST("/api/v1/history/list").WithJSON(iris.Map{}).Expect().Status(200).JSON().Object()
	data := resp.Value("data").Object()
	data.Value("total").Number().IsEqual(1)
	items := data.Value("items").Array()
	items.Length().IsEqual(1)
	items.Element(0).Object().Value("last_content_id").String().IsEqual("c-9")
}

func TestHistoryHandler_Delete_UsesPathParam(t *testing.T) {
	svc := &stubHistoryService{}
	e := httptest.New(t, historyTestApp(svc))

	e.POST("/api/v1/history/p-42/delete").Expect().Status(200).JSON().Object().
		Value("code").Number().IsEqual(0)
	require.Equal(t, "p-42", svc.deletedPostID)
}

func TestHistoryHandler_Clear_SuccessEnvelope(t *testing.T) {
	svc := &stubHistoryService{}
	e := httptest.New(t, historyTestApp(svc))

	e.POST("/api/v1/history/clear").Expect().Status(200).JSON().Object().
		Value("code").Number().IsEqual(0)
	require.True(t, svc.cleared)
}

// 上报阅读位置：body 里的 last_content_id 必须原样透传到 service，
// 空 body 也不能打成 400（前端只在滚动后才带游标）。
func TestPostHandler_RecordView_ForwardsCursorAndToleratesEmptyBody(t *testing.T) {
	svc := &stubHistoryService{}
	e := httptest.New(t, historyTestApp(svc))

	e.POST("/api/v1/posts/p1/view").WithJSON(iris.Map{"last_content_id": "c-12"}).
		Expect().Status(200).JSON().Object().Value("code").Number().IsEqual(0)
	require.Equal(t, "p1", svc.viewPostID)
	require.Equal(t, "c-12", svc.viewCursor)

	e.POST("/api/v1/posts/p1/view").Expect().Status(200)
	require.Equal(t, "", svc.viewCursor)
}

// service 的业务错误必须经 HandleError 变成对应业务码，不能被吞成 200。
func TestPostHandler_RecordView_PropagatesServiceError(t *testing.T) {
	svc := &stubHistoryService{viewErr: pkg.NewAppError(pkg.NOT_FOUND, "评论不存在")}
	e := httptest.New(t, historyTestApp(svc))

	e.POST("/api/v1/posts/p1/view").WithJSON(iris.Map{"last_content_id": "bogus"}).
		Expect().Status(404).JSON().Object().
		Value("code").Number().IsEqual(int64(pkg.NOT_FOUND))
}
