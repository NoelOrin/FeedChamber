package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/pkg/kvstore"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

// Deps 是 HTTP 层消费的全部依赖，由 server/iris.go 从 container 的装配结果里填入。
//
// 之所以显式列出而不是把 *container.Container 传进来：后者会让 HTTP 层反向依赖装配包，
// 并使"这一层到底用了哪些服务"不可审计（handler 任取任拿）。字段名与 service 类型对齐，
// 装配处一眼可见来源。
type Deps struct {
	// 仅 /health 系统探针需要的基础设施句柄。
	Store db.Store
	KV    kvstore.Store
	Cfg   config.Config

	Admin        service.AdminService
	Announcement service.AnnouncementService
	AuditLog     service.AuditLogService
	Auth         service.AuthService
	Backup       service.BackupService
	Badge        service.BadgeService
	Banned       service.BannedService
	CheckIn      service.CheckInService
	Content      service.ContentService
	Draft        service.DraftService
	Email        service.EmailService
	Favorite     service.FavoriteService
	Follow       service.FollowService
	Group        service.GroupService
	Invite       service.InviteService
	Leaderboard  service.LeaderboardService
	Like         service.LikeService
	MFA          service.MFAService
	Moderation   service.ModerationService
	Motion       service.MotionService
	Notification service.NotificationService
	OAuth        service.OAuthService
	Permission   service.PermissionService
	PM           service.PMService
	Plugin       service.PluginService
	Post         service.PostService
	Report       service.ReportService
	Revision     service.RevisionService
	Role         service.RoleService
	Sessions     service.SessionStore
	Setting      service.SettingService
	Space        service.SpaceService
	Tag          service.TagService
	Theme        service.ThemeService
	Trust        service.TrustService
	Gate         service.GateService
	Upload       service.UploadService
	User         service.UserService
	Watch        service.WatchService
	UserBlock    service.UserBlockService
	Webhook      service.WebhookService
}

type Handler struct {
	Auth         AuthHandler
	MFA          MFAHandler
	Post         PostHandler
	Content      ContentHandler
	Like         LikeHandler
	Favorite     FavoriteHandler
	Follow       FollowHandler
	Watch        WatchHandler
	UserBlock    UserBlockHandler
	Invite       InviteHandler
	User         UserHandler
	Role         RoleHandler
	Permission   PermissionHandler
	Group        GroupHandler
	Tag          TagHandler
	Upload       UploadHandler
	Notification NotificationHandler
	PM           PMHandler
	Report       ReportHandler
	Moderation   ModerationHandler
	Motion       MotionHandler
	Admin        AdminHandler
	Setting      SettingHandler
	Banned       BannedHandler
	Announcement AnnouncementHandler
	OAuth        *OAuthHandler
	CheckIn      CheckInHandler
	Space        SpaceHandler
	SEO          SEOHandler
	Leaderboard  LeaderboardHandler
	Badge        BadgeHandler
	Webhook      WebhookHandler
	Draft        DraftHandler
	Revision     RevisionHandler
	Realtime     RealtimeHandler
	Backup       BackupHandler
	Trash        TrashHandler
	Theme        ThemeHandler
	Plugin       PluginHandler
	Trust        TrustHandler
	Gate         GateHandler
	Home         HomeHandler
	// 以下三项只服务 /health 探针。
	store db.Store
	kv    kvstore.Store
	cfg   config.Config
}

// New builds the handler aggregate from an already-assembled Deps.
//
// Background worker lifecycle does NOT live here: it belongs to
// container.Runtime, which server/iris.go starts and shuts down.
func New(d *Deps) *Handler {
	return &Handler{
		Auth:         newAuthHandler(d.Auth, d.Sessions, d.Email, d.AuditLog),
		MFA:          newMFAHandler(d.MFA, d.Auth, d.User, d.Sessions),
		Post:         newPostHandler(d.Post),
		Content:      newContentHandler(d.Content),
		Like:         newLikeHandler(d.Like),
		Favorite:     newFavoriteHandler(d.Favorite),
		Follow:       newFollowHandler(d.Follow),
		Watch:        newWatchHandler(d.Watch),
		UserBlock:    newUserBlockHandler(d.UserBlock),
		Invite:       newInviteHandler(d.Invite),
		User:         newUserHandler(d.User, d.Follow, d.AuditLog, d.Trust),
		Role:         newRoleHandler(d.Role),
		Permission:   newPermissionHandler(d.Permission, d.AuditLog),
		Group:        newGroupHandler(d.Group),
		Tag:          newTagHandler(d.Tag),
		Upload:       newUploadHandler(d.Upload),
		Notification: newNotificationHandler(d.Notification, d.KV, d.Setting),
		PM:           newPMHandler(d.PM),
		Report:       newReportHandler(d.Report, d.AuditLog),
		Moderation:   newModerationHandler(d.Moderation, d.AuditLog),
		Motion:       newMotionHandler(d.Motion, d.AuditLog),
		Admin:        newAdminHandler(d.Admin, d.AuditLog),
		Setting:      newSettingHandler(d.Setting, d.AuditLog),
		Banned:       newBannedHandler(d.Banned, d.User, d.Permission, d.Setting, d.AuditLog),
		Announcement: newAnnouncementHandler(d.Announcement),
		OAuth:        NewOAuthHandler(d.OAuth, d.Banned, d.Sessions),
		CheckIn:      newCheckInHandler(d.CheckIn),
		Space:        newSpaceHandler(d.Space, d.Trust),
		SEO:          newSEOHandler(d.Post, d.Space, d.Tag, d.Setting),
		Leaderboard:  newLeaderboardHandler(d.Leaderboard),
		Badge:        newBadgeHandler(d.Badge),
		Webhook:      newWebhookHandler(d.Webhook),
		Draft:        newDraftHandler(d.Draft),
		Revision:     newRevisionHandler(d.Revision),
		Realtime:     newRealtimeHandler(d.KV),
		Backup:       newBackupHandler(d.Backup),
		Trash:        newTrashHandler(d.Post, d.Content, d.AuditLog),
		Theme:        newThemeHandler(d.Theme),
		Plugin:       newPluginHandler(d.Plugin),
		Trust:        newTrustHandler(d.Trust, d.AuditLog),
		Gate:         newGateHandler(d.Gate, d.AuditLog),
		Home:         newHomeHandler(d.User, d.Follow, d.Announcement),
		store:        d.Store,
		kv:           d.KV,
		cfg:          d.Cfg,
	}
}

// Health 健康检查
// @Summary      健康检查
// @Description  检查 DB、KV Store、Storage 是否正常运行
// @Tags         系统
// @Accept       json
// @Produce      json
// @Success      200  {object}  object{db=string,kvstore=string,storage=string}
// @Failure      503  {object}  object{db=string,kvstore=string,storage=string}
// @Router       /health [get]
func (h *Handler) Health(ctx iris.Context) {
	results := map[string]string{}

	// ── DB ─────────────────────────────────────────────────────────
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer dbCancel()

	sqlStore := h.store.SQL()
	if sqlStore == nil {
		results["db"] = "sql store not available"
	} else {
		gdb := sqlStore.DB()
		sqlDB, err := gdb.DB()
		if err != nil {
			results["db"] = fmt.Sprintf("get sql db: %s", err.Error())
		} else if err := sqlDB.PingContext(dbCtx); err != nil {
			results["db"] = fmt.Sprintf("db ping: %s", err.Error())
		} else {
			results["db"] = "ok"
			// Pool stats for monitoring (O-02)
			poolStats := h.store.PoolStats()
			if poolStats != nil {
				results["db_pool_max_open"] = fmt.Sprintf("%d", poolStats.MaxOpenConnections)
				results["db_pool_open"] = fmt.Sprintf("%d", poolStats.OpenConnections)
				results["db_pool_in_use"] = fmt.Sprintf("%d", poolStats.InUse)
				results["db_pool_idle"] = fmt.Sprintf("%d", poolStats.Idle)
				results["db_pool_wait_count"] = fmt.Sprintf("%d", poolStats.WaitCount)
			}
		}
	}

	// ── KV Store ───────────────────────────────────────────────────
	kvCtx, kvCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer kvCancel()

	testKey := "_health_check"
	if err := h.kv.Set(kvCtx, testKey, "ok", 10*time.Second); err != nil {
		results["kvstore"] = fmt.Sprintf("kv set: %s", err.Error())
	} else if _, err := h.kv.Get(kvCtx, testKey); err != nil {
		results["kvstore"] = fmt.Sprintf("kv get: %s", err.Error())
	} else if err := h.kv.Delete(kvCtx, testKey); err != nil {
		results["kvstore"] = fmt.Sprintf("kv delete: %s", err.Error())
	} else {
		results["kvstore"] = "ok"
	}

	// ── Storage ────────────────────────────────────────────────────
	switch h.cfg.Storage.Provider {
	case "", "local":
		dir := h.cfg.Storage.LocalDir
		if dir == "" {
			dir = "uploads"
		}
		if _, err := os.Stat(dir); err != nil {
			results["storage"] = fmt.Sprintf("storage dir stat: %s", err.Error())
		} else {
			results["storage"] = "ok"
		}
	case "s3":
		if h.cfg.Storage.Bucket == "" {
			results["storage"] = "s3 bucket not configured"
		} else {
			results["storage"] = "ok"
		}
	default:
		results["storage"] = fmt.Sprintf("unknown storage provider: %s", h.cfg.Storage.Provider)
	}

	hasFailed := false
	for _, k := range []string{"db", "kvstore", "storage"} {
		if v, ok := results[k]; !ok || v != "ok" {
			hasFailed = true
			break
		}
	}

	if hasFailed {
		ctx.StatusCode(http.StatusServiceUnavailable)
	}
	ctx.JSON(results)
}
