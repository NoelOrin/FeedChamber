package handler

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
	"github.com/sirupsen/logrus"
)

type BannedHandler interface {
	// 全局禁言
	Ban(ctx iris.Context)
	Unban(ctx iris.Context)
	Get(ctx iris.Context)

	// 版面禁言
	BanInSpace(ctx iris.Context)
	UnbanInSpace(ctx iris.Context)
	GetInSpace(ctx iris.Context)

	// 通用
	List(ctx iris.Context)
	ListByUserID(ctx iris.Context)
}

type bannedHandler struct {
	svc         service.BannedService
	user        service.UserService
	permSvc     service.PermissionChecker
	settingSvc  service.SettingService
	auditLogSvc service.AuditLogService
}

type banRequest struct {
	Reason          string  `json:"reason"`
	IsPermanent     bool    `json:"is_permanent"`
	DurationMinutes *int    `json:"duration_minutes"`
	ExpiresAt       *string `json:"expires_at"`
}

func normalizeBanRequest(req *banRequest) error {
	if !req.IsPermanent && req.DurationMinutes == nil && req.ExpiresAt == nil {
		req.IsPermanent = true
		return nil
	}
	if req.IsPermanent {
		return nil
	}
	if req.DurationMinutes != nil && *req.DurationMinutes > 0 {
		return nil
	}
	if req.ExpiresAt == nil {
		return errors.New("临时禁言必须指定有效时长或到期时间")
	}
	expiresAt, err := time.Parse(time.RFC3339, *req.ExpiresAt)
	if err != nil {
		return errors.New("expires_at 必须是 RFC3339 时间")
	}
	if !expiresAt.After(time.Now()) {
		return errors.New("expires_at 必须晚于当前时间")
	}
	minutes := int(expiresAt.Sub(time.Now()).Minutes())
	if minutes < 1 {
		minutes = 1
	}
	req.DurationMinutes = &minutes
	return nil
}

func newBannedHandler(svc service.BannedService, user service.UserService, permSvc service.PermissionChecker, settingSvc service.SettingService, auditLogSvc service.AuditLogService) BannedHandler {
	return &bannedHandler{svc: svc, user: user, permSvc: permSvc, settingSvc: settingSvc, auditLogSvc: auditLogSvc}
}

// bannedStatusAllowed checks whether the caller may read ban status for targetUUID.
// Returns false → caller should respond with 404.
func (h *bannedHandler) bannedStatusAllowed(ctx iris.Context, targetUUID string) bool {
	visible, _ := h.settingSvc.Get(ctx.Request().Context(), "banned_status_visible")
	if visible != "true" && targetUUID != currentUserID(ctx) {
		return false
	}
	publicAccess, _ := h.settingSvc.Get(ctx.Request().Context(), "banned_status_public_access")
	if publicAccess == "true" {
		return true
	}
	return targetUUID == currentUserID(ctx) || h.permSvc.HasPermission(currentRole(ctx), model.PermBannedManage)
}

// resolveUserID resolves the URL param "id" to User.ID (UUID).
func (h *bannedHandler) resolveUserID(ctx iris.Context) (string, bool) {
	user, err := h.user.GetByID(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return "", false
	}
	return user.ID, true
}

// Ban 禁言用户
// @Summary      禁言用户
// @Description  禁言指定用户，支持永久或临时禁言
// @Tags         禁言
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "用户 ID"
// @Param        body body      object{reason=string,is_permanent=boolean,duration_minutes=integer}  true  "禁言信息"
// @Success      200  {object}  pkg.Response{data=model.Banned}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /user/{id}/banned [post]
func (h *bannedHandler) Ban(ctx iris.Context) {
	var req banRequest
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := normalizeBanRequest(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}

	u, ok := requireUser(ctx)
	if !ok {
		return
	}

	userUUID, ok := h.resolveUserID(ctx)
	if !ok {
		return
	}

	banned, err := h.svc.Ban(ctx.Request().Context(), userUUID, req.Reason, u.Username, req.IsPermanent, req.DurationMinutes)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	detail, _ := json.Marshal(map[string]any{"reason": req.Reason, "is_permanent": req.IsPermanent, "duration_minutes": req.DurationMinutes})
	if err := h.auditLogSvc.Record(ctx.Request().Context(), u.ID, "user.ban", userUUID, string(detail)); err != nil {
		logrus.Warnf("audit log write failed: %v", err)
	}
	pkg.Success(ctx, banned)
}

// Unban 解禁用户
// @Summary      解禁用户
// @Description  解除对指定用户的禁言
// @Tags         禁言
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "用户 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /user/{id}/unbanned [post]
func (h *bannedHandler) Unban(ctx iris.Context) {
	userUUID, ok := h.resolveUserID(ctx)
	if !ok {
		return
	}
	if err := h.svc.Unban(ctx.Request().Context(), userUUID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	if err := h.auditLogSvc.Record(ctx.Request().Context(), currentUserID(ctx), "user.unban", userUUID, ""); err != nil {
		logrus.Warnf("audit log write failed: %v", err)
	}
	pkg.Success(ctx, nil)
}

// Get 获取用户的禁言记录
// @Summary      获取用户禁言记录
// @Description  获取指定用户的禁言状态和记录
// @Tags         禁言
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "用户 ID"
// @Success      200  {object}  pkg.Response{data=model.Banned}
// @Failure      401  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /user/{id}/banned/status [post]
func (h *bannedHandler) Get(ctx iris.Context) {
	userUUID, ok := h.resolveUserID(ctx)
	if !ok {
		return
	}

	// 查看他人禁言需 PermBannedManage
	if !h.bannedStatusAllowed(ctx, userUUID) {
		pkg.Fail(ctx, pkg.NOT_FOUND, "")
		return
	}

	banned, err := h.svc.GetByUserID(ctx.Request().Context(), userUUID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, banned)
}

// BanInSpace 在指定版面禁言用户
// @Summary      版面禁言用户
// @Description  在指定版面禁言用户，支持永久或临时禁言
// @Tags         版面禁言
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id        path      string  true  "用户 ID"
// @Param        space_id  path      string  true  "版面 ID"
// @Param        body      body      object{reason=string,is_permanent=boolean,duration_minutes=integer}  true  "禁言信息"
// @Success      200  {object}  pkg.Response{data=model.Banned}
// @Failure      400  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /user/{id}/space/{space_id}/banned [post]
func (h *bannedHandler) BanInSpace(ctx iris.Context) {
	var req banRequest
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := normalizeBanRequest(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}

	u, ok := requireUser(ctx)
	if !ok {
		return
	}

	userUUID, ok := h.resolveUserID(ctx)
	if !ok {
		return
	}

	spaceID := ctx.Params().Get("space_id")
	banned, err := h.svc.BanInSpace(ctx.Request().Context(), userUUID, spaceID, req.Reason, u.Username, req.IsPermanent, req.DurationMinutes)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	detail, _ := json.Marshal(map[string]any{"space_id": spaceID, "reason": req.Reason, "is_permanent": req.IsPermanent, "duration_minutes": req.DurationMinutes})
	if err := h.auditLogSvc.Record(ctx.Request().Context(), u.ID, "user.ban_space", userUUID, string(detail)); err != nil {
		logrus.Warnf("audit log write failed: %v", err)
	}
	pkg.Success(ctx, banned)
}

// UnbanInSpace 解除指定版面禁言
// @Summary      解除版面禁言
// @Description  解除对指定用户在指定版面的禁言
// @Tags         版面禁言
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id        path      string  true  "用户 ID"
// @Param        space_id  path      string  true  "版面 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /user/{id}/space/{space_id}/unbanned [post]
func (h *bannedHandler) UnbanInSpace(ctx iris.Context) {
	userUUID, ok := h.resolveUserID(ctx)
	if !ok {
		return
	}
	spaceID := ctx.Params().Get("space_id")
	if err := h.svc.UnbanInSpace(ctx.Request().Context(), userUUID, spaceID); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	detail, _ := json.Marshal(map[string]any{"space_id": spaceID})
	if err := h.auditLogSvc.Record(ctx.Request().Context(), currentUserID(ctx), "user.unban_space", userUUID, string(detail)); err != nil {
		logrus.Warnf("audit log write failed: %v", err)
	}
	pkg.Success(ctx, nil)
}

// GetInSpace 获取用户在指定版面的禁言状态
// @Summary      获取版面禁言状态
// @Description  获取指定用户在指定版面的禁言状态和记录
// @Tags         版面禁言
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id        path      string  true  "用户 ID"
// @Param        space_id  path      string  true  "版面 ID"
// @Success      200  {object}  pkg.Response{data=model.Banned}
// @Failure      401  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /user/{id}/space/{space_id}/banned/status [post]
func (h *bannedHandler) GetInSpace(ctx iris.Context) {
	userUUID, ok := h.resolveUserID(ctx)
	if !ok {
		return
	}

	// 查看他人禁言需 PermBannedManage
	if !h.bannedStatusAllowed(ctx, userUUID) {
		pkg.Fail(ctx, pkg.NOT_FOUND, "")
		return
	}

	banned, err := h.svc.GetInSpace(ctx.Request().Context(), userUUID, ctx.Params().Get("space_id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, banned)
}

// BannedListView 是禁言列表的对外形状：在 model.Banned 之上补上用户侧信息。
// 后台表格列（用户名/邮箱/禁言时间/操作人）都依赖这些字段；api-client 的
// BannedUser 类型也早已按这个形状声明，此前直接回 model.Banned 导致列表
// 用户名、邮箱、"禁言时间" 三列全空。
type BannedListView struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	Scope       string     `json:"scope"`
	SpaceID     string     `json:"space_id,omitempty"`
	Reason      string     `json:"reason"`
	IsPermanent bool       `json:"is_permanent"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	BannedBy    string     `json:"banned_by"`
	BannedAt    time.Time  `json:"banned_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// enrichBanned 把禁言记录补成 bannedListView；用户信息查不到时留空，不影响列表。
func (h *bannedHandler) enrichBanned(ctx context.Context, bans []model.Banned) []BannedListView {
	out := make([]BannedListView, 0, len(bans))
	cache := make(map[string]*model.User, len(bans))
	for _, b := range bans {
		u, ok := cache[b.UserID]
		if !ok {
			u, _ = h.user.GetByID(ctx, b.UserID)
			cache[b.UserID] = u
		}
		item := BannedListView{
			ID:          b.ID,
			UserID:      b.UserID,
			Scope:       b.Scope,
			SpaceID:     b.SpaceID,
			Reason:      b.Reason,
			IsPermanent: b.IsPermanent,
			ExpiresAt:   b.ExpiresAt,
			BannedBy:    b.CreatedBy,
			BannedAt:    b.CreatedAt,
			CreatedAt:   b.CreatedAt,
		}
		if u != nil {
			item.Username = u.Username
			item.Email = u.Email
		}
		out = append(out, item)
	}
	return out
}

// List 禁言记录列表
// @Summary      禁言记录列表
// @Description  分页获取禁言记录列表，支持按 scope 和 space_id 筛选
// @Tags         禁言
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page      body     int     false  "页码"      default(1)
// @Param        pageSize  body     int     false  "每页数量"  default(20)
// @Param        scope     body     string  false  "禁言范围"  Enums(global, space)
// @Param        space_id  body     string  false  "版面ID"    default()
// @Success      200  {object}  pkg.Response{data=object{items=[]handler.BannedListView}}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /admin/banned/list [post]
func (h *bannedHandler) List(ctx iris.Context) {
	var req struct {
		Page     int    `json:"page"`
		PageSize int    `json:"page_size"`
		Scope    string `json:"scope"`
		SpaceID  string `json:"space_id"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	req.Page, req.PageSize = pkg.NormalizePagination(req.Page, req.PageSize)

	var bans []model.Banned
	var err error

	if req.Scope != "" {
		bans, err = h.svc.ListByScope(ctx.Request().Context(), req.Scope, req.SpaceID, req.Page, req.PageSize)
	} else {
		bans, err = h.svc.List(ctx.Request().Context(), req.Page, req.PageSize)
	}

	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"items": h.enrichBanned(ctx.Request().Context(), bans)})
}

// ListByUserID 获取用户的所有禁言记录
// @Summary      用户禁言概览
// @Description  获取指定用户的所有禁言记录（全局和各版面）
// @Tags         禁言
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      string  true  "用户 ID"
// @Success      200  {object}  pkg.Response{data=object{items=[]handler.BannedListView}}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /user/{id}/banned/overview [post]
func (h *bannedHandler) ListByUserID(ctx iris.Context) {
	userUUID, ok := h.resolveUserID(ctx)
	if !ok {
		return
	}

	// 查看他人禁言历史需 PermBannedManage
	if !h.bannedStatusAllowed(ctx, userUUID) {
		pkg.Fail(ctx, pkg.NOT_FOUND, "")
		return
	}

	bans, err := h.svc.ListByUserID(ctx.Request().Context(), userUUID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"items": h.enrichBanned(ctx.Request().Context(), bans)})
}
