package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type MotionHandler interface {
	Create(ctx iris.Context)
	Get(ctx iris.Context)
	List(ctx iris.Context)
	Vote(ctx iris.Context)
	Resolve(ctx iris.Context)
	UpdateStatus(ctx iris.Context)
	Delete(ctx iris.Context)
}

type motionHandler struct {
	svc   service.MotionService
	audit service.AuditLogService
}

func newMotionHandler(svc service.MotionService, audit service.AuditLogService) MotionHandler {
	return &motionHandler{svc: svc, audit: audit}
}

// Create 发起自治动议
// @Summary      发起自治动议
// @Description  对帖子/评论/用户发起 fold/watch/escalate/exile/reverse 动议（社区集体裁判，非举报）
// @Tags         自治动议
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{target_type=string,target_id=string,type=string,reason=string,source_motion_id=string} true "动议信息"
// @Success      201  {object}  pkg.Response{data=model.Motion}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /motions/create [post]
func (h *motionHandler) Create(ctx iris.Context) {
	var body struct {
		TargetType     string `json:"target_type"`
		TargetID       string `json:"target_id"`
		Type           string `json:"type"`
		Reason         string `json:"reason"`
		SourceMotionID string `json:"source_motion_id"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	m, err := h.svc.Create(ctx.Request().Context(), u.ID, body.TargetType, body.TargetID, body.Type, body.Reason, body.SourceMotionID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	ctx.StatusCode(iris.StatusCreated)
	pkg.Success(ctx, m)
}

// Get 动议详情
// @Summary      动议详情
// @Description  通过 ID 获取单个动议及其票数
// @Tags         自治动议
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "动议 ID"
// @Success      200  {object}  pkg.Response{data=model.Motion}
// @Failure      401  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /motions/{id} [get]
func (h *motionHandler) Get(ctx iris.Context) {
	m, err := h.svc.GetByID(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, m)
}

// List 动议列表
// @Summary      动议列表
// @Description  分页获取动议，可按状态与目标类型筛选
// @Tags         自治动议
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{status=string,target_type=string,page=int,page_size=int} true "筛选与分页"
// @Success      200  {object}  pkg.Response{data=[]model.MotionListItem}
// @Failure      401  {object}  pkg.Response
// @Router       /motions/list [post]
func (h *motionHandler) List(ctx iris.Context) {
	var req struct {
		Status     string `json:"status"`
		TargetType string `json:"target_type"`
		Page       int    `json:"page"`
		PageSize   int    `json:"page_size"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	req.Page, req.PageSize = pkg.NormalizePagination(req.Page, req.PageSize)
	items, total, err := h.svc.List(ctx.Request().Context(), req.Status, req.TargetType, req.Page, req.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, int(total))
}

// Vote 动议投票
// @Summary      动议投票
// @Description  信誉加权投票，side=approve|reject，一人一票
// @Tags         自治动议
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "动议 ID"
// @Param        body body object{side=string} true "vote"
// @Success      200  {object}  pkg.Response{data=model.Motion}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /motions/{id}/vote [post]
func (h *motionHandler) Vote(ctx iris.Context) {
	var body struct {
		Side string `json:"side"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	m, err := h.svc.Vote(ctx.Request().Context(), ctx.Params().Get("id"), u.ID, body.Side)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, m)
}

// Resolve 结算动议
// @Summary      结算动议
// @Description  开放期满或达法定人数后结算动议（motion:manage）
// @Tags         自治动议
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "动议 ID"
// @Success      200  {object}  pkg.Response{data=model.Motion}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /motions/{id}/resolve [post]
func (h *motionHandler) Resolve(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	m, err := h.svc.Resolve(ctx.Request().Context(), ctx.Params().Get("id"), u.ID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), u.ID, "motion.resolve", m.ID, "")
	pkg.Success(ctx, m)
}

// UpdateStatus 状态变更（翻案 / 标记滥诉 / 升级硬封禁）
// @Summary      动议状态变更
// @Description  motion:manage：翻案（overturned）解除软隔离、标记滥诉、将软隔离升级为硬封禁
// @Tags         自治动议
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "动议 ID"
// @Param        body body object{status=string,abuse=bool,hard_ban=bool} true "状态变更"
// @Success      200  {object}  pkg.Response{data=model.Motion}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /motions/{id}/status [post]
func (h *motionHandler) UpdateStatus(ctx iris.Context) {
	var body struct {
		Status  string `json:"status"`
		Abuse   *bool  `json:"abuse"`
		HardBan bool   `json:"hard_ban"`
	}
	if err := ctx.ReadJSON(&body); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	m, err := h.svc.UpdateStatus(ctx.Request().Context(), ctx.Params().Get("id"), u.ID, body.Status, body.Abuse, body.HardBan)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), u.ID, "motion.status", m.ID,
		"status="+body.Status+" hard_ban="+boolStr(body.HardBan))
	pkg.Success(ctx, m)
}

// Delete 撤回动议
// @Summary      撤回动议
// @Description  发起者撤回进行中（pending）的动议
// @Tags         自治动议
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "动议 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /motions/{id}/delete [post]
func (h *motionHandler) Delete(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id"), u.ID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	_ = h.audit.Record(ctx.Request().Context(), u.ID, "motion.delete", ctx.Params().Get("id"), "")
	pkg.Success(ctx, nil)
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
