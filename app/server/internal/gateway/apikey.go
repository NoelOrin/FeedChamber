package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"

	"github.com/kataras/iris/v12"
)

// apiKeyScopeRegistry maps coarse API key scopes to exact permission codes.
// Precise permission codes can also be used as scopes, e.g. "post:read,report:manage".
var apiKeyScopeRegistry = map[string][]string{
	"read": {
		model.PermPostRead, model.PermCommentRead, model.PermUserRead,
		model.PermRoleRead, model.PermFollowRead, model.PermSecurityRead,
	},
	"write": {
		model.PermPostCreate, model.PermPostUpdate, model.PermPostDelete,
		model.PermCommentCreate, model.PermCommentUpdate, model.PermCommentDelete,
		model.PermTagCreate, model.PermTagUpdate, model.PermTagDelete,
		model.PermSpaceCreate, model.PermSpaceUpdate, model.PermSpaceDelete,
		model.PermReportCreate, model.PermFavoriteSelf, model.PermContentLike,
		model.PermFollowSelf, model.PermUploadSelf, model.PermNotificationSelf,
		model.PermPMSelf, model.PermCheckinSelf,
	},
	"admin": {
		model.PermRoleManage, model.PermSettingManage, model.PermReportManage,
		model.PermAnnouncementManage, model.PermBatchEdit, model.PermBannedManage,
		model.PermStatsRead, model.PermAdminSearch, model.PermAdminMetrics,
		model.PermReconcileCounters, model.PermUserUpdate, model.PermUserDelete,
		model.PermUserActivityRead,
	},
}

func normalizeAPIScopes(scopes string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, part := range strings.Split(scopes, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	sort.Strings(out)
	return out
}

func apiKeyScopesToPerms(scopes []string) map[string]struct{} {
	perms := map[string]struct{}{}
	for _, scope := range scopes {
		if mapped, ok := apiKeyScopeRegistry[scope]; ok {
			for _, code := range mapped {
				perms[code] = struct{}{}
			}
			continue
		}
		perms[scope] = struct{}{}
	}
	return perms
}

func apiKeyHasPermission(scopes string, permCode string) bool {
	if permCode == "" {
		return true
	}
	_, ok := apiKeyScopesToPerms(normalizeAPIScopes(scopes))[permCode]
	return ok
}

func validAPIScopes(scopes []string) bool {
	if len(scopes) == 0 {
		return false
	}
	for _, scope := range scopes {
		if _, ok := apiKeyScopeRegistry[scope]; ok {
			continue
		}
		if !strings.Contains(scope, ":") {
			return false
		}
	}
	return true
}

// APIKeyManager handles API key authentication and management.
type APIKeyManager struct {
	deps Deps
}

// NewAPIKeyManager creates a new APIKeyManager.
func NewAPIKeyManager(deps Deps) *APIKeyManager {
	return &APIKeyManager{deps: deps}
}

// ── Middleware ────────────────────────────────────────────────────────────────

// Middleware checks the Authorization: Bearer ak_xxx or X-API-Key header.
// Returns true if the request should continue, false if stopped (401/403).
// When successful it sets ctx.Values() for user_uuid, user_role, and scopes.
func (m *APIKeyManager) Middleware(ctx iris.Context) bool {
	// 1. Check if API key auth is enabled in settings
	if m.deps.SettingService == nil {
		pkg.Fail(ctx, pkg.FORBIDDEN, "API Key 认证未启用")
		return false
	}
	enabled, err := m.deps.SettingService.Get(ctx.Request().Context(), model.SettingAPIKeyEnabled)
	if err != nil || enabled != "true" {
		pkg.Fail(ctx, pkg.FORBIDDEN, "API Key 认证未启用")
		return false
	}

	// 2. Extract key from header
	key := ctx.GetHeader("X-API-Key")
	if key == "" {
		authHeader := ctx.GetHeader("Authorization")
		if authHeader == "" {
			pkg.Fail(ctx, pkg.TOKEN_WRONG, "API Key 不存在")
			return false
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			pkg.Fail(ctx, pkg.TOKEN_WRONG)
			return false
		}
		key = parts[1]
	}

	// 3. Validate key format (must start with "ak_")
	if !strings.HasPrefix(key, "ak_") {
		pkg.Fail(ctx, pkg.TOKEN_WRONG, "API Key 格式错误")
		return false
	}

	keyID := key[3:] // strip "ak_" prefix

	// 4. Look up in DB
	db := m.deps.DB
	if db == nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "数据库不可用")
		return false
	}

	var apiKey model.APIKey
	if err := db.WithContext(ctx.Request().Context()).First(&apiKey, "key_id = ?", keyID).Error; err != nil {
		pkg.Fail(ctx, pkg.TOKEN_WRONG, "API Key 不存在")
		return false
	}

	// 5. Verify bcrypt hash
	if err := bcrypt.CompareHashAndPassword([]byte(apiKey.SecretHash), []byte(key)); err != nil {
		pkg.Fail(ctx, pkg.TOKEN_WRONG, "API Key 无效")
		return false
	}

	// 6. Check expiry
	if apiKey.ExpiresAt != nil && time.Now().Unix() > *apiKey.ExpiresAt {
		pkg.Fail(ctx, pkg.TOKEN_EXPIRED, "API Key 已过期")
		return false
	}

	// 7. Check revocation
	if apiKey.Status == "revoked" {
		pkg.Fail(ctx, pkg.TOKEN_REVOKED, "API Key 已被吊销")
		return false
	}

	// 8. Rate limit check via KV
	if m.deps.KV != nil {
		allowed := m.checkRateLimit(ctx, &apiKey)
		if !allowed {
			return false
		}
	}

	// 9. Look up user role from DB (needed for permission checks downstream)
	userRole := "user"
	var user model.User
	if err := db.WithContext(ctx.Request().Context()).First(&user, "id = ?", apiKey.UserUUID).Error; err == nil {
		userRole = user.Role
	}

	// 10. Set context values
	ctx.Values().Set(pkg.CtxUserID, apiKey.UserUUID)
	ctx.Values().Set(pkg.CtxRole, userRole)
	ctx.Values().Set(pkg.CtxAPIKeyScopes, apiKey.Scopes)
	ctx.Values().Set(pkg.CtxAuthMethod, pkg.AuthMethodAPIKey)

	// 11. Update last_used_at asynchronously
	go m.updateLastUsed(apiKey.ID)

	return true
}

func (m *APIKeyManager) checkRateLimit(ctx iris.Context, apiKey *model.APIKey) bool {
	ctxReq := ctx.Request().Context()

	// Determine limit: use per-key limit if set, otherwise default
	limit := apiKey.RateLimit
	if limit <= 0 {
		if m.deps.SettingService != nil {
			val, err := m.deps.SettingService.Get(ctxReq, model.SettingAPIKeyDefaultRateLimit)
			if err == nil {
				if n, err := strconv.Atoi(val); err == nil && n > 0 {
					limit = n
				}
			}
		}
		if limit <= 0 {
			limit = 100 // hardcoded fallback
		}
	}

	key := "apikey_rl:" + apiKey.KeyID
	window := time.Minute

	store := m.deps.KV
	if store == nil {
		return true // fail open
	}

	_, err := store.SetNX(ctxReq, key, "0", window)
	if err != nil {
		return true
	}

	count, err := store.Incr(ctxReq, key)
	if err != nil {
		return true
	}

	if count > int64(limit) {
		pkg.Fail(ctx, pkg.RATE_LIMITED, "API Key 请求过于频繁")
		return false
	}

	return true
}

func (m *APIKeyManager) updateLastUsed(id uint) {
	if m.deps.DB == nil {
		return
	}
	now := time.Now().Unix()
	_ = m.deps.DB.Model(&model.APIKey{}).Where("id = ?", id).Update("last_used_at", now).Error
}

// ── Management handlers ───────────────────────────────────────────────────────

// 创建 API Key
// @Summary      创建 API Key
// @Description  创建一枚 API Key；明文仅在创建响应中返回一次。
// @Tags         API Key
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{name=string,scopes=[]string,expires_in=int} false "名称与权限范围"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /api-keys [post]
// Create generates a new API key for the current user.
// POST /api/v1/api-keys
func (m *APIKeyManager) Create(ctx iris.Context) {
	userUUID := ctx.Values().GetString(pkg.CtxUserID)
	if userUUID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}

	var req struct {
		Name      string `json:"name"`
		Scopes    string `json:"scopes"`
		RateLimit int    `json:"rate_limit"`
		ExpiresAt *int64 `json:"expires_at"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	if req.Name == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "缺少 name")
		return
	}
	scopes := normalizeAPIScopes(req.Scopes)
	if !validAPIScopes(scopes) {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "scopes 为空或非法")
		return
	}
	if m.deps.DB == nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "数据库不可用")
		return
	}
	var creator model.User
	if err := m.deps.DB.WithContext(ctx.Request().Context()).First(&creator, "id = ?", userUUID).Error; err != nil {
		pkg.Fail(ctx, pkg.FORBIDDEN, "API Key 的创建者不存在")
		return
	}
	permSet := apiKeyScopesToPerms(scopes)
	for permCode := range permSet {
		if m.deps.Perms == nil || !m.deps.Perms.HasPermission(creator.Role, permCode) {
			pkg.Fail(ctx, pkg.FORBIDDEN, "申请的权限范围超出创建者的权限")
			return
		}
	}
	if req.ExpiresAt != nil && *req.ExpiresAt <= 0 {
		req.ExpiresAt = nil
	}

	// Generate key_id (32 random bytes = 64 hex chars)
	keyIDBytes := make([]byte, 32)
	if _, err := rand.Read(keyIDBytes); err != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "API Key 生成失败")
		return
	}
	keyID := hex.EncodeToString(keyIDBytes)

	// Generate full key: ak_<keyID>
	fullKey := "ak_" + keyID

	// Hash the full key
	hash, err := bcrypt.GenerateFromPassword([]byte(fullKey), bcrypt.DefaultCost)
	if err != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "API Key 哈希失败")
		return
	}

	now := time.Now().Unix()
	apiKey := model.APIKey{
		KeyID:      keyID,
		SecretHash: string(hash),
		UserUUID:   userUUID,
		Name:       req.Name,
		Scopes:     strings.Join(scopes, ","),
		RateLimit:  req.RateLimit,
		ExpiresAt:  req.ExpiresAt,
		Status:     "active",
		CreatedAt:  now,
	}

	if err := m.deps.DB.WithContext(ctx.Request().Context()).Create(&apiKey).Error; err != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "API Key 创建失败")
		return
	}

	pkg.Success(ctx, map[string]any{
		"id":         apiKey.ID,
		"key_id":     apiKey.KeyID,
		"api_key":    fullKey,
		"name":       apiKey.Name,
		"scopes":     apiKey.Scopes,
		"rate_limit": apiKey.RateLimit,
		"expires_at": apiKey.ExpiresAt,
		"created_at": apiKey.CreatedAt,
	})
}

// API Key 列表
// @Summary      API Key 列表
// @Description  列出当前用户的 API Key（不含明文）。
// @Tags         API Key
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /api-keys/list [post]
// List returns all active API keys for the current user.
// GET /api/v1/api-keys
func (m *APIKeyManager) List(ctx iris.Context) {
	userUUID := ctx.Values().GetString(pkg.CtxUserID)
	if userUUID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}

	if m.deps.DB == nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "数据库不可用")
		return
	}

	var keys []model.APIKey
	if err := m.deps.DB.WithContext(ctx.Request().Context()).
		Where("user_uuid = ?", userUUID).
		Order("created_at desc").
		Find(&keys).Error; err != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "API Key 列表读取失败")
		return
	}

	// Mask keys: only show first 8 chars of key_id
	type keyResponse struct {
		ID         uint   `json:"id"`
		KeyID      string `json:"key_id"`
		Name       string `json:"name"`
		Scopes     string `json:"scopes"`
		RateLimit  int    `json:"rate_limit"`
		ExpiresAt  *int64 `json:"expires_at"`
		LastUsedAt *int64 `json:"last_used_at"`
		Status     string `json:"status"`
		CreatedAt  int64  `json:"created_at"`
	}

	resp := make([]keyResponse, 0, len(keys))
	for _, k := range keys {
		masked := k.KeyID
		if len(masked) > 8 {
			masked = masked[:8] + "****"
		}
		resp = append(resp, keyResponse{
			ID:         k.ID,
			KeyID:      "ak_" + masked,
			Name:       k.Name,
			Scopes:     k.Scopes,
			RateLimit:  k.RateLimit,
			ExpiresAt:  k.ExpiresAt,
			LastUsedAt: k.LastUsedAt,
			Status:     k.Status,
			CreatedAt:  k.CreatedAt,
		})
	}

	pkg.Success(ctx, resp)
}

// 吊销 API Key
// @Summary      吊销 API Key
// @Description  吊销指定 API Key。
// @Tags         API Key
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "API Key ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /api-keys/{id}/revoke [post]
// Revoke deletes or marks an API key as revoked.
// POST /api/v1/api-keys/:id/revoke
func (m *APIKeyManager) Revoke(ctx iris.Context) {
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

	// Parse as string (could be uint or ULID — APIKey uses uint)
	var id uint
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "id 无效")
		return
	}

	if m.deps.DB == nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "数据库不可用")
		return
	}

	result := m.deps.DB.WithContext(ctx.Request().Context()).
		Model(&model.APIKey{}).
		Where("id = ? AND user_uuid = ?", id, userUUID).
		Update("status", "revoked")

	if result.Error != nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "API Key 吊销失败")
		return
	}
	if result.RowsAffected == 0 {
		pkg.Fail(ctx, pkg.TOKEN_WRONG, "API Key 不存在")
		return
	}

	pkg.Success(ctx, map[string]any{"message": "API key revoked"})
}
