package gateway

import (
	"fmt"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"

	"github.com/kataras/iris/v12"
)

// SessionManager tracks user sessions and provides session management APIs.
// Rows are created by Login (via SessionStore.Issue) — this manager only
// touches, lists, revokes and rotates them.
type SessionManager struct {
	deps     Deps
	sessions *SessionStore
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(deps Deps) *SessionManager {
	return &SessionManager{deps: deps, sessions: NewSessionStore(deps)}
}

// ── Tracking middleware ───────────────────────────────────────────────────

// currentSession returns the *model.UserSession the auth middleware resolved
// for this request, or nil when the caller is not session-authenticated
// (API key / OAuth client requests carry no session).
func currentSession(ctx iris.Context) *model.UserSession {
	sess, _ := ctx.Values().Get(pkg.CtxSession).(*model.UserSession)
	return sess
}

// TrackingMiddleware updates the session's last_active timestamp on each
// authenticated request (throttled). It never creates rows — creation is
// owned by Login.
func (s *SessionManager) TrackingMiddleware(ctx iris.Context) {
	if s.sessions == nil {
		return
	}
	sess := currentSession(ctx)
	if sess == nil || sess.SessionID == "" {
		return
	}
	s.sessions.Touch(ctx.Request().Context(), sess, sessionTouchInterval)
}

// extractDeviceName does a basic parse of the User-Agent to derive a friendly
// device name. This can be enhanced later.
func extractDeviceName(ua string) string {
	if ua == "" {
		return "Unknown"
	}
	// Simple detection for common browsers
	if len(ua) > 100 {
		ua = ua[:100]
	}
	return ua
}

// ── Session management handlers ───────────────────────────────────────────

// sessionResponse is the JSON shape for a session in list responses.
type sessionResponse struct {
	ID         uint   `json:"id"`
	SessionID  string `json:"session_id"`
	IP         string `json:"ip"`
	UserAgent  string `json:"user_agent"`
	DeviceName string `json:"device_name"`
	Location   string `json:"location"`
	LastActive int64  `json:"last_active"`
	CreatedAt  int64  `json:"created_at"`
	ExpiresAt  int64  `json:"expires_at"`
	IsCurrent  bool   `json:"is_current"`
}

// 会话列表
// @Summary      会话列表
// @Description  列出当前用户的所有有效会话（含设备、IP、最近活跃）。
// @Tags         会话
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /sessions [get]
// ListSessions returns all active sessions for the current user.
// GET /api/v1/sessions
func (s *SessionManager) ListSessions(ctx iris.Context) {
	userUUID := ctx.Values().GetString(pkg.CtxUserID)
	if userUUID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}

	db := s.deps.DB
	if db == nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "数据库不可用")
		return
	}

	now := time.Now().Unix()
	var sessions []model.UserSession
	if err := db.WithContext(ctx.Request().Context()).
		Where("user_uuid = ? AND expires_at > ?", userUUID, now).
		Order("last_active desc").
		Find(&sessions).Error; err != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "会话列表读取失败")
		return
	}

	currentSessionID := ""
	if sess := currentSession(ctx); sess != nil {
		currentSessionID = sess.SessionID
	}

	resp := make([]sessionResponse, 0, len(sessions))
	for _, sess := range sessions {
		resp = append(resp, sessionResponse{
			ID:         sess.ID,
			SessionID:  sess.SessionID,
			IP:         sess.IP,
			UserAgent:  sess.UserAgent,
			DeviceName: sess.DeviceName,
			Location:   sess.Location,
			LastActive: sess.LastActive,
			CreatedAt:  sess.CreatedAt,
			ExpiresAt:  sess.ExpiresAt,
			IsCurrent:  sess.SessionID == currentSessionID,
		})
	}

	pkg.Success(ctx, resp)
}

// 吊销会话
// @Summary      吊销会话
// @Description  吊销指定会话，使其即时失效。
// @Tags         会话
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /sessions/{id}/revoke [post]
// RevokeSession revokes a specific session by ID.
// POST /api/v1/sessions/:id/revoke
func (s *SessionManager) RevokeSession(ctx iris.Context) {
	userUUID := ctx.Values().GetString(pkg.CtxUserID)
	if userUUID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}

	idStr := ctx.Params().Get("id")
	if idStr == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "缺少 id")
		return
	}

	var id uint
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "id 无效")
		return
	}

	db := s.deps.DB
	if db == nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "数据库不可用")
		return
	}

	var session model.UserSession
	if err := db.WithContext(ctx.Request().Context()).
		Where("id = ? AND user_uuid = ?", id, userUUID).
		First(&session).Error; err != nil {
		pkg.Fail(ctx, pkg.NOT_FOUND, "会话不存在")
		return
	}

	// Don't allow revoking the current session via this endpoint
	currentSessionID := ""
	if sess := currentSession(ctx); sess != nil {
		currentSessionID = sess.SessionID
	}
	if session.SessionID == currentSessionID {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "不能吊销当前正在使用的会话，请改用 /auth/logout")
		return
	}

	// Deleting the row + cache entry IS the revocation — the credential stops
	// resolving on the very next request.
	if s.sessions != nil {
		if err := s.sessions.Revoke(ctx.Request().Context(), session.SessionID); err != nil {
			pkg.Fail(ctx, pkg.INTERNAL_ERROR, "会话吊销失败")
			return
		}
	} else {
		if err := db.WithContext(ctx.Request().Context()).
			Delete(&session).Error; err != nil {
			pkg.Fail(ctx, pkg.INTERNAL_ERROR, "会话吊销失败")
			return
		}
	}

	pkg.Success(ctx, map[string]any{"message": "Session revoked"})
}

// 登出全部设备
// @Summary      登出全部设备
// @Description  吊销当前用户的全部会话。
// @Tags         会话
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /sessions/logout-all [post]
// LogoutAll revokes all sessions for the current user.
// POST /api/v1/sessions/logout-all
func (s *SessionManager) LogoutAll(ctx iris.Context) {
	userUUID := ctx.Values().GetString(pkg.CtxUserID)
	if userUUID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}

	if s.deps.DB == nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "数据库不可用")
		return
	}
	if s.sessions == nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "会话存储不可用")
		return
	}

	if err := s.sessions.RevokeAllForUser(ctx.Request().Context(), userUUID); err != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "批量吊销会话失败")
		return
	}

	pkg.Success(ctx, map[string]any{"message": "All sessions revoked"})
}

// LogoutHandler revokes the presenting session and clears the cookie.
// Wired as POST /api/v1/auth/logout.
//
// @Summary      用户登出
// @Description  吊销当前会话并清除 cookie（会话即时失效）
// @Tags         认证
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /auth/logout [post]
func (s *SessionManager) LogoutHandler(ctx iris.Context) {
	if sess := currentSession(ctx); sess != nil && s.sessions != nil {
		_ = s.sessions.Revoke(ctx.Request().Context(), sess.SessionID)
	}
	pkg.ClearSessionCookie(ctx)
	pkg.Success(ctx, nil)
}
