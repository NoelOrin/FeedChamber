package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type WebhookHandler interface {
	Create(ctx iris.Context)
	List(ctx iris.Context)
	Update(ctx iris.Context)
	Delete(ctx iris.Context)
	ListDeliveries(ctx iris.Context)
	Ping(ctx iris.Context)
	RetryDelivery(ctx iris.Context)
}

type webhookHandler struct{ svc service.WebhookService }

func newWebhookHandler(svc service.WebhookService) WebhookHandler {
	return &webhookHandler{svc: svc}
}

// 创建 Webhook
// @Summary      创建 Webhook
// @Description  订阅事件总线上的事件，投递到指定 URL。
// @Tags         Webhook
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{url=string,secret=string,events=[]string,enabled=bool,tls_verify=bool} true "Webhook 配置"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/webhooks/create [post]
func (h *webhookHandler) Create(ctx iris.Context) {
	var body struct {
		URL       string   `json:"url"`
		Secret    string   `json:"secret"`
		Events    []string `json:"events"`
		Enabled   *bool    `json:"enabled"`
		TLSVerify *bool    `json:"tls_verify"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	tlsVerify := true
	if body.TLSVerify != nil {
		tlsVerify = *body.TLSVerify
	}
	w, err := h.svc.Create(ctx.Request().Context(), body.URL, body.Secret, body.Events, enabled, tlsVerify)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, w)
}

// Webhook 列表
// @Summary      Webhook 列表
// @Description  分页列出已配置的 Webhook。
// @Tags         Webhook
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{page=int,page_size=int} false "分页参数"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/webhooks/list [post]
func (h *webhookHandler) List(ctx iris.Context) {
	p, err := readListParams(ctx)
	if err != nil {
		return
	}
	items, total, err := h.svc.List(ctx.Request().Context(), p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, total)
}

// 更新 Webhook
// @Summary      更新 Webhook
// @Description  修改 URL、密钥、订阅事件或启用状态。
// @Tags         Webhook
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Param        body body object{url=string,secret=string,events=[]string,enabled=bool} false "更新字段"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/webhooks/{id}/update [post]
func (h *webhookHandler) Update(ctx iris.Context) {
	id := ctx.Params().Get("id")
	var body struct {
		URL     string   `json:"url"`
		Secret  string   `json:"secret"`
		Events  []string `json:"events"`
		Enabled *bool    `json:"enabled"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	w, err := h.svc.Update(ctx.Request().Context(), id, body.URL, body.Secret, body.Events, body.Enabled)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, w)
}

// 删除 Webhook
// @Summary      删除 Webhook
// @Description  删除 Webhook 及其投递记录。
// @Tags         Webhook
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/webhooks/{id}/delete [post]
func (h *webhookHandler) Delete(ctx iris.Context) {
	id := ctx.Params().Get("id")
	if err := h.svc.Delete(ctx.Request().Context(), id); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// 投递记录列表
// @Summary      投递记录列表
// @Description  分页列出该 Webhook 的投递记录与结果。
// @Tags         Webhook
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Param        body body object{page=int,page_size=int} false "分页参数"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/webhooks/{id}/deliveries/list [post]
func (h *webhookHandler) ListDeliveries(ctx iris.Context) {
	id := ctx.Params().Get("id")
	p, err := readListParams(ctx)
	if err != nil {
		return
	}
	items, total, err := h.svc.ListDeliveries(ctx.Request().Context(), id, p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, total)
}

// Webhook 测试投递
// @Summary      Webhook 测试投递
// @Description  向该 Webhook 立即发送一条测试事件。
// @Tags         Webhook
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/webhooks/{id}/ping [post]
func (h *webhookHandler) Ping(ctx iris.Context) {
	d, err := h.svc.Ping(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, d)
}

// 重投投递
// @Summary      重投投递
// @Description  重新投递某条失败的投递记录。
// @Tags         Webhook
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Param        deliveryId path string true "投递记录 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/webhooks/{id}/deliveries/{deliveryId}/retry [post]
func (h *webhookHandler) RetryDelivery(ctx iris.Context) {
	d, err := h.svc.RetryDelivery(ctx.Request().Context(), ctx.Params().Get("id"), ctx.Params().Get("deliveryId"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, d)
}
